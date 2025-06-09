package apttransport

import (
	"context"
	"io"
	"net/url"
	"time"
)

// Transport represents an APT transport method (http, https, ftp, file, etc.)
type Transport interface {
	// Schemes returns the archiveRoot scheme this transport handles (e.g., "http", "https", "ftp")
	Schemes() []string

	// Acquire fetches a resource from the given archiveRoot
	Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error)
}

// AcquireRequest represents a request to fetch a resource
type AcquireRequest struct {
	// URI is the resource to fetch
	URI *url.URL

	// Filename is where to save the downloaded file (optional)
	Filename string

	// LastModified for conditional requests (optional)
	LastModified *time.Time

	// ExpectedSize for validation (optional, 0 means unknown)
	ExpectedSize int64

	// ExpectedHashes for integrity verification (optional)
	ExpectedHashes map[string]string // algorithm -> hash

	// Headers for additional request headers
	Headers map[string]string

	// Timeout for the request
	Timeout time.Duration

	// ProgressCallback for reporting download progress (optional)
	ProgressCallback func(downloaded, total int64)
}

// AcquireResponse represents the result of an acquire operation
type AcquireResponse struct {
	// URI that was actually fetched (may differ due to redirects)
	URI *url.URL

	// Filename where the content was saved (if requested)
	Filename string

	// Content provides direct access to the downloaded data
	Content io.ReadCloser

	// Size of the downloaded content
	Size int64

	// LastModified timestamp from the server
	LastModified *time.Time

	// Hashes of the downloaded content
	Hashes map[string]string // algorithm -> hash

	// Headers from the response
	Headers map[string]string
}

type AcquireError struct {
	URI    *url.URL
	Reason string
	Err    error
}

func (e *AcquireError) Error() string {
	return "failed to acquire " + e.URI.String() + ": " + e.Reason
}

func (e *AcquireError) Unwrap() error {
	return e.Err
}

type UnsupportedSchemeError struct {
	Scheme string
}

func (e *UnsupportedSchemeError) Error() string {
	return "unsupported scheme: " + e.Scheme
}
