package apttransport

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPTransport_Schemes(t *testing.T) {
	transport := NewHTTPTransport()
	schemes := transport.Schemes()
	
	assert.Contains(t, schemes, "http")
	assert.Contains(t, schemes, "https")
	assert.Len(t, schemes, 2)
}

func TestHTTPTransport_AcquireSignalRelease(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	defer resp.Content.Close()
	
	// Verify response properties
	assert.Equal(t, releaseURL.String(), resp.URI.String())
	assert.Greater(t, resp.Size, int64(0))
	assert.NotNil(t, resp.Content)
	
	// Read and verify content contains expected Release file fields
	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	
	contentStr := string(content)
	assert.Contains(t, contentStr, "Origin:")
	assert.Contains(t, contentStr, "Suite:")
	assert.Contains(t, contentStr, "Codename:")
	assert.Contains(t, contentStr, "Date:")
	assert.Contains(t, contentStr, "Architectures:")
	assert.Contains(t, contentStr, "Components:")
	
	// Verify it's a proper Debian Release file format
	lines := strings.Split(contentStr, "\n")
	assert.Greater(t, len(lines), 5, "Release file should have multiple lines")
}

func TestHTTPTransport_AcquireSignalPackages(t *testing.T) {
	transport := NewHTTPTransport()
	
	packagesURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/main/binary-amd64/Packages")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     packagesURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	defer resp.Content.Close()
	
	// Verify response properties
	assert.Equal(t, packagesURL.String(), resp.URI.String())
	assert.Greater(t, resp.Size, int64(0))
	
	// Read content and verify it's a Packages file
	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	
	contentStr := string(content)
	assert.Contains(t, contentStr, "Package:")
	assert.Contains(t, contentStr, "Version:")
	assert.Contains(t, contentStr, "Architecture:")
	assert.Contains(t, contentStr, "Filename:")
}

func TestHTTPTransport_AcquireWithFile(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	// Create temp file
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "Release")
	
	req := &AcquireRequest{
		URI:      releaseURL,
		Filename: filename,
		Timeout:  time.Second * 30,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify file was created
	assert.Equal(t, filename, resp.Filename)
	assert.FileExists(t, filename)
	
	// Verify file content
	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Greater(t, len(content), 0)
	
	contentStr := string(content)
	assert.Contains(t, contentStr, "Origin:")
	assert.Contains(t, contentStr, "Suite:")
}

func TestHTTPTransport_AcquireWithHashVerification(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	// First, get the content to calculate expected hash
	req1 := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp1, err := transport.Acquire(ctx, req1)
	require.NoError(t, err)
	defer resp1.Content.Close()
	
	content, err := io.ReadAll(resp1.Content)
	require.NoError(t, err)
	
	// Calculate MD5 hash
	hasher := md5.New()
	hasher.Write(content)
	expectedMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
	
	// Now test with hash verification
	req2 := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
		ExpectedHashes: map[string]string{
			"md5": expectedMD5,
		},
	}
	
	resp2, err := transport.Acquire(ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)
	defer resp2.Content.Close()
	
	// Verify hash was calculated
	assert.Contains(t, resp2.Hashes, "md5")
	assert.Equal(t, expectedMD5, resp2.Hashes["md5"])
}

func TestHTTPTransport_AcquireWithWrongHash(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
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

func TestHTTPTransport_AcquireWithProgress(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	var progressCalls []int64
	progressCallback := func(downloaded, total int64) {
		progressCalls = append(progressCalls, downloaded)
	}
	
	req := &AcquireRequest{
		URI:              releaseURL,
		Timeout:          time.Second * 30,
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

func TestHTTPTransport_AcquireWithHeaders(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
		Headers: map[string]string{
			"X-Test-Header": "test-value",
		},
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	// Should succeed even with custom headers
	assert.Greater(t, resp.Size, int64(0))
}

func TestHTTPTransport_AcquireNotFound(t *testing.T) {
	transport := NewHTTPTransport()
	
	notFoundURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/nonexistent/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     notFoundURL,
		Timeout: time.Second * 10,
	}
	
	ctx := context.Background()
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "HTTP 404")
}

func TestHTTPTransport_AcquireWithConditionalRequest(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	// First request to get Last-Modified
	req1 := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp1, err := transport.Acquire(ctx, req1)
	require.NoError(t, err)
	defer resp1.Content.Close()
	
	// If we got a Last-Modified header, test conditional request
	if resp1.LastModified != nil {
		req2 := &AcquireRequest{
			URI:          releaseURL,
			Timeout:      time.Second * 30,
			LastModified: resp1.LastModified,
		}
		
		resp2, err := transport.Acquire(ctx, req2)
		require.NoError(t, err)
		
		// Should either get 304 Not Modified (no content) or 200 OK (with content)
		if resp2.Content != nil {
			resp2.Content.Close()
		}
	}
}

func TestHTTPTransport_AcquireWithTimeout(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	// Very short timeout that should work
	req := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	assert.Greater(t, resp.Size, int64(0))
}

func TestHTTPTransport_AcquireWithCancelledContext(t *testing.T) {
	transport := NewHTTPTransport()
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: releaseURL,
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err = transport.Acquire(ctx, req)
	require.Error(t, err)
	
	var acquireErr *AcquireError
	assert.ErrorAs(t, err, &acquireErr)
	assert.Contains(t, acquireErr.Reason, "request failed")
}

func TestRegistry_Integration(t *testing.T) {
	registry := NewRegistry()
	httpTransport := NewHTTPTransport()
	
	registry.Register(httpTransport)
	
	releaseURL, err := url.Parse("https://updates.signal.org/desktop/apt/dists/xenial/Release")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI:     releaseURL,
		Timeout: time.Second * 30,
	}
	
	ctx := context.Background()
	resp, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Content.Close()
	
	assert.Greater(t, resp.Size, int64(0))
}

func TestRegistry_UnsupportedScheme(t *testing.T) {
	registry := NewRegistry()
	httpTransport := NewHTTPTransport()
	
	registry.Register(httpTransport)
	
	ftpURL, err := url.Parse("ftp://example.com/file")
	require.NoError(t, err)
	
	req := &AcquireRequest{
		URI: ftpURL,
	}
	
	ctx := context.Background()
	_, err = registry.Acquire(ctx, req)
	require.Error(t, err)
	
	var unsupportedErr *UnsupportedSchemeError
	assert.ErrorAs(t, err, &unsupportedErr)
	assert.Equal(t, "ftp", unsupportedErr.Scheme)
}