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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var _ Transport = &HTTPTransport{}

type HTTPTransport struct {
	userAgent string
	timeout   time.Duration
	client    *http.Client
}

func NewHTTPTransport() *HTTPTransport {
	timeout := time.Second * 60
	return &HTTPTransport{
		userAgent: "apt-look/1.0",
		timeout:   timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (t *HTTPTransport) Schemes() []string {
	return []string{"http", "https"}
}

func (t *HTTPTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URI.String(), nil)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to create request",
			Err:    err,
		}
	}

	// Set headers
	httpReq.Header.Set("User-Agent", t.userAgent)
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Handle conditional requests
	if req.LastModified != nil {
		httpReq.Header.Set("If-Modified-Since", req.LastModified.UTC().Format(http.TimeFormat))
	}

	// Use request timeout if specified
	client := t.client
	if req.Timeout > 0 {
		client.Timeout = req.Timeout
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "request failed",
			Err:    err,
		}
	}

	if resp.StatusCode == http.StatusNotModified {
		resp.Body.Close()
		return &AcquireResponse{
			URI:          req.URI,
			Headers:      responseHeaders(resp),
			LastModified: parseLastModified(resp.Header.Get("Last-Modified")),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Err:    nil,
		}
	}

	// Prepare response
	response := &AcquireResponse{
		URI:          httpReq.URL, // May have changed due to redirects
		Headers:      responseHeaders(resp),
		LastModified: parseLastModified(resp.Header.Get("Last-Modified")),
	}

	// Get content length if available
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			response.Size = size
		}
	}

	// If saving to file, handle that
	if req.Filename != "" {
		return t.saveToFile(resp, response, req)
	}

	// Otherwise return content directly
	content, hashes, size, err := t.readAndHash(resp.Body, req.ExpectedHashes, req.ProgressCallback, response.Size)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to read content",
			Err:    err,
		}
	}

	response.Content = content
	response.Hashes = hashes
	response.Size = size

	// Verify expected hashes
	if err := verifyHashes(response.Hashes, req.ExpectedHashes); err != nil {
		content.Close()
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "hash verification failed",
			Err:    err,
		}
	}

	return response, nil
}

func (t *HTTPTransport) saveToFile(resp *http.Response, response *AcquireResponse, req *AcquireRequest) (*AcquireResponse, error) {
	defer resp.Body.Close()

	file, err := os.Create(req.Filename)
	if err != nil {
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to create file",
			Err:    err,
		}
	}
	defer file.Close()

	// Create hash writers if needed
	hashers := make(map[string]hash.Hash)
	if len(req.ExpectedHashes) > 0 {
		for algo := range req.ExpectedHashes {
			if hasher := createHasher(algo); hasher != nil {
				hashers[algo] = hasher
			}
		}
	}

	// Create multi-writer for file and hashers
	writers := []io.Writer{file}
	for _, hasher := range hashers {
		writers = append(writers, hasher)
	}
	multiWriter := io.MultiWriter(writers...)

	// Copy with progress reporting
	var written int64
	if req.ProgressCallback != nil {
		progressReader := &progressReader{
			reader:   resp.Body,
			callback: req.ProgressCallback,
			total:    response.Size,
		}
		written, err = io.Copy(multiWriter, progressReader)
	} else {
		written, err = io.Copy(multiWriter, resp.Body)
	}

	if err != nil {
		os.Remove(req.Filename)
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "failed to write file",
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
	if err := verifyHashes(response.Hashes, req.ExpectedHashes); err != nil {
		os.Remove(req.Filename)
		return nil, &AcquireError{
			URI:    req.URI,
			Reason: "hash verification failed",
			Err:    err,
		}
	}

	return response, nil
}

func (t *HTTPTransport) readAndHash(reader io.ReadCloser, expectedHashes map[string]string, progressCallback func(int64, int64), totalSize int64) (io.ReadCloser, map[string]string, int64, error) {
	defer reader.Close()

	// Create hash writers if needed
	hashers := make(map[string]hash.Hash)
	if len(expectedHashes) > 0 {
		for algo := range expectedHashes {
			if hasher := createHasher(algo); hasher != nil {
				hashers[algo] = hasher
			}
		}
	}

	// Read all content
	var buf []byte
	var written int64
	if len(hashers) > 0 || progressCallback != nil {
		// Need to process content for hashing or progress
		writers := make([]io.Writer, 0, len(hashers))
		for _, hasher := range hashers {
			writers = append(writers, hasher)
		}

		var finalReader io.ReadCloser = reader
		if progressCallback != nil {
			finalReader = &progressReader{
				reader:   reader,
				callback: progressCallback,
				total:    totalSize,
			}
		}

		var err error
		if len(writers) > 0 {
			multiWriter := io.MultiWriter(writers...)
			buf, err = io.ReadAll(io.TeeReader(finalReader, multiWriter))
		} else {
			buf, err = io.ReadAll(finalReader)
		}
		if err != nil {
			return nil, nil, 0, err
		}
		written = int64(len(buf))
	} else {
		// Simple read
		var err error
		buf, err = io.ReadAll(reader)
		if err != nil {
			return nil, nil, 0, err
		}
		written = int64(len(buf))
	}

	// Collect hashes
	hashes := make(map[string]string)
	for algo, hasher := range hashers {
		hashes[algo] = fmt.Sprintf("%x", hasher.Sum(nil))
	}

	return io.NopCloser(strings.NewReader(string(buf))), hashes, written, nil
}

func responseHeaders(resp *http.Response) map[string]string {
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}

func parseLastModified(value string) *time.Time {
	if value == "" {
		return nil
	}
	if t, err := time.Parse(http.TimeFormat, value); err == nil {
		return &t
	}
	return nil
}

func createHasher(algorithm string) hash.Hash {
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

func verifyHashes(actual, expected map[string]string) error {
	for algo, expectedHash := range expected {
		if actualHash, ok := actual[algo]; ok {
			if actualHash != expectedHash {
				return fmt.Errorf("hash mismatch for %s: expected %s, got %s", algo, expectedHash, actualHash)
			}
		}
	}
	return nil
}

type progressReader struct {
	reader   io.ReadCloser
	callback func(int64, int64)
	total    int64
	read     int64
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.read += int64(n)
	if pr.callback != nil {
		pr.callback(pr.read, pr.total)
	}
	return n, err
}

func (pr *progressReader) Close() error {
	return pr.reader.Close()
}
