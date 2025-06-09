package apttransport

import (
	"context"
	"errors"
	"sync"
)

var DefaultRegistry = NewRegistryWithCache(CacheConfig{})

func init() {
	// TODO: do this better
	DefaultRegistry.Register(NewHTTPTransport())
	DefaultRegistry.Register(NewFileTransport())
}

// Registry manages multiple transport implementations with optional caching
type Registry struct {
	transports       map[string]Transport
	cachedTransports map[string]*CacheTransport
	cacheConfig      CacheConfig
	mu               sync.RWMutex
}

// NewRegistry creates a new transport registry
func NewRegistry() *Registry {
	return &Registry{
		transports:       make(map[string]Transport),
		cachedTransports: make(map[string]*CacheTransport),
	}
}

// NewRegistryWithCache creates a new transport registry with caching configuration
func NewRegistryWithCache(config CacheConfig) *Registry {
	return &Registry{
		transports:       make(map[string]Transport),
		cachedTransports: make(map[string]*CacheTransport),
		cacheConfig:      config,
	}
}

// Register adds a transport for a specific scheme
func (r *Registry) Register(transport Transport) {
	for _, scheme := range transport.Schemes() {
		r.transports[scheme] = transport
	}
}

// SetCacheConfig updates the caching configuration
func (r *Registry) SetCacheConfig(config CacheConfig) {
	r.cacheConfig = config
}

func (r *Registry) Select(scheme string) (Transport, error) {
	t, ok := r.transports[scheme]
	if !ok {
		return nil, errors.New("invalid transport")
	}
	return t, nil
}

func (r *Registry) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	r.mu.RLock()
	transport, exists := r.transports[req.URI.Scheme]
	r.mu.RUnlock()

	if !exists {
		return nil, &UnsupportedSchemeError{Scheme: req.URI.Scheme}
	}

	// Use cached transport if caching is enabled
	if !r.cacheConfig.Disabled {
		r.mu.Lock()
		cachedTransport, cached := r.cachedTransports[req.URI.Scheme]
		if !cached {
			// Create and store cached transport for this scheme
			newCachedTransport, err := NewCacheTransport(transport, r.cacheConfig)
			if err != nil {
				r.mu.Unlock()
				// If cache setup fails, fall back to uncached transport
				return transport.Acquire(ctx, req)
			}
			r.cachedTransports[req.URI.Scheme] = newCachedTransport
			cachedTransport = newCachedTransport
		}
		r.mu.Unlock()
		return cachedTransport.Acquire(ctx, req)
	}

	return transport.Acquire(ctx, req)
}

// PurgeCache removes all cached files (if caching is enabled)
func (r *Registry) PurgeCache() error {
	if r.cacheConfig.Disabled {
		return nil
	}

	// Create a temporary cache transport just to access the purge functionality
	dummyTransport := NewHTTPTransport() // Any transport will do
	cacheTransport, err := NewCacheTransport(dummyTransport, r.cacheConfig)
	if err != nil {
		return err
	}

	return cacheTransport.PurgeCache()
}

// GetCacheStats returns aggregate cache statistics across all cached transports
func (r *Registry) GetCacheStats() (hits, misses int64, hitRatio float64) {
	if r.cacheConfig.Disabled {
		return 0, 0, 0.0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	totalHits := int64(0)
	totalMisses := int64(0)

	for _, cachedTransport := range r.cachedTransports {
		h, m := cachedTransport.GetStats().GetStats()
		totalHits += h
		totalMisses += m
	}

	total := totalHits + totalMisses
	if total == 0 {
		return totalHits, totalMisses, 0.0
	}

	return totalHits, totalMisses, float64(totalHits) / float64(total)
}
