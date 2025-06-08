package rfc822

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Parser parses RFC822-style messages
type Parser struct{}

// NewParser creates a new RFC822-style message parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseHeader parses a single RFC822 header section and returns it as a Header
func (p *Parser) ParseHeader(r io.Reader) (Header, error) {
	scanner := bufio.NewScanner(r)
	var header Header
	var currentField string
	var currentValue strings.Builder

	flushCurrentField := func() {
		if currentField != "" {
			value := strings.TrimSpace(currentValue.String())
			lines := strings.Split(value, "\n")
			header = append(header, Field{
				Name:  currentField,
				Value: lines,
			})
			currentField = ""
			currentValue.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment lines (start with '#')
		// This is not technically part of RFC822; comment lines are introduced by deb822
		// But it's way easier to just implement here.
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}

		// Empty line indicates end of header section
		if strings.TrimSpace(line) == "" {
			break
		}

		// Continuation line (starts with space or tab)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if currentField == "" {
				return nil, fmt.Errorf("continuation line without field: %q", line)
			}
			// Remove leading whitespace and add to current value
			currentValue.WriteString("\n")
			currentValue.WriteString(strings.TrimLeft(line, " \t"))
			continue
		}

		// New field line
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field line: %q", line)
		}

		// Flush previous field
		flushCurrentField()

		// Validate field name
		fieldName := strings.TrimSpace(parts[0])
		if err := p.validateFieldName(fieldName); err != nil {
			return nil, fmt.Errorf("invalid field name %q: %w", fieldName, err)
		}

		// Check for duplicate field in current header
		if header.Has(fieldName) {
			return nil, fmt.Errorf("duplicate field %q in header", fieldName)
		}

		currentField = fieldName
		value := strings.TrimLeft(parts[1], " \t")
		currentValue.WriteString(value)
	}

	// Flush any remaining field
	flushCurrentField()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return header, nil
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
