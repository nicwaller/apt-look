package sources

import (
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

	// Repository ArchiveRoot
	ArchiveRoot *url.URL `json:"archiveroot"`

	// Distribution/Suite (e.g., "stable", "jammy", "bookworm")
	// This may be "/" or "." if the archive uses a flat repository format
	// A flat repository does not use the dists hierarchy of directories,
	// and instead places meta index and indices directly into the archive root (or some part below it)
	// In sources.list syntax, a flat repository is specified like this:
	//    deb uri directory/
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
