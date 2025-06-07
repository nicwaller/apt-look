package apttransport

import "context"

// Registry manages multiple transport implementations
type Registry map[string]Transport

// NewRegistry creates a new transport registry
func NewRegistry() Registry {
	return make(map[string]Transport)
}

// Register adds a transport for a specific scheme
func (r Registry) Register(transport Transport) {
	for _, scheme := range transport.Schemes() {
		r[scheme] = transport
	}
}

// Acquire automatically selects the appropriate transport and fetches the resource
func (r Registry) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	transport, exists := r[req.URI.Scheme]
	if !exists {
		return nil, &UnsupportedSchemeError{Scheme: req.URI.Scheme}
	}

	return transport.Acquire(ctx, req)
}
