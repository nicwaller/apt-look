package apttransport

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileTransport struct{}

//goland:noinspection GoUnusedExportedFunction
func NewFileTransport() *FileTransport {
	return &FileTransport{}
}

func (t *FileTransport) Schemes() []string {
	return []string{"file"}
}

func (t *FileTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	// Convert file:// archiveRoot to local path
	path := req.URI.Path
	if req.URI.Host != "" {
		// Handle file://host/path format (though host should be empty for local files)
		path = filepath.Join(req.URI.Host, path)
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "context cancelled",
			Err:    ctx.Err(),
		}
	default:
	}

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &AcquireError{
				URI:    req.URI,
				Reason: "file not found",
				Err:    err,
			}
		}
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to stat file",
			Err:    err,
		}
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "path is a directory",
			Err:    nil,
		}
	}

	// Check modification time for conditional requests
	modTime := fileInfo.ModTime()
	if req.LastModified != nil && !modTime.After(*req.LastModified) {
		// File hasn't been modified since the last request
		return &AcquireResponse{
			URI:          req.URI,
			LastModified: &modTime,
			Size:         fileInfo.Size(),
		}, nil
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to open file",
			Err:    err,
		}
	}

	// Prepare response
	response := &AcquireResponse{
		URI:          req.URI,
		LastModified: &modTime,
		Size:         fileInfo.Size(),
	}

	// If saving to a different file, handle that
	if req.Filename != "" && req.Filename != path {
		return t.copyToFile(file, response, req)
	}

	// Otherwise return content directly
	content, hashes, err := t.readAndHash(file, req.ExpectedHashes, req.ProgressCallback, response.Size)
	if err != nil {
		file.Close()
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to read file",
			Err:    err,
		}
	}

	response.Content = content
	response.Hashes = hashes

	// Verify expected hashes
	if err := t.verifyHashes(response.Hashes, req.ExpectedHashes); err != nil {
		content.Close()
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "hash verification failed",
			Err:    err,
		}
	}

	return response, nil
}

func (t *FileTransport) copyToFile(sourceFile *os.File, response *AcquireResponse, req *AcquireRequest) (*AcquireResponse, error) {
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(req.Filename)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to create destination file",
			Err:    err,
		}
	}
	defer destFile.Close()

	// Create hash writers if needed
	hashers := make(map[string]hash.Hash)
	if len(req.ExpectedHashes) > 0 {
		for algo := range req.ExpectedHashes {
			if hasher := t.createHasher(algo); hasher != nil {
				hashers[algo] = hasher
			}
		}
	}

	// Create multi-writer for file and hashers
	writers := []io.Writer{destFile}
	for _, hasher := range hashers {
		writers = append(writers, hasher)
	}
	multiWriter := io.MultiWriter(writers...)

	// Copy with progress reporting
	var written int64
	if req.ProgressCallback != nil {
		reader := &fileProgressReader{
			reader:   sourceFile,
			callback: req.ProgressCallback,
			total:    response.Size,
		}
		written, err = io.Copy(multiWriter, reader)
	} else {
		written, err = io.Copy(multiWriter, sourceFile)
	}

	if err != nil {
		os.Remove(req.Filename)
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to copy file",
			Err:    err,
		}
	}

	// Collect hashes
	hashes := make(map[string]string)
	for algo, hasher := range hashers {
		hashes[algo] = fmt.Sprintf("%x", hasher.Sum(nil))
	}

	response.Filename = req.Filename
	response.Hashes = hashes
	response.Size = written

	// Verify expected hashes
	if err := t.verifyHashes(response.Hashes, req.ExpectedHashes); err != nil {
		os.Remove(req.Filename)
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "hash verification failed",
			Err:    err,
		}
	}

	return response, nil
}

func (t *FileTransport) readAndHash(file *os.File, expectedHashes map[string]string, progressCallback func(int64, int64), totalSize int64) (io.ReadCloser, map[string]string, error) {
	// Create hash writers if needed
	hashers := make(map[string]hash.Hash)
	if len(expectedHashes) > 0 {
		for algo := range expectedHashes {
			if hasher := t.createHasher(algo); hasher != nil {
				hashers[algo] = hasher
			}
		}
	}

	// Read all content
	var buf []byte
	if len(hashers) > 0 || progressCallback != nil {
		// Need to process content for hashing or progress
		writers := make([]io.Writer, 0, len(hashers))
		for _, hasher := range hashers {
			writers = append(writers, hasher)
		}

		var reader io.Reader = file
		if progressCallback != nil {
			reader = &fileProgressReader{
				reader:   file,
				callback: progressCallback,
				total:    totalSize,
			}
		}

		var err error
		if len(writers) > 0 {
			multiWriter := io.MultiWriter(writers...)
			buf, err = io.ReadAll(io.TeeReader(reader, multiWriter))
		} else {
			buf, err = io.ReadAll(reader)
		}
		if err != nil {
			return nil, nil, err
		}
	} else {
		// Simple read
		var err error
		buf, err = io.ReadAll(file)
		if err != nil {
			return nil, nil, err
		}
	}

	// Close original file since we've read all content
	file.Close()

	// Collect hashes
	hashes := make(map[string]string)
	for algo, hasher := range hashers {
		hashes[algo] = fmt.Sprintf("%x", hasher.Sum(nil))
	}

	return io.NopCloser(strings.NewReader(string(buf))), hashes, nil
}

func (t *FileTransport) createHasher(algorithm string) hash.Hash {
	switch strings.ToLower(algorithm) {
	case "md5":
		return md5.New()
	case "sha1":
		return sha1.New()
	case "sha256":
		return sha256.New()
	case "sha512":
		return sha512.New()
	default:
		return nil
	}
}

func (t *FileTransport) verifyHashes(actual, expected map[string]string) error {
	for algo, expectedHash := range expected {
		if actualHash, ok := actual[algo]; ok {
			if actualHash != expectedHash {
				return fmt.Errorf("hash mismatch for %s: expected %s, got %s", algo, expectedHash, actualHash)
			}
		}
	}
	return nil
}

type fileProgressReader struct {
	reader   io.Reader
	callback func(int64, int64)
	total    int64
	read     int64
}

func (fpr *fileProgressReader) Read(p []byte) (n int, err error) {
	n, err = fpr.reader.Read(p)
	fpr.read += int64(n)
	if fpr.callback != nil {
		fpr.callback(fpr.read, fpr.total)
	}
	return n, err
}

func (t *FileTransport) Close() error {
	return nil
}
