package debian

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
	"testing"
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
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		record = r
		found = true
		break
	}
	
	if !found {
		t.Fatal("No records found")
	}

	// Test field access
	if record.Get("Package") != "test-package" {
		t.Errorf("Package: got %q, want %q", record.Get("Package"), "test-package")
	}
	
	if record.Get("Version") != "1.0.0" {
		t.Errorf("Version: got %q, want %q", record.Get("Version"), "1.0.0")
	}
	
	expectedDesc := "A test package\nThis is a multi-line description\nwith additional details."
	if record.Get("Description") != expectedDesc {
		t.Errorf("Description: got %q, want %q", record.Get("Description"), expectedDesc)
	}
	
	// Test case-insensitive access
	if record.Get("package") != "test-package" {
		t.Errorf("Case-insensitive access failed")
	}
	
	// Test field ordering preservation
	fields := record.Fields()
	expectedOrder := []string{"Package", "Version", "Architecture", "Description"}
	if len(fields) != len(expectedOrder) {
		t.Fatalf("Field count: got %d, want %d", len(fields), len(expectedOrder))
	}
	
	for i, expected := range expectedOrder {
		if fields[i] != expected {
			t.Errorf("Field order[%d]: got %q, want %q", i, fields[i], expected)
		}
	}
}

func TestMultipleRecords(t *testing.T) {
	input := `Package: package1
Version: 1.0.0

Package: package2
Version: 2.0.0`

	parser := NewParser()
	var records []Record
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		records = append(records, record)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	if records[0].Get("Package") != "package1" {
		t.Errorf("First record: got %q, want %q", records[0].Get("Package"), "package1")
	}

	if records[1].Get("Package") != "package2" {
		t.Errorf("Second record: got %q, want %q", records[1].Get("Package"), "package2")
	}
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
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		originalRecord = r
		found = true
		break
	}
	
	if !found {
		t.Fatal("No records found")
	}
	
	// Convert to JSON
	jsonData, err := json.Marshal(originalRecord)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	
	// Convert back from JSON
	var roundTripRecord Record
	if err := json.Unmarshal(jsonData, &roundTripRecord); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	
	// Verify data integrity
	if len(originalRecord) != len(roundTripRecord) {
		t.Errorf("Field count mismatch: original=%d, roundtrip=%d", len(originalRecord), len(roundTripRecord))
	}
	
	for i, originalField := range originalRecord {
		if i >= len(roundTripRecord) {
			t.Errorf("Missing field after round-trip: %s", originalField.Name)
			continue
		}
		
		roundTripField := roundTripRecord[i]
		if originalField.Name != roundTripField.Name {
			t.Errorf("Field name mismatch at index %d: original=%q, roundtrip=%q", i, originalField.Name, roundTripField.Name)
		}
		
		if originalField.Value != roundTripField.Value {
			t.Errorf("Field value mismatch for %s: original=%q, roundtrip=%q", originalField.Name, originalField.Value, roundTripField.Value)
		}
	}
	
	t.Logf("JSON output: %s", string(jsonData))
}

func TestControlFormatRoundTrip(t *testing.T) {
	// Test case with exact formatting for byte-for-byte comparison
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
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		record = r
		found = true
		break
	}
	
	if !found {
		t.Fatal("No records found")
	}
	
	// Convert back to control format
	output := record.String()
	
	// Verify byte-for-byte identical
	if output != input {
		t.Errorf("Round-trip conversion not identical")
		t.Logf("Original:\n%q", input)
		t.Logf("Output:\n%q", output)
		
		// Show character-by-character diff for debugging
		minLen := len(input)
		if len(output) < minLen {
			minLen = len(output)
		}
		
		for i := 0; i < minLen; i++ {
			if input[i] != output[i] {
				t.Logf("First difference at position %d: input=%q output=%q", i, input[i], output[i])
				break
			}
		}
		
		if len(input) != len(output) {
			t.Logf("Length difference: input=%d output=%d", len(input), len(output))
		}
	} else {
		t.Log("Perfect byte-for-byte round-trip achieved!")
	}
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
				if err == nil {
					t.Errorf("Expected error for invalid field name: %s", tt.input)
				}
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
		if err == nil {
			t.Error("Expected error for duplicate field")
		}
		if !strings.Contains(err.Error(), "duplicate field") {
			t.Errorf("Error should mention duplicate field, got: %v", err)
		}
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
		if err != nil {
			t.Fatalf("Iterator failed: %v", err)
		}
		count++
		if count == 2 {
			break
		}
		if !record.Has("Package") {
			t.Errorf("Record %d missing Package field", count)
		}
	}
	
	if count != 2 {
		t.Errorf("Expected to process 2 records, got %d", count)
	}
}

func TestRepositoryFixtures(t *testing.T) {
	fixtures := []struct {
		name           string
		releaseFile    string
		packagesFile   string
		expectedOrigin string
		expectedPackageCount int
	}{
		{
			name:           "Spotify",
			releaseFile:    "spotify-release.gz",
			packagesFile:   "spotify-packages.gz",
			expectedOrigin: "Spotify LTD",
			expectedPackageCount: 4,
		},
	}

	parser := NewParser()

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			// Test Release file
			releaseFile, err := os.Open("testdata/" + fixture.releaseFile)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", fixture.releaseFile, err)
			}
			defer releaseFile.Close()

			gz, err := gzip.NewReader(releaseFile)
			if err != nil {
				t.Fatalf("Failed to create gzip reader for %s: %v", fixture.releaseFile, err)
			}
			defer gz.Close()

			var release Record
			found := false
			for r, err := range parser.ParseRecords(gz) {
				if err != nil {
					t.Fatalf("Failed to parse %s: %v", fixture.releaseFile, err)
				}
				release = r
				found = true
				break
			}
			
			if !found {
				t.Fatalf("No release record found in %s", fixture.releaseFile)
			}

			// Check Origin field
			if got := release.Get("Origin"); got != fixture.expectedOrigin {
				t.Errorf("%s Origin: got %q, want %q", fixture.name, got, fixture.expectedOrigin)
			}

			// Test Packages file
			packagesFile, err := os.Open("testdata/" + fixture.packagesFile)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", fixture.packagesFile, err)
			}
			defer packagesFile.Close()

			pgz, err := gzip.NewReader(packagesFile)
			if err != nil {
				t.Fatalf("Failed to create gzip reader for %s: %v", fixture.packagesFile, err)
			}
			defer pgz.Close()

			var packages []Record
			for record, err := range parser.ParseRecords(pgz) {
				if err != nil {
					t.Fatalf("Failed to parse %s: %v", fixture.packagesFile, err)
				}
				packages = append(packages, record)
			}

			// Verify expected package count
			if len(packages) != fixture.expectedPackageCount {
				t.Errorf("%s: expected %d packages, got %d", fixture.name, fixture.expectedPackageCount, len(packages))
			}

			// Verify all packages have required fields
			for i, pkg := range packages {
				if !pkg.Has("Package") {
					t.Errorf("%s package %d: missing Package field", fixture.name, i)
				}
				if !pkg.Has("Version") {
					t.Errorf("%s package %d: missing Version field", fixture.name, i)
				}
			}

			t.Logf("%s: parsed %d packages successfully", fixture.name, len(packages))
		})
	}
}