package rfc822

import (
	"io"
	"strings"
)

// Lookup retrieves a field value from a record (case-insensitive)
// Returns the value and whether the field was found, following Go map idiom
func (r Record) Lookup(field string) (Field, bool) {
	// Try exact match first
	for _, f := range r {
		if f.Name == field {
			return f, true
		}
	}

	// Try case-insensitive match
	for _, f := range r {
		if strings.EqualFold(f.Name, field) {
			return f, true
		}
	}

	return Field{}, false
}

// Get retrieves a field value from a record (case-insensitive) as a single string
// Returns empty string if field doesn't exist
func (r Record) Get(fieldName string) string {
	field, exists := r.Lookup(fieldName)
	if !exists {
		return ""
	}
	return field.Value.Unfold()
}

// GetLines retrieves a field value from a record (case-insensitive) as lines
// Returns empty slice if field doesn't exist
func (r Record) GetLines(fieldName string) FieldValues {
	field, exists := r.Lookup(fieldName)
	if !exists {
		return FieldValues{}
	}
	return field.Value
}

// Has checks if a field exists in a record (case-insensitive)
func (r Record) Has(field string) bool {
	_, exists := r.Lookup(field)
	return exists
}

// Fields returns all field names in the record
func (r Record) Fields() []string {
	fields := make([]string, 0, len(r))
	for _, f := range r {
		fields = append(fields, f.Name)
	}
	return fields
}

func (r Record) Write(writer io.Writer) (int, error) {
	if len(r) == 0 {
		return 0, nil
	}

	var sb strings.Builder
	for _, field := range r {
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
