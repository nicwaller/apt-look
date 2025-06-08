package rfc822

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"path/filepath"
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

	expectedComment := "A test field\nThis is a multi-line comment\nwith additional details."
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

func TestJSONRoundTrip(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0
Type: example
Size: 1024
Comment: A test field
 Multi-line comment`

	parser := NewParser()

	var originalRecord Record
	found := false
	for r, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		originalRecord = r
		found = true
		break
	}
	require.True(t, found)

	// Convert to JSON and back
	jsonData, err := json.Marshal(originalRecord)
	require.NoError(t, err)

	var roundTripRecord Record
	require.NoError(t, json.Unmarshal(jsonData, &roundTripRecord))

	// Verify perfect data integrity
	assert.Len(t, roundTripRecord, len(originalRecord))

	for i, originalField := range originalRecord {
		require.Less(t, i, len(roundTripRecord), "Missing field after round-trip: %s", originalField.Name)

		roundTripField := roundTripRecord[i]
		assert.Equal(t, originalField.Name, roundTripField.Name)
		assert.Equal(t, originalField.Value, roundTripField.Value)
	}

	t.Logf("JSON output: %s", string(jsonData))
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

func TestAccessorMethods(t *testing.T) {
	input := `Name: test-item
Value: 1.0.0`

	parser := NewParser()

	var record Record
	for r, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		record = r
		break
	}

	// Test Lookup method
	value, exists := record.Lookup("Name")
	assert.True(t, exists)
	assert.Equal(t, []string{"test-item"}, value)

	value, exists = record.Lookup("NonExistent")
	assert.False(t, exists)
	assert.Empty(t, value)

	// Test case-insensitive lookup
	value, exists = record.Lookup("name")
	assert.True(t, exists)
	assert.Equal(t, []string{"test-item"}, value)

	// Test Has method
	assert.True(t, record.Has("Name"))
	assert.True(t, record.Has("value")) // case-insensitive
	assert.False(t, record.Has("NonExistent"))

	// Test Get method
	assert.Equal(t, "test-item", record.Get("Name"))
	assert.Equal(t, "1.0.0", record.Get("value")) // case-insensitive
	assert.Empty(t, record.Get("NonExistent"))

	// Test Fields method
	fields := record.Fields()
	assert.Equal(t, []string{"Name", "Value"}, fields)
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

func TestRealWorldFixtures(t *testing.T) {
	fixtures := []struct {
		name                string
		releaseFile         string
		packagesFile        string
		expectedRecordCount int
	}{
		{
			name:                "Spotify",
			releaseFile:         "spotify-release.gz",
			packagesFile:        "spotify-packages.gz",
			expectedRecordCount: 4,
		},
	}

	parser := NewParser()

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			// Test Release file - just verify it parses without errors
			releaseFile, err := os.Open("testdata/" + fixture.releaseFile)
			require.NoError(t, err)
			defer releaseFile.Close()

			gz, err := gzip.NewReader(releaseFile)
			require.NoError(t, err)
			defer gz.Close()

			var release Record
			found := false
			for r, err := range parser.ParseRecords(gz) {
				require.NoError(t, err)
				release = r
				found = true
				break
			}
			require.True(t, found)
			require.NotEmpty(t, release, "Release record should not be empty")

			// Test Packages file
			packagesFile, err := os.Open("testdata/" + fixture.packagesFile)
			require.NoError(t, err)
			defer packagesFile.Close()

			pgz, err := gzip.NewReader(packagesFile)
			require.NoError(t, err)
			defer pgz.Close()

			var records []Record
			for record, err := range parser.ParseRecords(pgz) {
				require.NoError(t, err)
				records = append(records, record)
			}

			// Verify expected record count
			assert.Len(t, records, fixture.expectedRecordCount)

			// Verify all records have at least one field
			for i, record := range records {
				assert.True(t, len(record) > 0, "%s record %d: empty record", fixture.name, i)
			}

			t.Logf("%s: parsed %d records successfully", fixture.name, len(records))
		})
	}
}

func TestAllTestdataFiles(t *testing.T) {
	// Get all .gz files in testdata directory
	testdataDir := "testdata"
	files, err := filepath.Glob(filepath.Join(testdataDir, "*.gz"))
	require.NoError(t, err, "Failed to read testdata directory")
	require.NotEmpty(t, files, "No test files found in testdata directory")

	parser := NewParser()

	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		t.Run(fileName, func(t *testing.T) {
			file, err := os.Open(filePath)
			require.NoError(t, err, "Failed to open %s", fileName)
			defer file.Close()

			gz, err := gzip.NewReader(file)
			require.NoError(t, err, "Failed to create gzip reader for %s", fileName)
			defer gz.Close()

			var recordCount int

			for record, err := range parser.ParseRecords(gz) {
				require.NoError(t, err, "Failed to parse record %d in %s", recordCount+1, fileName)
				require.NotEmpty(t, record, "Empty record %d in %s", recordCount+1, fileName)

				recordCount++

				// Verify each record has at least one field
				assert.True(t, len(record) > 0, "Record %d in %s has no fields", recordCount, fileName)

				// Verify all field names are non-empty
				for _, field := range record {
					assert.NotEmpty(t, field.Name, "Empty field name in record %d of %s", recordCount, fileName)
					// Verify field values are properly structured
					assert.NotNil(t, field.Value, "Field %s in record %d has nil value", field.Name, recordCount)
				}
			}

			require.Greater(t, recordCount, 0, "No records found in %s", fileName)
			t.Logf("%s: parsed %d RFC822-style record(s) successfully", fileName, recordCount)
		})
	}

	t.Logf("Successfully tested %d testdata files", len(files))
}
