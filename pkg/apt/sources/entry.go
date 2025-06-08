package sources

import (
	"fmt"
	"net/url"
	"strings"
)

// SourceType represents the type of APT source entry
type SourceType string

const (
	SourceTypeDeb     SourceType = "deb"     // Binary packages
	SourceTypeSrc     SourceType = "deb-src" // Source packages
	SourceTypeUnknown SourceType = "unknown"
)

// Entry represents a single APT source entry from sources.list
type Entry struct {
	// Entry type (deb or deb-src)
	Type SourceType `json:"type"`

	// Repository URI
	URI string `json:"uri"`

	// Distribution/Suite (e.g., "stable", "jammy", "bookworm")
	Distribution string `json:"distribution"`

	// Components (e.g., "main", "contrib", "non-free")
	Components []string `json:"components,omitempty"`

	// Options in square brackets (e.g., arch=amd64, trusted=yes)
	Options map[string]string `json:"options,omitempty"`

	// Original line text for reference
	originalLine string

	// Line number in the source file
	LineNumber int `json:"line_number,omitempty"`
}

// validateURI validates that the URI is well-formed
func validateURI(uri string) error {
	// Basic URI validation
	if uri == "" {
		return fmt.Errorf("URI cannot be empty")
	}

	// Handle special cases
	if uri == "/" {
		return nil // Root directory is valid for some contexts
	}

	// Try to parse as URL
	if _, err := url.Parse(uri); err != nil {
		return fmt.Errorf("malformed URI: %w", err)
	}

	return nil
}

// isSourceLine checks if a line looks like a source line (starts with deb or deb-src)
func isSourceLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}

	// Check if it starts with a known source type (accounting for options)
	firstField := fields[0]

	// Handle options in square brackets
	if strings.HasPrefix(firstField, "[") {
		// Find the actual type after options
		for _, field := range fields {
			if !strings.HasPrefix(field, "[") && !strings.HasSuffix(field, "]") {
				firstField = field
				break
			}
		}
	}

	return parseSourceType(firstField) != SourceTypeUnknown
}
