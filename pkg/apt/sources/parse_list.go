package sources

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ParseSourcesList parses an entire sources.list file into a slice of entries
func ParseSourcesList(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip commented lines
		if strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseSourceLine(line, lineNumber)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		entries = append(entries, *entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return entries, nil
}

// parseSourceLine parses a single line from sources.list
func parseSourceLine(line string, lineNumber int) (*Entry, error) {
	originalLine := line
	line = strings.TrimSpace(line)

	if line == "" {
		return nil, errors.New("empty line")
	}

	if strings.HasPrefix(line, "#") {
		return nil, errors.New("commented line")
	}

	// Parse options in square brackets (they come after the source type)
	options := make(map[string]string)
	optionsRegex := regexp.MustCompile(`^(\S+)\s+\[([^]]+)]\s*(.*)`)
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
		return &Entry{
			originalLine: originalLine,
			LineNumber:   lineNumber,
		}, fmt.Errorf("invalid source line format: expected at least 3 fields (type, uri, distribution)")
	}

	// Parse source type
	sourceType := parseSourceType(fields[0])
	if sourceType == SourceTypeUnknown {
		return &Entry{
			originalLine: originalLine,
			LineNumber:   lineNumber,
		}, fmt.Errorf("unknown source type: %s", fields[0])
	}

	// Parse URI
	uri := fields[1]
	if err := validateURI(uri); err != nil {
		return &Entry{
			Type:         sourceType,
			URI:          uri,
			originalLine: originalLine,
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

	return &Entry{
		Type:         sourceType,
		URI:          uri,
		Distribution: distribution,
		Components:   components,
		Options:      options,
		originalLine: originalLine,
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
