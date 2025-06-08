package deb822

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []map[string]string
	}{
		{
			name: "single record",
			input: `Package: test-package
Version: 1.0.0
Architecture: amd64`,
			expected: []map[string]string{
				{
					"Package":      "test-package",
					"Version":      "1.0.0",
					"Architecture": "amd64",
				},
			},
		},
		{
			name: "multiple records",
			input: `Package: first-package
Version: 1.0.0

Package: second-package
Version: 2.0.0
Architecture: arm64`,
			expected: []map[string]string{
				{
					"Package": "first-package",
					"Version": "1.0.0",
				},
				{
					"Package":      "second-package",
					"Version":      "2.0.0",
					"Architecture": "arm64",
				},
			},
		},
		{
			name: "records with multi-line fields",
			input: `Package: complex-package
Description: A test package
 with a multi-line description
 that spans several lines

Package: simple-package
Description: Simple description`,
			expected: []map[string]string{
				{
					"Package":     "complex-package",
					"Description": "A test package with a multi-line description that spans several lines",
				},
				{
					"Package":     "simple-package",
					"Description": "Simple description",
				},
			},
		},
		{
			name: "records with comments",
			input: `# This is a comment
Package: test-package
Version: 1.0.0
# Another comment

# Comment in second record
Package: another-package
Version: 2.0.0`,
			expected: []map[string]string{
				{
					"Package": "test-package",
					"Version": "1.0.0",
				},
				{
					"Package": "another-package",
					"Version": "2.0.0",
				},
			},
		},
		{
			name: "empty input",
			input: "",
			expected: []map[string]string{},
		},
		{
			name: "only whitespace and comments",
			input: `
# Just a comment

   
# Another comment
   `,
			expected: []map[string]string{},
		},
		{
			name: "trailing blank lines",
			input: `Package: test-package
Version: 1.0.0


`,
			expected: []map[string]string{
				{
					"Package": "test-package",
					"Version": "1.0.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var headers []map[string]string

			for header, err := range ParseRecords(strings.NewReader(tt.input)) {
				require.NoError(t, err, "Unexpected error parsing records")

				// Convert header to map for easier comparison
				headerMap := make(map[string]string)
				for _, field := range header.Fields() {
					headerMap[field] = header.Get(field)
				}
				headers = append(headers, headerMap)
			}

			// Handle nil vs empty slice difference
			if headers == nil && len(tt.expected) == 0 {
				headers = []map[string]string{}
			}

			assert.Equal(t, tt.expected, headers)
		})
	}
}

func TestParseRecordsWithErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "invalid field format",
			input: `Package: test-package
Invalid line without colon
Version: 1.0.0`,
		},
		{
			name: "continuation line without field",
			input: ` This is a continuation line without a preceding field
Package: test-package`,
		},
		{
			name: "invalid field name",
			input: `Package: test-package
-Invalid: field name starting with dash`,
		},
		{
			name: "duplicate field in same record",
			input: `Package: test-package
Version: 1.0.0
Package: duplicate-package`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errorOccurred bool

			for header, err := range ParseRecords(strings.NewReader(tt.input)) {
				if err != nil {
					errorOccurred = true
					assert.Error(t, err, "Expected error for invalid input")
					assert.Nil(t, header, "Header should be nil when error occurs")
					break
				}
				// If no error occurred, we didn't get the expected error
				// Continue processing to see if error occurs later
			}

			assert.True(t, errorOccurred, "Expected an error to occur during parsing")
		})
	}
}

func TestParseRecordsIteratorBehavior(t *testing.T) {
	input := `Package: first
Version: 1.0.0

Package: second
Version: 2.0.0

Package: third
Version: 3.0.0`

	t.Run("early termination", func(t *testing.T) {
		var count int
		for _, err := range ParseRecords(strings.NewReader(input)) {
			require.NoError(t, err)
			count++
			if count == 2 {
				// Stop iteration early
				break
			}
		}
		assert.Equal(t, 2, count, "Should have processed exactly 2 records before breaking")
	})

	t.Run("full iteration", func(t *testing.T) {
		var packages []string
		for header, err := range ParseRecords(strings.NewReader(input)) {
			require.NoError(t, err)
			packages = append(packages, header.Get("Package"))
		}
		expected := []string{"first", "second", "third"}
		assert.Equal(t, expected, packages)
	})
}

func TestParseRecordsEmptyRecords(t *testing.T) {
	// Test that empty records (just blank lines) are ignored
	input := `Package: test-package
Version: 1.0.0



Package: another-package
Version: 2.0.0`

	var packages []string
	for header, err := range ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		packages = append(packages, header.Get("Package"))
	}

	expected := []string{"test-package", "another-package"}
	assert.Equal(t, expected, packages)
}