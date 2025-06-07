package debian

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Record represents a single record (paragraph) in a Debian control format file
type Record map[string]string

// Parser parses Debian control format files
type Parser struct{}

// NewParser creates a new Debian control format parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseRecord parses a single record from a Debian control format file
func (p *Parser) ParseRecord(r io.Reader) (Record, error) {
	records, err := p.ParseRecords(r)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, io.EOF
	}
	return records[0], nil
}

// ParseRecords parses multiple records from a Debian control format file
func (p *Parser) ParseRecords(r io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(r)
	var records []Record
	currentRecord := make(Record)
	var currentField string
	var currentValue strings.Builder

	flushCurrentField := func() {
		if currentField != "" {
			value := strings.TrimSpace(currentValue.String())
			currentRecord[currentField] = value
			currentField = ""
			currentValue.Reset()
		}
	}

	flushCurrentRecord := func() {
		flushCurrentField()
		if len(currentRecord) > 0 {
			records = append(records, currentRecord)
			currentRecord = make(Record)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment lines (start with '#')
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}

		// Empty line indicates end of record
		if strings.TrimSpace(line) == "" {
			flushCurrentRecord()
			continue
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

		// Validate and normalize field name
		fieldName := strings.TrimSpace(parts[0])
		if err := p.validateFieldName(fieldName); err != nil {
			return nil, fmt.Errorf("invalid field name %q: %w", fieldName, err)
		}
		normalizedField := p.normalizeFieldName(fieldName)
		
		// Check for duplicate field in current record
		if currentRecord.Has(normalizedField) {
			return nil, fmt.Errorf("duplicate field %q in record", normalizedField)
		}
		
		currentField = normalizedField
		value := strings.TrimLeft(parts[1], " \t")
		currentValue.WriteString(value)
	}

	// Flush any remaining field and record
	flushCurrentRecord()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return records, nil
}

// normalizeFieldName converts field names to standard case for consistent storage
func (p *Parser) normalizeFieldName(name string) string {
	// Convert to lowercase for comparison, but preserve standard casing
	lower := strings.ToLower(name)
	
	// Standard field name mappings (case-insensitive input -> standard case output)
	standardNames := map[string]string{
		"package":        "Package",
		"version":        "Version",
		"architecture":   "Architecture",
		"maintainer":     "Maintainer",
		"depends":        "Depends",
		"recommends":     "Recommends",
		"suggests":       "Suggests",
		"conflicts":      "Conflicts",
		"breaks":         "Breaks",
		"replaces":       "Replaces",
		"provides":       "Provides",
		"enhances":       "Enhances",
		"pre-depends":    "Pre-Depends",
		"installed-size": "Installed-Size",
		"homepage":       "Homepage",
		"description":    "Description",
		"tag":            "Tag",
		"section":        "Section",
		"priority":       "Priority",
		"essential":      "Essential",
		"origin":         "Origin",
		"label":          "Label",
		"suite":          "Suite",
		"codename":       "Codename",
		"date":           "Date",
		"valid-until":    "Valid-Until",
		"architectures":  "Architectures",
		"components":     "Components",
		"md5sum":         "MD5sum",
		"sha1":           "SHA1",
		"sha256":         "SHA256",
		"sha512":         "SHA512",
		"filename":       "Filename",
		"size":           "Size",
	}
	
	if standard, exists := standardNames[lower]; exists {
		return standard
	}
	
	// For unknown fields, preserve original case
	return name
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

// Get retrieves a field value from a record (case-insensitive)
func (r Record) Get(field string) string {
	// Try exact match first
	if value, exists := r[field]; exists {
		return value
	}
	
	// Try case-insensitive match
	for k, v := range r {
		if strings.EqualFold(k, field) {
			return v
		}
	}
	
	return ""
}

// Has checks if a field exists in a record (case-insensitive)
func (r Record) Has(field string) bool {
	// Try exact match first
	if _, exists := r[field]; exists {
		return true
	}
	
	// Try case-insensitive match
	for k := range r {
		if strings.EqualFold(k, field) {
			return true
		}
	}
	
	return false
}

// Fields returns all field names in the record
func (r Record) Fields() []string {
	fields := make([]string, 0, len(r))
	for field := range r {
		fields = append(fields, field)
	}
	return fields
}