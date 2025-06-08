package apttransport

import "context"

// Registry manages multiple transport implementations with optional caching
type Registry struct {
	transports  map[string]Transport
	cacheConfig CacheConfig
}

// NewRegistry creates a new transport registry
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]Transport),
	}
}

// NewRegistryWithCache creates a new transport registry with caching configuration
func NewRegistryWithCache(config CacheConfig) *Registry {
	return &Registry{
		transports:  make(map[string]Transport),
		cacheConfig: config,
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

// Acquire automatically selects the appropriate transport and fetches the resource
func (r *Registry) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	transport, exists := r.transports[req.URI.Scheme]
	if !exists {
		return nil, &UnsupportedSchemeError{Scheme: req.URI.Scheme}
	}

	// Wrap transport with caching if not disabled
	if !r.cacheConfig.Disabled {
		cachedTransport, err := NewCacheTransport(transport, r.cacheConfig)
		if err != nil {
			// If cache setup fails, fall back to uncached transport
			return transport.Acquire(ctx, req)
		}
		transport = cachedTransport
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
