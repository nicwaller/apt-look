package debian

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSimpleRecord(t *testing.T) {
	input := `Package: test-package
Version: 1.0.0
Architecture: amd64
Description: A test package
 This is a multi-line description
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
	assert.Equal(t, "test-package", record.Get("Package"))
	assert.Equal(t, "1.0.0", record.Get("Version"))
	assert.Equal(t, "amd64", record.Get("Architecture"))
	
	expectedDesc := "A test package\nThis is a multi-line description\nwith additional details."
	assert.Equal(t, expectedDesc, record.Get("Description"))
	
	// Test case-insensitive access
	assert.Equal(t, "test-package", record.Get("package"))
	
	// Test field ordering preservation
	fields := record.Fields()
	expectedOrder := []string{"Package", "Version", "Architecture", "Description"}
	assert.Equal(t, expectedOrder, fields)
}

func TestMultipleRecords(t *testing.T) {
	input := `Package: package1
Version: 1.0.0

Package: package2
Version: 2.0.0`

	parser := NewParser()
	var records []Record
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		records = append(records, record)
	}

	assert.Len(t, records, 2)
	assert.Equal(t, "package1", records[0].Get("Package"))
	assert.Equal(t, "package2", records[1].Get("Package"))
}

func TestJSONRoundTrip(t *testing.T) {
	input := `Package: test-package
Version: 1.0.0
Architecture: amd64
Installed-Size: 1024
Description: A test package
 Multi-line description`

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
	input := `Package: test-package
Version: 1.0.0
Architecture: amd64
Description: A test package
 This is a multi-line description
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
	output := record.String()
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
	input := `Package: test-package
Version: 1.0.0
Package: duplicate-package`

	parser := NewParser()
	for _, err := range parser.ParseRecords(strings.NewReader(input)) {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate field")
		break
	}
}

func TestIteratorInterface(t *testing.T) {
	input := `Package: package1
Version: 1.0.0

Package: package2
Version: 2.0.0

Package: package3
Version: 3.0.0`

	parser := NewParser()
	
	// Test early termination
	count := 0
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		count++
		if count == 2 {
			break
		}
		assert.True(t, record.Has("Package"))
	}
	
	assert.Equal(t, 2, count)
}

func TestAccessorMethods(t *testing.T) {
	input := `Package: test-package
Version: 1.0.0`

	parser := NewParser()
	
	var record Record
	for r, err := range parser.ParseRecords(strings.NewReader(input)) {
		require.NoError(t, err)
		record = r
		break
	}

	// Test Lookup method
	value, exists := record.Lookup("Package")
	assert.True(t, exists)
	assert.Equal(t, "test-package", value)
	
	value, exists = record.Lookup("NonExistent")
	assert.False(t, exists)
	assert.Empty(t, value)
	
	// Test case-insensitive lookup
	value, exists = record.Lookup("package")
	assert.True(t, exists)
	assert.Equal(t, "test-package", value)
	
	// Test Has method
	assert.True(t, record.Has("Package"))
	assert.True(t, record.Has("version")) // case-insensitive
	assert.False(t, record.Has("NonExistent"))
	
	// Test Get method
	assert.Equal(t, "test-package", record.Get("Package"))
	assert.Equal(t, "1.0.0", record.Get("version")) // case-insensitive
	assert.Empty(t, record.Get("NonExistent"))
	
	// Test Fields method
	fields := record.Fields()
	assert.Equal(t, []string{"Package", "Version"}, fields)
}

func TestRepositoryFixtures(t *testing.T) {
	fixtures := []struct {
		name                 string
		releaseFile          string
		packagesFile         string
		expectedOrigin       string
		expectedPackageCount int
	}{
		{
			name:                 "Spotify",
			releaseFile:          "spotify-release.gz",
			packagesFile:         "spotify-packages.gz",
			expectedOrigin:       "Spotify LTD",
			expectedPackageCount: 4,
		},
	}

	parser := NewParser()

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			// Test Release file
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

			// Check Origin field
			assert.Equal(t, fixture.expectedOrigin, release.Get("Origin"))

			// Test Packages file
			packagesFile, err := os.Open("testdata/" + fixture.packagesFile)
			require.NoError(t, err)
			defer packagesFile.Close()

			pgz, err := gzip.NewReader(packagesFile)
			require.NoError(t, err)
			defer pgz.Close()

			var packages []Record
			for record, err := range parser.ParseRecords(pgz) {
				require.NoError(t, err)
				packages = append(packages, record)
			}

			// Verify expected package count
			assert.Len(t, packages, fixture.expectedPackageCount)

			// Verify all packages have required fields
			for i, pkg := range packages {
				assert.True(t, pkg.Has("Package"), "%s package %d: missing Package field", fixture.name, i)
				assert.True(t, pkg.Has("Version"), "%s package %d: missing Version field", fixture.name, i)
			}

			t.Logf("%s: parsed %d packages successfully", fixture.name, len(packages))
		})
	}
}