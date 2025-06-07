package apttransport

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTransport_Schemes(t *testing.T) {
	transport := NewFileTransport()
	schemes := transport.Schemes()
	
	assert.Contains(t, schemes, "file")
	assert.Len(t, schemes, 1)
}

func TestFileTransport_AcquireBasic(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	defer resp.Content.Close()
	
	// Verify response properties
	assert.Equal(t, fileURL.String(), resp.URI.String())
	assert.Equal(t, int64(len(testContent)), resp.Size)
	assert.NotNil(t, resp.LastModified)
	assert.NotNil(t, resp.Content)
	
	// Read and verify content
	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileTransport_AcquireWithHash(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Calculate expected MD5 hash
	hasher := md5.New()
	hasher.Write([]byte(testContent))
	expectedMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
		ExpectedHashes: map[string]string{
			"md5": expectedMD5,
		},
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	defer resp.Content.Close()
	
	// Verify hash was calculated
	assert.Contains(t, resp.Hashes, "md5")
	assert.Equal(t, expectedMD5, resp.Hashes["md5"])
}

func TestFileTransport_AcquireWithWrongHash(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
		ExpectedHashes: map[string]string{
			"md5": "deadbeefcafebabe", // Wrong hash
		},
	}
	
	ctx := context.Background()
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "hash verification failed")
}

func TestFileTransport_AcquireWithCopy(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:      fileURL,
		Filename: destFile,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify file was copied
	assert.Equal(t, destFile, resp.Filename)
	assert.FileExists(t, destFile)
	
	// Verify content
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
	assert.Equal(t, int64(len(testContent)), resp.Size)
}

func TestFileTransport_AcquireWithProgress(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!\nThis is a longer test file to trigger progress callbacks."
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	var progressCalls []int64
	progressCallback := func(downloaded, total int64) {
		progressCalls = append(progressCalls, downloaded)
	}
	
	req := &AcquireRequest{
		URI:              fileURL,
		ProgressCallback: progressCallback,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	// Consume content to trigger progress callbacks
	_, err = io.ReadAll(resp.Content)
	require.NoError(t, err)
	
	// Verify progress was called
	assert.Greater(t, len(progressCalls), 0)
	assert.Equal(t, resp.Size, progressCalls[len(progressCalls)-1])
}

func TestFileTransport_AcquireNotFound(t *testing.T) {
	transport := NewFileTransport()
	
	// Create file:// URL for non-existent file
	fileURL, err := url.Parse("file:///nonexistent/file.txt")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx := context.Background()
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "file not found")
}

func TestFileTransport_AcquireDirectory(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a directory
	tmpDir := t.TempDir()
	
	// Create file:// URL for directory
	fileURL, err := url.Parse("file://" + tmpDir)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx := context.Background()
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "path is a directory")
}

func TestFileTransport_AcquireWithConditionalRequest(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Get file modification time
	fileInfo, err := os.Stat(testFile)
	require.NoError(t, err)
	modTime := fileInfo.ModTime()
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	// Test with same modification time (should return not modified)
	req := &AcquireRequest{
		URI:          fileURL,
		LastModified: &modTime,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Should not have content since not modified
	assert.Nil(t, resp.Content)
	assert.Equal(t, int64(len(testContent)), resp.Size)
	assert.NotNil(t, resp.LastModified)
	
	// Test with older modification time (should return content)
	oldTime := modTime.Add(-time.Hour)
	req2 := &AcquireRequest{
		URI:          fileURL,
		LastModified: &oldTime,
	}
	
	resp2, err := transport.Acquire(ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)
	defer resp2.Content.Close()
	
	// Should have content since file is newer
	assert.NotNil(t, resp2.Content)
	content, err := io.ReadAll(resp2.Content)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileTransport_AcquireWithCancelledContext(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "context cancelled")
}

func TestFileTransport_AcquireWithCopyAndHash(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Calculate expected MD5 hash
	hasher := md5.New()
	hasher.Write([]byte(testContent))
	expectedMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:      fileURL,
		Filename: destFile,
		ExpectedHashes: map[string]string{
			"md5": expectedMD5,
		},
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify file was copied and hash calculated
	assert.Equal(t, destFile, resp.Filename)
	assert.FileExists(t, destFile)
	assert.Contains(t, resp.Hashes, "md5")
	assert.Equal(t, expectedMD5, resp.Hashes["md5"])
	
	// Verify content
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileTransport_AcquireMultipleHashes(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
		ExpectedHashes: map[string]string{
			"md5":    "65a8e27d8879283831b664bd8b7f0ad4", // Known MD5 for "Hello, World!"
			"sha256": "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f", // Known SHA256
		},
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	// Verify both hashes were calculated
	assert.Contains(t, resp.Hashes, "md5")
	assert.Contains(t, resp.Hashes, "sha256")
	assert.Equal(t, "65a8e27d8879283831b664bd8b7f0ad4", resp.Hashes["md5"])
	assert.Equal(t, "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f", resp.Hashes["sha256"])
}

func TestFileTransport_Close(t *testing.T) {
	transport := NewFileTransport()
	err := transport.Close()
	assert.NoError(t, err)
}

func TestFileTransport_RelativePath(t *testing.T) {
	transport := NewFileTransport()
	
	// Create a test file in current directory
	testFile := "test_relative.txt"
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)
	
	// Get absolute path for file:// URL
	absPath, err := filepath.Abs(testFile)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + absPath)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	// Verify content
	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileRegistry_Integration(t *testing.T) {
	registry := NewRegistry()
	fileTransport := NewFileTransport()
	
	registry.Register(fileTransport)
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create file:// URL
	fileURL, err := url.Parse("file://" + testFile)
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: fileURL,
	}
	
	ctx := context.Background()
	resp, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	// Verify content
	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}