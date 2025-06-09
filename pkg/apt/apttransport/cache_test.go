package apttransport

import (
	"compress/gzip"
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

// mockTransport for testing
type mockTransport struct {
	responses map[string]string // Store content as strings
	callCount map[string]int
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses: make(map[string]string),
		callCount: make(map[string]int),
	}
}

func (m *mockTransport) Schemes() []string {
	return []string{"mock"}
}

func (m *mockTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	key := req.URI.String()
	m.callCount[key]++

	if content, ok := m.responses[key]; ok {
		hash := md5.Sum([]byte(content))
		now := time.Now()

		resp := &AcquireResponse{
			URI:          req.URI,
			Content:      io.NopCloser(strings.NewReader(content)),
			Size:         int64(len(content)),
			LastModified: &now,
			Headers:      make(map[string]string),
			Hashes: map[string]string{
				"md5": fmt.Sprintf("%x", hash),
			},
		}
		return resp, nil
	}

	return nil, &AcquireError{
		URI:    req.URI,
		Reason: "mock response not found",
		Err:    nil,
	}
}

func (m *mockTransport) setResponse(uri string, content string) {
	m.responses[uri] = content
}

func (m *mockTransport) getCallCount(uri string) int {
	return m.callCount[uri]
}

func TestCacheTransport_Schemes(t *testing.T) {
	mock := newMockTransport()
	config := CacheConfig{Disabled: false, CacheDir: t.TempDir()}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	assert.Equal(t, mock.Schemes(), cache.Schemes())
}

func TestCacheTransport_ReleaseFileBypass(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Set up mock response for Release file
	releaseURI := "mock://example.com/dists/jammy/Release"
	releaseContent := "Origin: Ubuntu\nSuite: jammy\nCodename: jammy\n"
	mock.setResponse(releaseURI, releaseContent)

	parsedURI, err := url.Parse(releaseURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// First request
	resp1, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp1.Content.Close()

	content1, err := io.ReadAll(resp1.Content)
	require.NoError(t, err)
	assert.Equal(t, releaseContent, string(content1))

	// Second request - should hit wrapped transport again (no caching for Release files)
	resp2, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp2.Content.Close()

	content2, err := io.ReadAll(resp2.Content)
	require.NoError(t, err)
	assert.Equal(t, releaseContent, string(content2))

	// Verify both requests hit the wrapped transport
	assert.Equal(t, 2, mock.getCallCount(releaseURI))

	// Verify no cache files were created
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestCacheTransport_PackagesFileCaching(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Set up mock response for Packages file
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\nArchitecture: amd64\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// First request - should hit wrapped transport and cache
	resp1, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp1.Content.Close()

	content1, err := io.ReadAll(resp1.Content)
	require.NoError(t, err)
	assert.Equal(t, packagesContent, string(content1))
	assert.Equal(t, 1, mock.getCallCount(packagesURI))

	// Verify cache file was created
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".gz"))

	// Second request - should hit cache, not wrapped transport
	resp2, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp2.Content.Close()

	content2, err := io.ReadAll(resp2.Content)
	require.NoError(t, err)
	assert.Equal(t, packagesContent, string(content2))

	// Verify wrapped transport was only called once
	assert.Equal(t, 1, mock.getCallCount(packagesURI))
}

func TestRegistry_CacheStats(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	registry := NewRegistryWithCache(config)
	registry.Register(mock)

	// Initially no stats
	hits, misses, hitRatio := registry.GetCacheStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 0.0, hitRatio)

	// Set up mock response for Packages file
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// First request - should be a miss
	resp1, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	resp1.Content.Close()

	hits, misses, hitRatio = registry.GetCacheStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(1), misses)
	assert.Equal(t, 0.0, hitRatio)

	// Second request - should be a hit
	resp2, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	resp2.Content.Close()

	hits, misses, hitRatio = registry.GetCacheStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)
	assert.Equal(t, 0.5, hitRatio)

	// Third request - another hit
	resp3, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	resp3.Content.Close()

	hits, misses, hitRatio = registry.GetCacheStats()
	assert.Equal(t, int64(2), hits)
	assert.Equal(t, int64(1), misses)
	assert.InDelta(t, 0.6667, hitRatio, 0.001) // 2/3 ≈ 0.6667
}

func TestCacheTransport_CacheKeyGeneration(t *testing.T) {
	mock := newMockTransport()
	config := CacheConfig{Disabled: false, CacheDir: t.TempDir()}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	uri1, _ := url.Parse("mock://example.com/packages")
	uri2, _ := url.Parse("mock://example.com/other")

	key1 := cache.getCacheKey(uri1)
	key2 := cache.getCacheKey(uri2)

	// Keys should be different for different URIs
	assert.NotEqual(t, key1, key2)

	// Keys should be consistent for same archiveRoot
	assert.Equal(t, key1, cache.getCacheKey(uri1))

	// Keys should be MD5 hashes (32 hex characters)
	assert.Len(t, key1, 32)
	assert.Len(t, key2, 32)
}

func TestCacheTransport_CacheFileCompression(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Set up mock response
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\nArchitecture: amd64\nDescription: A test package\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// Make request to create cache file
	resp, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp.Content.Close()

	// Find the cache file
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	cacheFile := filepath.Join(cacheDir, entries[0].Name())

	// Verify file is gzip compressed
	file, err := os.Open(cacheFile)
	require.NoError(t, err)
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzipReader.Close()

	cachedContent, err := io.ReadAll(gzipReader)
	require.NoError(t, err)

	assert.Equal(t, packagesContent, string(cachedContent))
}

func TestCacheTransport_DisabledCache(t *testing.T) {
	mock := newMockTransport()
	config := CacheConfig{Disabled: true, CacheDir: t.TempDir()}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Set up mock response
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// Make two requests
	resp1, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	resp1.Content.Close()

	resp2, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	resp2.Content.Close()

	// Both should hit wrapped transport (no caching)
	assert.Equal(t, 2, mock.getCallCount(packagesURI))

	// No cache files should be created
	entries, err := os.ReadDir(config.CacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestCacheTransport_PurgeCache(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Create some cache files by making requests
	for i := 0; i < 3; i++ {
		uri := fmt.Sprintf("mock://example.com/packages%d", i)
		content := fmt.Sprintf("Package: test-package-%d\nVersion: 1.0.0\n", i)
		mock.setResponse(uri, content)

		parsedURI, _ := url.Parse(uri)
		req := &AcquireRequest{URI: parsedURI}
		resp, err := cache.Acquire(context.Background(), req)
		require.NoError(t, err)
		resp.Content.Close()
	}

	// Verify cache files were created
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Purge cache
	err = cache.PurgeCache()
	require.NoError(t, err)

	// Verify cache files were removed
	entries, err = os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestCacheTransport_PurgeCacheDisabled(t *testing.T) {
	mock := newMockTransport()
	config := CacheConfig{Disabled: true, CacheDir: t.TempDir()}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Should not error when purging disabled cache
	err = cache.PurgeCache()
	require.NoError(t, err)
}

func TestCacheTransport_DefaultCacheDir(t *testing.T) {
	// Test XDG_CACHE_HOME
	originalXDG := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", originalXDG)

	testDir := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", testDir)

	cacheDir := getDefaultCacheDir()
	assert.Equal(t, filepath.Join(testDir, "apt-look"), cacheDir)

	// Test fallback to ~/.cache
	os.Unsetenv("XDG_CACHE_HOME")
	cacheDir = getDefaultCacheDir()
	homeDir, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(homeDir, ".cache", "apt-look"), cacheDir)
}

func TestCacheTransport_FileDetection(t *testing.T) {
	tests := []struct {
		uri         string
		isRelease   bool
		isPackages  bool
		isCacheable bool
	}{
		{"http://example.com/dists/jammy/Release", true, false, false},
		{"http://example.com/dists/jammy/Release.gpg", true, false, false},
		{"http://example.com/dists/jammy/InRelease", true, false, false},
		{"http://example.com/dists/jammy/main/binary-amd64/Packages", false, true, true},
		{"http://example.com/dists/jammy/main/binary-amd64/Packages.gz", false, true, true},
		{"http://example.com/dists/jammy/main/binary-amd64/Packages.bz2", false, true, true},
		{"http://example.com/dists/jammy/main/binary-amd64/Packages.xz", false, true, true},
		{"http://example.com/dists/jammy/Contents-amd64", false, false, true},
		{"http://example.com/dists/jammy/Contents-amd64.gz", false, false, true},
		{"http://example.com/dists/jammy/Contents-amd64.bz2", false, false, true},
		{"http://example.com/dists/jammy/Contents-amd64.xz", false, false, true},
		{"http://example.com/dists/jammy/main/source/Sources", false, false, true},
		{"http://example.com/dists/jammy/main/source/Sources.gz", false, false, true},
		{"http://example.com/dists/jammy/main/source/Sources.bz2", false, false, true},
		{"http://example.com/dists/jammy/main/source/Sources.xz", false, false, true},
		{"http://example.com/dists/jammy/main/i18n/Translation-en", false, false, true},
		{"http://example.com/dists/jammy/main/i18n/Translation-en.gz", false, false, true},
		{"http://example.com/some/other/file", false, false, false},
		{"http://example.com/pool/main/a/apache2/apache2_2.4.41-4ubuntu3_amd64.deb", false, false, false},
	}

	for _, test := range tests {
		uri, _ := url.Parse(test.uri)
		assert.Equal(t, test.isRelease, isReleaseFile(uri), "isReleaseFile failed for %s", test.uri)
		assert.Equal(t, test.isPackages, isPackagesFile(uri), "isPackagesFile failed for %s", test.uri)
		assert.Equal(t, test.isCacheable, isCacheableFile(uri), "isCacheableFile failed for %s", test.uri)
	}
}

func TestRegistry_WithCaching(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	registry := NewRegistryWithCache(config)
	registry.Register(mock)

	// Set up mock response
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// First request
	resp1, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp1.Content.Close()

	// Second request - should hit cache
	resp2, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp2.Content.Close()

	// Verify wrapped transport was only called once
	assert.Equal(t, 1, mock.getCallCount(packagesURI))

	// Verify cache file was created
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestRegistry_CacheDisabled(t *testing.T) {
	mock := newMockTransport()
	config := CacheConfig{Disabled: true, CacheDir: t.TempDir()}

	registry := NewRegistryWithCache(config)
	registry.Register(mock)

	// Set up mock response
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// Make two requests
	resp1, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	resp1.Content.Close()

	resp2, err := registry.Acquire(ctx, req)
	require.NoError(t, err)
	resp2.Content.Close()

	// Both should hit wrapped transport (no caching)
	assert.Equal(t, 2, mock.getCallCount(packagesURI))
}

func TestRegistry_PurgeCache(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	registry := NewRegistryWithCache(config)
	registry.Register(mock)

	// Create cache file
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	resp, err := registry.Acquire(context.Background(), req)
	require.NoError(t, err)
	resp.Content.Close()

	// Verify cache file exists
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Purge cache
	err = registry.PurgeCache()
	require.NoError(t, err)

	// Verify cache file was removed
	entries, err = os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestCacheTransport_CacheReadError(t *testing.T) {
	mock := newMockTransport()
	cacheDir := t.TempDir()
	config := CacheConfig{Disabled: false, CacheDir: cacheDir}

	cache, err := NewCacheTransport(mock, config)
	require.NoError(t, err)

	// Set up mock response
	packagesURI := "mock://example.com/dists/jammy/main/binary-amd64/Packages"
	packagesContent := "Package: test-package\nVersion: 1.0.0\n"
	mock.setResponse(packagesURI, packagesContent)

	parsedURI, err := url.Parse(packagesURI)
	require.NoError(t, err)

	// Create a corrupted cache file
	cacheKey := cache.getCacheKey(parsedURI)
	cachePath := filepath.Join(cacheDir, cacheKey+".gz")
	err = os.WriteFile(cachePath, []byte("corrupted data"), 0644)
	require.NoError(t, err)

	req := &AcquireRequest{URI: parsedURI}
	ctx := context.Background()

	// Should fall back to wrapped transport when cache read fails
	resp, err := cache.Acquire(ctx, req)
	require.NoError(t, err)
	defer resp.Content.Close()

	content, err := io.ReadAll(resp.Content)
	require.NoError(t, err)
	assert.Equal(t, packagesContent, string(content))

	// Verify wrapped transport was called
	assert.Equal(t, 1, mock.getCallCount(packagesURI))
}
