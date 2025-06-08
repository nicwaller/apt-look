package aptrepo

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"net/url"
	"regexp"
	"strings"
)

// SourceType represents the type of APT source entry
type SourceType string

const (
	SourceTypeDeb     SourceType = "deb"     // Binary packages
	SourceTypeSrc     SourceType = "deb-src" // Source packages
	SourceTypeUnknown SourceType = "unknown"
)

// SourceEntry represents a single APT source entry from sources.list
type SourceEntry struct {
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

	// Whether this entry is enabled (true) or commented out (false)
	Enabled bool `json:"enabled"`

	// Original line text for reference
	OriginalLine string `json:"original_line,omitempty"`

	// Line number in the source file
	LineNumber int `json:"line_number,omitempty"`
}

// SourcesList represents a collection of APT source entries
type SourcesList struct {
	Entries []SourceEntry `json:"entries"`
}

// ParseSources parses APT sources.list format and returns an iterator over source entries
func ParseSources(r io.Reader) iter.Seq2[*SourceEntry, error] {
	return func(yield func(*SourceEntry, error) bool) {
		scanner := bufio.NewScanner(r)
		lineNumber := 0

		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()

			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			entry, err := parseSourceLine(line, lineNumber)
			if err != nil {
				yield(nil, fmt.Errorf("line %d: %w", lineNumber, err))
				return
			}

			// Skip lines that are just comments
			if entry == nil {
				continue
			}

			if !yield(entry, nil) {
				return // Stop iteration if yield returns false
			}
		}

		if err := scanner.Err(); err != nil {
			yield(nil, fmt.Errorf("scanner error: %w", err))
		}
	}
}

// ParseSourcesList parses an entire sources.list file into a SourcesList structure
func ParseSourcesList(r io.Reader) (*SourcesList, error) {
	var entries []SourceEntry

	for entry, err := range ParseSources(r) {
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}

	return &SourcesList{Entries: entries}, nil
}

// parseSourceLine parses a single line from sources.list
func parseSourceLine(line string, lineNumber int) (*SourceEntry, error) {
	originalLine := line
	line = strings.TrimSpace(line)

	// Skip empty lines
	if line == "" {
		return nil, nil
	}

	// Check if line is commented out
	enabled := true
	if strings.HasPrefix(line, "#") {
		enabled = false
		line = strings.TrimSpace(line[1:]) // Remove # and trim

		// If it's just a comment without source info, skip it
		if line == "" || !isSourceLine(line) {
			return nil, nil
		}
	}

	// Parse options in square brackets (they come after the source type)
	options := make(map[string]string)
	optionsRegex := regexp.MustCompile(`^(\S+)\s+\[([^\]]+)\]\s*(.*)`)
	if match := optionsRegex.FindStringSubmatch(line); match != nil {
		// We have options: [type] [options] [rest]
		sourceType := match[1]
		optionsStr := match[2]
		rest := match[3]
		line = sourceType + " " + rest // Reconstruct without options

		for _, opt := range strings.Fields(optionsStr) {
			if parts := strings.SplitN(opt, "=", 2); len(parts) == 2 {
				options[parts[0]] = parts[1]
			} else {
				options[opt] = "true" // Options without values are treated as boolean true
			}
		}
	}

	// Split remaining line into fields
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return &SourceEntry{
			Enabled:      enabled,
			OriginalLine: originalLine,
			LineNumber:   lineNumber,
		}, fmt.Errorf("invalid source line format: expected at least 3 fields (type, uri, distribution)")
	}

	// Parse source type
	sourceType := parseSourceType(fields[0])
	if sourceType == SourceTypeUnknown {
		return &SourceEntry{
			Enabled:      enabled,
			OriginalLine: originalLine,
			LineNumber:   lineNumber,
		}, fmt.Errorf("unknown source type: %s", fields[0])
	}

	// Parse URI
	uri := fields[1]
	if err := validateURI(uri); err != nil {
		return &SourceEntry{
			Type:         sourceType,
			URI:          uri,
			Enabled:      enabled,
			OriginalLine: originalLine,
			LineNumber:   lineNumber,
		}, fmt.Errorf("invalid URI: %w", err)
	}

	// Parse distribution
	distribution := fields[2]

	// Parse components (remaining fields)
	var components []string
	if len(fields) > 3 {
		components = fields[3:]
	}

	return &SourceEntry{
		Type:         sourceType,
		URI:          uri,
		Distribution: distribution,
		Components:   components,
		Options:      options,
		Enabled:      enabled,
		OriginalLine: originalLine,
		LineNumber:   lineNumber,
	}, nil
}

// parseSourceType converts string to SourceType
func parseSourceType(typeStr string) SourceType {
	switch strings.ToLower(typeStr) {
	case "deb":
		return SourceTypeDeb
	case "deb-src":
		return SourceTypeSrc
	default:
		return SourceTypeUnknown
	}
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

// GetEnabledEntries returns only enabled source entries
func (sl *SourcesList) GetEnabledEntries() []SourceEntry {
	var enabled []SourceEntry
	for _, entry := range sl.Entries {
		if entry.Enabled {
			enabled = append(enabled, entry)
		}
	}
	return enabled
}

// GetDisabledEntries returns only disabled (commented out) source entries
func (sl *SourcesList) GetDisabledEntries() []SourceEntry {
	var disabled []SourceEntry
	for _, entry := range sl.Entries {
		if !entry.Enabled {
			disabled = append(disabled, entry)
		}
	}
	return disabled
}

// GetByType returns entries of a specific type (deb or deb-src)
func (sl *SourcesList) GetByType(sourceType SourceType) []SourceEntry {
	var filtered []SourceEntry
	for _, entry := range sl.Entries {
		if entry.Type == sourceType {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// GetByURI returns entries matching a specific URI
func (sl *SourcesList) GetByURI(uri string) []SourceEntry {
	var filtered []SourceEntry
	for _, entry := range sl.Entries {
		if entry.URI == uri {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// HasComponent checks if an entry contains a specific component
func (se *SourceEntry) HasComponent(component string) bool {
	for _, comp := range se.Components {
		if comp == component {
			return true
		}
	}
	return false
}

// GetOption returns the value of a specific option, with a default value if not found
func (se *SourceEntry) GetOption(key, defaultValue string) string {
	if value, exists := se.Options[key]; exists {
		return value
	}
	return defaultValue
}

// HasOption checks if an entry has a specific option set
func (se *SourceEntry) HasOption(key string) bool {
	_, exists := se.Options[key]
	return exists
}

// String returns a string representation of the source entry in sources.list format
func (se *SourceEntry) String() string {
	var parts []string

	// Add comment prefix if disabled
	prefix := ""
	if !se.Enabled {
		prefix = "# "
	}

	// Add options if present
	optionsStr := ""
	if len(se.Options) > 0 {
		var opts []string
		for key, value := range se.Options {
			if value == "true" {
				opts = append(opts, key)
			} else {
				opts = append(opts, fmt.Sprintf("%s=%s", key, value))
			}
		}
		optionsStr = fmt.Sprintf("[%s] ", strings.Join(opts, " "))
	}

	// Build the line
	parts = append(parts, string(se.Type), se.URI, se.Distribution)
	parts = append(parts, se.Components...)

	return fmt.Sprintf("%s%s%s", prefix, optionsStr, strings.Join(parts, " "))
}
