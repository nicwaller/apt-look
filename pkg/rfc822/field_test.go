package rfc822

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFieldUnfold(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected string
	}{
		{
			name:     "empty field",
			field:    Field{Name: "Name", Value: []string{}},
			expected: "",
		},
		{
			name:     "single line field",
			field:    Field{Name: "Name", Value: []string{"test-item"}},
			expected: "test-item",
		},
		{
			name:     "two line field",
			field:    Field{Name: "Comment", Value: []string{"Short comment", "Longer detailed comment"}},
			expected: "Short comment Longer detailed comment",
		},
		{
			name:     "three line field",
			field:    Field{Name: "Comment", Value: []string{"Short comment", "Longer detailed comment", "Even more details"}},
			expected: "Short comment Longer detailed comment Even more details",
		},
		{
			name:     "field with empty lines",
			field:    Field{Name: "Comment", Value: []string{"First line", "", "Third line"}},
			expected: "First line  Third line",
		},
		{
			name:     "field with spaces in continuation",
			field:    Field{Name: "Comment", Value: []string{"First line", " Already indented", "  Double indented"}},
			expected: "First line  Already indented   Double indented",
		},
		{
			name:     "Hash field (typical multi-entry field)",
			field:    Field{Name: "Hash", Value: []string{"a1b2c3d4e5f6 12345 path/to/file1.ext", "f6e5d4c3b2a1 67890 path/to/file2.ext"}},
			expected: "a1b2c3d4e5f6 12345 path/to/file1.ext f6e5d4c3b2a1 67890 path/to/file2.ext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.field.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
