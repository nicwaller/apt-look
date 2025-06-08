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

	var record Record
	found := false
	for r, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		record = r
		found = true
		break
	}

	require.True(t, found, "No records found")

	// Test field access
	assert.Equal(t, "test-item", record.Get("Name"))
	assert.Equal(t, "1.0.0", record.Get("Value"))
	assert.Equal(t, "example", record.Get("Type"))

	expectedComment := "A test field This is a multi-line comment with additional details."
	assert.Equal(t, expectedComment, record.Get("Comment"))

	// Test case-insensitive access
	assert.Equal(t, "test-item", record.Get("name"))

	// Test field ordering preservation
	fields := record.Fields()
	expectedOrder := []string{"Name", "Value", "Type", "Comment"}
	assert.Equal(t, expectedOrder, fields)
}

func TestMultipleRecords(t *testing.T) {
	input := `Name: item1
Value: 1.0.0

Name: item2
Value: 2.0.0`

	parser := NewParser()
	var records []Record
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		records = append(records, record)
	}

	assert.Len(t, records, 2)
	assert.Equal(t, "item1", records[0].Get("Name"))
	assert.Equal(t, "item2", records[1].Get("Name"))
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

	var record Record
	found := false
	for r, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		record = r
		found = true
		break
	}
	require.True(t, found)

	// Convert back to control format and verify byte-for-byte identical
	var sb strings.Builder
	_, _ = record.Write(&sb)
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
		{"field name starting with hash", `#Field: value`},
		{"field name starting with dash", `-Field: value`},
		{"field name with control char", "Field\x01: value"},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, err := range parser.ParseRecords(strings.NewReader(tt.input)) {
				assert.Error(t, err, "Expected error for invalid field name: %s", tt.input)
				break
			}
		})
	}
}

func TestDuplicateFields(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0
Name: duplicate-item`

	parser := NewParser()
	for _, err := range parser.ParseRecords(strings.NewReader(input)) {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate field")
		break
	}
}

func TestIteratorInterface(t *testing.T) {
	input := `Name: item1
Value: 1.0.0

Name: item2
Value: 2.0.0

Name: item3
Value: 3.0.0`

	parser := NewParser()

	// Test early termination
	count := 0
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		count++
		if count == 2 {
			break
		}
		assert.True(t, record.Has("Name"))
	}

	assert.Equal(t, 2, count)
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

