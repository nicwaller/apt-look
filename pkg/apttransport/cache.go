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
	"sync"

	"github.com/rs/zerolog/log"
)

var _ Transport = &CacheTransport{}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	hits   int64
	misses int64
	mu     sync.RWMutex
}

func (cs *CacheStats) Hit() {
	cs.mu.Lock()
	cs.hits++
	cs.mu.Unlock()
}

func (cs *CacheStats) Miss() {
	cs.mu.Lock()
	cs.misses++
	cs.mu.Unlock()
}

func (cs *CacheStats) GetStats() (hits, misses int64) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.hits, cs.misses
}

func (cs *CacheStats) GetHitRatio() float64 {
	hits, misses := cs.GetStats()
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// CacheTransport wraps another transport with caching capabilities
type CacheTransport struct {
	wrapped  Transport
	cacheDir string
	disabled bool
	stats    *CacheStats
}

// CacheConfig configures the caching behavior
type CacheConfig struct {
	// Disabled completely disables caching
	Disabled bool
	
	// CacheDir specifies the cache directory. If empty, uses XDG_CACHE_HOME/apt-look
	CacheDir string
}

// NewCacheTransport creates a new caching transport that wraps another transport
func NewCacheTransport(wrapped Transport, config CacheConfig) (*CacheTransport, error) {
	cacheDir := config.CacheDir
	if cacheDir == "" {
		cacheDir = getDefaultCacheDir()
	}

	// Create cache directory if it doesn't exist
	if !config.Disabled {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
		}
		log.Debug().Str("cache_dir", cacheDir).Msg("cache: initialized")
	} else {
		log.Debug().Msg("cache: disabled")
	}

	return &CacheTransport{
		wrapped:  wrapped,
		cacheDir: cacheDir,
		disabled: config.Disabled,
		stats:    &CacheStats{},
	}, nil
}

func (c *CacheTransport) Schemes() []string {
	return c.wrapped.Schemes()
}

func (c *CacheTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	// Never cache Release files - always fetch fresh
	if isReleaseFile(req.URI) {
		log.Debug().Str("uri", req.URI.String()).Msg("cache: bypassing cache for Release file")
		return c.wrapped.Acquire(ctx, req)
	}

	// If caching is disabled, pass through
	if c.disabled {
		log.Debug().Str("uri", req.URI.String()).Msg("cache: disabled, passing through")
		return c.wrapped.Acquire(ctx, req)
	}

	// Check if we have a cached version
	cacheKey := c.getCacheKey(req.URI)
	cachePath := filepath.Join(c.cacheDir, cacheKey+".gz")

	// Try to load from cache first
	if cached, err := c.loadFromCache(cachePath, req); err == nil && cached != nil {
		c.stats.Hit()
		log.Debug().Str("uri", req.URI.String()).Str("cache_key", cacheKey).Msg("cache: HIT")
		return cached, nil
	}

	// Only count cache misses for cacheable files (not Release files or when disabled)
	if isPackagesFile(req.URI) {
		c.stats.Miss()
	}
	log.Debug().Str("uri", req.URI.String()).Str("cache_key", cacheKey).Msg("cache: MISS")

	// Fetch from wrapped transport
	resp, err := c.wrapped.Acquire(ctx, req)
	if err != nil {
		return nil, err
	}

	// If this is a packages file and we got content, cache it
	if isPackagesFile(req.URI) && resp.Content != nil {
		log.Debug().Str("uri", req.URI.String()).Str("cache_key", cacheKey).Msg("cache: storing response")
		return c.cacheResponse(resp, cachePath, req)
	}

	return resp, nil
}

// PurgeCache removes all files from the cache directory
func (c *CacheTransport) PurgeCache() error {
	if c.disabled {
		log.Debug().Msg("cache: purge skipped, caching disabled")
		return nil
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("cache_dir", c.cacheDir).Msg("cache: purge skipped, cache directory doesn't exist")
			return nil // Cache directory doesn't exist, nothing to purge
		}
		return err
	}

	var purged int
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gz") {
			path := filepath.Join(c.cacheDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return err
			}
			purged++
		}
	}

	log.Debug().Str("cache_dir", c.cacheDir).Int("files_removed", purged).Msg("cache: purged")
	return nil
}

func (c *CacheTransport) getCacheKey(uri *url.URL) string {
	// Use MD5 hash of the URI as cache key
	hash := md5.Sum([]byte(uri.String()))
	return fmt.Sprintf("%x", hash)
}

func (c *CacheTransport) loadFromCache(cachePath string, req *AcquireRequest) (*AcquireResponse, error) {
	file, err := os.Open(cachePath)
	if err != nil {
		return nil, err // Cache miss
	}
	defer file.Close()

	// Get file info for last modified time
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}

	// Read the cached content
	content, err := io.ReadAll(gzipReader)
	if err != nil {
		gzipReader.Close()
		return nil, err
	}
	gzipReader.Close()

	// Create response with cached content
	modTime := info.ModTime()
	resp := &AcquireResponse{
		URI:          req.URI,
		Content:      io.NopCloser(strings.NewReader(string(content))),
		Size:         int64(len(content)),
		LastModified: &modTime,
		Headers:      make(map[string]string),
	}

	// Calculate MD5 hash of the content for verification
	hash := md5.Sum(content)
	resp.Hashes = map[string]string{
		"md5": fmt.Sprintf("%x", hash),
	}

	return resp, nil
}

func (c *CacheTransport) cacheResponse(resp *AcquireResponse, cachePath string, req *AcquireRequest) (*AcquireResponse, error) {
	// Read all content from the response
	content, err := io.ReadAll(resp.Content)
	if err != nil {
		return nil, err
	}
	resp.Content.Close()

	// Create cache file
	file, err := os.Create(cachePath)
	if err != nil {
		// If caching fails, still return the response
		resp.Content = io.NopCloser(strings.NewReader(string(content)))
		return resp, nil
	}
	defer file.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	// Write compressed content to cache
	if _, err := gzipWriter.Write(content); err != nil {
		// If caching fails, still return the response
		resp.Content = io.NopCloser(strings.NewReader(string(content)))
		return resp, nil
	}

	// Update response with new content reader
	resp.Content = io.NopCloser(strings.NewReader(string(content)))
	resp.Size = int64(len(content))

	// Update hash if not already set
	if resp.Hashes == nil {
		resp.Hashes = make(map[string]string)
	}
	if _, exists := resp.Hashes["md5"]; !exists {
		hash := md5.Sum(content)
		resp.Hashes["md5"] = fmt.Sprintf("%x", hash)
	}

	return resp, nil
}

func getDefaultCacheDir() string {
	// Try XDG_CACHE_HOME first
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "apt-look")
	}

	// Fall back to ~/.cache/apt-look
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".cache", "apt-look")
	}

	// Last resort: current directory
	return filepath.Join(".", ".cache", "apt-look")
}

func isReleaseFile(uri *url.URL) bool {
	path := strings.ToLower(uri.Path)
	return strings.HasSuffix(path, "/release") ||
		strings.HasSuffix(path, "/release.gpg") ||
		strings.HasSuffix(path, "/inrelease")
}

func isPackagesFile(uri *url.URL) bool {
	path := strings.ToLower(uri.Path)
	return strings.Contains(path, "/packages") ||
		strings.HasSuffix(path, "/packages.gz") ||
		strings.HasSuffix(path, "/packages.bz2") ||
		strings.HasSuffix(path, "/packages.xz")
}

// GetStats returns the cache statistics
func (c *CacheTransport) GetStats() *CacheStats {
	return c.stats
}