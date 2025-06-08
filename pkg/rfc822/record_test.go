package rfc822

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAccessorMethods(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0`

	parser := NewParser()

	record, err := parser.ParseHeader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotEmpty(t, record, "No header found")

	// Test Lookup method
	field, exists := record.Lookup("Name")
	assert.True(t, exists)
	assert.Equal(t, "Name", field.Name)
	assert.Equal(t, FieldValues{"test-item"}, field.Value)

	field, exists = record.Lookup("NonExistent")
	assert.False(t, exists)
	assert.Empty(t, field.Name)
	assert.Empty(t, field.Value)

	// Test case-insensitive lookup
	field, exists = record.Lookup("name")
	assert.True(t, exists)
	assert.Equal(t, "Name", field.Name)
	assert.Equal(t, FieldValues{"test-item"}, field.Value)

	// Test Has method
	assert.True(t, record.Has("Name"))
	assert.True(t, record.Has("value")) // case-insensitive
	assert.False(t, record.Has("NonExistent"))

	// Test Get method
	assert.Equal(t, "test-item", record.Get("Name"))
	assert.Equal(t, "1.0.0", record.Get("value")) // case-insensitive
	assert.Empty(t, record.Get("NonExistent"))

	// Test GetLines method
	lines := record.GetLines("Name")
	assert.Equal(t, FieldValues{"test-item"}, lines)
	lines = record.GetLines("value") // case-insensitive
	assert.Equal(t, FieldValues{"1.0.0"}, lines)
	lines = record.GetLines("NonExistent")
	assert.Empty(t, lines)

	// Test Fields method
	fields := record.Fields()
	assert.Equal(t, []string{"Name", "Value"}, fields)
}
