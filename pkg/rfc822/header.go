package rfc822

import (
	"io"
	"strings"
)

// Lookup retrieves a field value from a header (case-insensitive)
// Returns the value and whether the field was found, following Go map idiom
func (h Header) Lookup(field string) (Field, bool) {
	// Try exact match first
	for _, f := range h {
		if f.Name == field {
			return f, true
		}
	}

	// Try case-insensitive match
	for _, f := range h {
		if strings.EqualFold(f.Name, field) {
			return f, true
		}
	}

	return Field{}, false
}

// Get retrieves a field value from a header (case-insensitive) as a single string
// Returns empty string if field doesn't exist
func (h Header) Get(fieldName string) string {
	field, exists := h.Lookup(fieldName)
	if !exists {
		return ""
	}
	return field.Value.Unfold()
}

// GetLines retrieves a field value from a header (case-insensitive) as lines
// Returns empty slice if field doesn't exist
func (h Header) GetLines(fieldName string) FieldValues {
	field, exists := h.Lookup(fieldName)
	if !exists {
		return FieldValues{}
	}
	return field.Value
}

// Has checks if a field exists in a header (case-insensitive)
func (h Header) Has(field string) bool {
	_, exists := h.Lookup(field)
	return exists
}

// Fields returns all field names in the header
func (h Header) Fields() []string {
	fields := make([]string, 0, len(h))
	for _, f := range h {
		fields = append(fields, f.Name)
	}
	return fields
}

func (h Header) Write(writer io.Writer) (int, error) {
	if len(h) == 0 {
		return 0, nil
	}

	var sb strings.Builder
	for _, field := range h {
		sb.WriteString(field.Name)
		sb.WriteString(": ")

		// Handle multi-line values
		if len(field.Value) > 0 {
			sb.WriteString(field.Value[0])
			sb.WriteString("\n")

			// Add continuation lines with proper indentation
			for _, line := range field.Value[1:] {
				sb.WriteString(" ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("\n")
		}
	}

	return writer.Write([]byte(sb.String()))
}
