package apttransport

import (
	"context"
	"time"
)

var _ Transport = &HTTPTransport{}

type HTTPTransport struct {
	userAgent string
	timeout   time.Duration
}

func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{
		userAgent: "apt-look/1.0",
		timeout:   time.Second * 60,
	}
}

func (t *HTTPTransport) Schemes() []string {
	return []string{"http", "https"}
}

func (t *HTTPTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	// TODO: Implement HTTP-specific logic
	// - Create HTTP request with proper headers
	// - Handle redirects
	// - Verify content hashes if provided
	// - Report progress via callback
	// - Handle conditional requests (If-Modified-Since)

	return nil, &AcquireError{
		URI:    req.URI,
		Reason: "not implemented",
		Err:    nil,
	}
}
