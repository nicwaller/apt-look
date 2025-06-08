package rfc822

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSimpleRecord(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0
Type: example
Comment: A test field
 This is a multi-line comment
 with additional details.`

	parser := NewParser()

	header, err := parser.ParseHeader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotEmpty(t, header, "No header found")

	// Test field access
	assert.Equal(t, "test-item", header.Get("Name"))
	assert.Equal(t, "1.0.0", header.Get("Value"))
	assert.Equal(t, "example", header.Get("Type"))

	expectedComment := "A test field This is a multi-line comment with additional details."
	assert.Equal(t, expectedComment, header.Get("Comment"))

	// Test case-insensitive access
	assert.Equal(t, "test-item", header.Get("name"))

	// Test field ordering preservation
	fields := header.Fields()
	expectedOrder := []string{"Name", "Value", "Type", "Comment"}
	assert.Equal(t, expectedOrder, fields)
}

func TestHeaderStopsAtBlankLine(t *testing.T) {
	// RFC 822 header parsing should stop at the first blank line
	input := `Name: item1
Value: 1.0.0

Name: item2
Value: 2.0.0`

	parser := NewParser()
	header, err := parser.ParseHeader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotEmpty(t, header, "No header found")

	// Should only parse the first header before the blank line
	assert.Equal(t, "item1", header.Get("Name"))
	assert.Equal(t, "1.0.0", header.Get("Value"))
	
	// Should not contain the second header
	assert.False(t, header.Has("item2"))
}

func TestControlFormatRoundTrip(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0
Type: example
Comment: A test field
 This is a multi-line comment
 with additional details.
`

	parser := NewParser()

	header, err := parser.ParseHeader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotEmpty(t, header, "No header found")

	// Convert back to control format and verify byte-for-byte identical
	var sb strings.Builder
	_, _ = header.Write(&sb)
	output := sb.String()
	assert.Equal(t, input, output, "Round-trip conversion not identical")
}

func TestFieldValidation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty field name", `: value`},
		{"field name with space", `Invalid Field: value`},
		{"field name starting with dash", `-Field: value`},
		{"field name with control char", "Field\x01: value"},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseHeader(strings.NewReader(tt.input))
			assert.Error(t, err, "Expected error for invalid field name: %s", tt.input)
		})
	}
}

func TestDuplicateFields(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0
Name: duplicate-item`

	parser := NewParser()
	_, err := parser.ParseHeader(strings.NewReader(input))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate field")
}

func TestFieldStringMethods(t *testing.T) {
	field := Field{Name: "Name", Value: []string{"test-item"}}

	// Test String() method (used by %v)
	expectedString := "Name: test-item"
	assert.Equal(t, expectedString, field.String())
	assert.Equal(t, expectedString, fmt.Sprintf("%v", field))

	// Test with multi-line value
	multilineField := Field{Name: "Comment", Value: []string{"First line", "Second line"}}
	expectedMultilineString := "Comment: First line Second line"

	assert.Equal(t, expectedMultilineString, multilineField.String())
}

