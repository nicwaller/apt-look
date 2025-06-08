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

	header, err := parser.ParseHeader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotEmpty(t, header, "No header found")

	// Test Lookup method
	field, exists := header.Lookup("Name")
	assert.True(t, exists)
	assert.Equal(t, "Name", field.Name)
	assert.Equal(t, FieldValues{"test-item"}, field.Value)

	field, exists = header.Lookup("NonExistent")
	assert.False(t, exists)
	assert.Empty(t, field.Name)
	assert.Empty(t, field.Value)

	// Test case-insensitive lookup
	field, exists = header.Lookup("name")
	assert.True(t, exists)
	assert.Equal(t, "Name", field.Name)
	assert.Equal(t, FieldValues{"test-item"}, field.Value)

	// Test Has method
	assert.True(t, header.Has("Name"))
	assert.True(t, header.Has("value")) // case-insensitive
	assert.False(t, header.Has("NonExistent"))

	// Test Get method
	assert.Equal(t, "test-item", header.Get("Name"))
	assert.Equal(t, "1.0.0", header.Get("value")) // case-insensitive
	assert.Empty(t, header.Get("NonExistent"))

	// Test GetLines method
	lines := header.GetLines("Name")
	assert.Equal(t, FieldValues{"test-item"}, lines)
	lines = header.GetLines("value") // case-insensitive
	assert.Equal(t, FieldValues{"1.0.0"}, lines)
	lines = header.GetLines("NonExistent")
	assert.Empty(t, lines)

	// Test Fields method
	fields := header.Fields()
	assert.Equal(t, []string{"Name", "Value"}, fields)
}
