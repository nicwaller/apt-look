package rfc822

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"regexp"
	"strings"
)

// Parser parses RFC822-style messages
type Parser struct{}

// NewParser creates a new RFC822-style message parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseRecords returns an iterator over records from an RFC822-style message
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
			lines := strings.Split(value, "\n")
			currentRecord = append(currentRecord, Field{
				Name:  currentField,
				Value: lines,
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
		// This is not technically part of RFC822; comment lines are introduced by deb822
		// But it's way easier to just implement here.
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

// validateFieldName checks if a field name is valid according to RFC822 rules
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
