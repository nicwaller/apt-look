package sources

import (
	"fmt"
	"io"
	"strings"

	"github.com/nicwaller/apt-look/pkg/deb822"
)

// ParseDeb822SourcesList parses a deb822 sources file into a slice of entries
func ParseDeb822SourcesList(r io.Reader) ([]Entry, error) {
	var entries []Entry
	recordNumber := 0

	for header, err := range deb822.ParseRecords(r) {
		if err != nil {
			return nil, fmt.Errorf("parsing deb822 record: %w", err)
		}

		recordNumber++

		// Extract required fields
		typesField := header.Get("Types")
		urisField := header.Get("URIs")
		suitesField := header.Get("Suites")
		componentsField := header.Get("Components")

		if typesField == "" {
			return nil, fmt.Errorf("record %d: missing required field 'Types'", recordNumber)
		}
		if urisField == "" {
			return nil, fmt.Errorf("record %d: missing required field 'URIs'", recordNumber)
		}
		if suitesField == "" {
			return nil, fmt.Errorf("record %d: missing required field 'Suites'", recordNumber)
		}

		// Parse space-separated values
		types := strings.Fields(typesField)
		uris := strings.Fields(urisField)
		suites := strings.Fields(suitesField)
		var components []string
		if componentsField != "" {
			components = strings.Fields(componentsField)
		}

		// Build options map from other fields
		options := make(map[string]string)
		for _, field := range header {
			fieldName := strings.ToLower(field.Name)
			switch fieldName {
			case "types", "uris", "suites", "components":
				// Skip the main fields we've already processed
				continue
			case "enabled":
				options["enabled"] = field.Value.Unfold()
			case "signed-by":
				options["signed-by"] = field.Value.Unfold()
			case "trusted":
				options["trusted"] = field.Value.Unfold()
			case "arch":
				options["arch"] = field.Value.Unfold()
			case "lang":
				options["lang"] = field.Value.Unfold()
			default:
				// Include any other fields as options
				options[fieldName] = field.Value.Unfold()
			}
		}

		// Generate entries for each combination of type, URI, and suite
		for _, typeStr := range types {
			sourceType := parseSourceType(typeStr)
			if sourceType == SourceTypeUnknown {
				return nil, fmt.Errorf("record %d: unknown source type: %s", recordNumber, typeStr)
			}

			for _, uri := range uris {
				if err := validateURI(uri); err != nil {
					return nil, fmt.Errorf("record %d: invalid URI %s: %w", recordNumber, uri, err)
				}

				for _, suite := range suites {
					entry := Entry{
						Type:         sourceType,
						URI:          uri,
						Distribution: suite,
						Components:   components,
						Options:      options,
						LineNumber:   recordNumber,
					}

					entries = append(entries, entry)
				}
			}
		}
	}

	return entries, nil
}
