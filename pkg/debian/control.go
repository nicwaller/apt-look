package debian

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"regexp"
	"strings"
)

// Field represents a single field in a Debian control format file
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (f Field) String() string {
	return fmt.Sprintf("%s: %s", f.Name, f.Value)
}

func (f Field) GoString() string {
	return fmt.Sprintf("debian.Field{Name: %q, Value: %q}", f.Name, f.Value)
}

// Record represents a single record (paragraph) in a Debian control format file
// Fields are stored in a slice to preserve the original ordering for round-trip conversion
type Record []Field

// Parser parses Debian control format files
type Parser struct{}

// NewParser creates a new Debian control format parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseRecords returns an iterator over records from a Debian control format file
func (p *Parser) ParseRecords(r io.Reader) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		if err := p.parseRecords(r, yield); err != nil {
			yield(nil, err)
		}
	}
}

// parseRecords contains the actual parsing logic
func (p *Parser) parseRecords(r io.Reader, yield func(Record, error) bool) error {
	scanner := bufio.NewScanner(r)
	var currentRecord Record
	var currentField string
	var currentValue strings.Builder

	flushCurrentField := func() {
		if currentField != "" {
			value := strings.TrimSpace(currentValue.String())
			currentRecord = append(currentRecord, Field{
				Name:  currentField,
				Value: value,
			})
			currentField = ""
			currentValue.Reset()
		}
	}

	flushCurrentRecord := func() bool {
		flushCurrentField()
		if len(currentRecord) > 0 {
			if !yield(currentRecord, nil) {
				return false // Stop iteration if yield returns false
			}
			currentRecord = Record{}
		}
		return true
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment lines (start with '#')
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}

		// Empty line indicates end of record
		if strings.TrimSpace(line) == "" {
			if !flushCurrentRecord() {
				return nil // Stop iteration if yield returned false
			}
			continue
		}

		// Continuation line (starts with space or tab)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if currentField == "" {
				return fmt.Errorf("continuation line without field: %q", line)
			}
			// Remove leading whitespace and add to current value
			currentValue.WriteString("\n")
			currentValue.WriteString(strings.TrimLeft(line, " \t"))
			continue
		}

		// New field line
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid field line: %q", line)
		}

		// Flush previous field
		flushCurrentField()

		// Validate field name
		fieldName := strings.TrimSpace(parts[0])
		if err := p.validateFieldName(fieldName); err != nil {
			return fmt.Errorf("invalid field name %q: %w", fieldName, err)
		}

		// Check for duplicate field in current record
		if currentRecord.Has(fieldName) {
			return fmt.Errorf("duplicate field %q in record", fieldName)
		}

		currentField = fieldName
		value := strings.TrimLeft(parts[1], " \t")
		currentValue.WriteString(value)
	}

	// Flush any remaining field and record
	flushCurrentRecord()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// validateFieldName checks if a field name is valid according to Debian policy
func (p *Parser) validateFieldName(name string) error {
	// Field names must not be empty
	if name == "" {
		return fmt.Errorf("field name cannot be empty")
	}

	// Field names must not start with '#' or '-'
	if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "-") {
		return fmt.Errorf("field name cannot start with '#' or '-'")
	}

	// Field names must use only US-ASCII characters, excluding control characters, spaces, and colons
	validFieldName := regexp.MustCompile(`^[!-9;-~]+$`) // ASCII printable chars except space (0x20) and colon (0x3A)
	if !validFieldName.MatchString(name) {
		return fmt.Errorf("field name contains invalid characters (must be US-ASCII excluding control chars, spaces, and colons)")
	}

	return nil
}

// Lookup retrieves a field value from a record (case-insensitive)
// Returns the value and whether the field was found, following Go map idiom
func (r Record) Lookup(field string) (string, bool) {
	// Try exact match first
	for _, f := range r {
		if f.Name == field {
			return f.Value, true
		}
	}

	// Try case-insensitive match
	for _, f := range r {
		if strings.EqualFold(f.Name, field) {
			return f.Value, true
		}
	}

	return "", false
}

// Get retrieves a field value from a record (case-insensitive)
// Returns empty string if field doesn't exist
func (r Record) Get(field string) string {
	value, _ := r.Lookup(field)
	return value
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
		lines := strings.Split(field.Value, "\n")
		sb.WriteString(lines[0])
		sb.WriteString("\n")

		// Add continuation lines with proper indentation
		for _, line := range lines[1:] {
			sb.WriteString(" ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return writer.Write([]byte(sb.String()))
}
