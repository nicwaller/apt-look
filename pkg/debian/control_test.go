package debian

import (
	"compress/gzip"
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
		break // Get first record
	}
	
	if !found {
		t.Fatal("No records found")
	}

	expected := map[string]string{
		"Package":      "test-package",
		"Version":      "1.0.0",
		"Architecture": "amd64",
		"Description":  "A test package\nThis is a multi-line description\nwith additional details.",
	}

	for field, expectedValue := range expected {
		if !record.Has(field) {
			t.Errorf("Missing field: %s", field)
			continue
		}
		if got := record.Get(field); got != expectedValue {
			t.Errorf("Field %s: got %q, want %q", field, got, expectedValue)
		}
	}
}

func TestParseMultipleRecords(t *testing.T) {
	input := `Package: package1
Version: 1.0.0

Package: package2
Version: 2.0.0
Architecture: amd64`

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
		t.Errorf("First record package: got %q, want %q", records[0].Get("Package"), "package1")
	}

	if records[1].Get("Package") != "package2" {
		t.Errorf("Second record package: got %q, want %q", records[1].Get("Package"), "package2")
	}
}

func TestParseSpotifyRelease(t *testing.T) {
	file, err := os.Open("testdata/spotify-release.gz")
	if err != nil {
		t.Fatalf("Failed to open test fixture: %v", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	parser := NewParser()
	
	var record Record
	found := false
	for r, err := range parser.ParseRecords(gz) {
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		record = r
		found = true
		break // Get first record
	}
	
	if !found {
		t.Fatal("No records found")
	}

	// Check that we have the expected fields
	expectedFields := []string{"Origin", "Label", "Suite", "Codename", "Architectures", "Components"}
	for _, field := range expectedFields {
		if !record.Has(field) {
			t.Errorf("Missing expected field: %s", field)
		}
	}

	// Check specific values
	if got := record.Get("Origin"); !strings.Contains(got, "Spotify") {
		t.Errorf("Origin field should contain 'Spotify', got: %q", got)
	}

	if got := record.Get("Suite"); got != "stable" {
		t.Errorf("Suite: got %q, want %q", got, "stable")
	}
}

func TestParseSpotifyPackages(t *testing.T) {
	file, err := os.Open("testdata/spotify-packages.gz")
	if err != nil {
		t.Fatalf("Failed to open test fixture: %v", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	parser := NewParser()
	var records []Record
	for record, err := range parser.ParseRecords(gz) {
		if err != nil {
			t.Fatalf("ParseRecords failed: %v", err)
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		t.Fatal("Expected at least one package record")
	}

	// Check the first package
	pkg := records[0]
	expectedFields := []string{"Package", "Version", "Architecture", "Filename", "Size", "Description"}
	for _, field := range expectedFields {
		if !pkg.Has(field) {
			t.Errorf("Missing expected field in package: %s", field)
		}
	}

	// All packages should have a Package field
	for i, pkg := range records {
		if !pkg.Has("Package") {
			t.Errorf("Package %d missing Package field", i)
		}
	}
}

func TestMultipleRepositoryFixtures(t *testing.T) {
	fixtures := []struct {
		name           string
		releaseFile    string
		packagesFile   string
		expectedOrigin string
		expectedPackageCount int
	}{
		{
			name:           "Brave",
			releaseFile:    "brave-release.gz",
			packagesFile:   "brave-packages.gz",
			expectedOrigin: "Brave Software",
			expectedPackageCount: 124,
		},
		{
			name:           "Chrome",
			releaseFile:    "chrome-release.gz",
			packagesFile:   "chrome-packages.gz",
			expectedOrigin: "Google LLC",
			expectedPackageCount: 4,
		},
		{
			name:           "Docker",
			releaseFile:    "docker-release.gz",
			packagesFile:   "docker-packages.gz",
			expectedOrigin: "Docker",
			expectedPackageCount: 306,
		},
		{
			name:           "HashiCorp",
			releaseFile:    "hashicorp-release.gz",
			packagesFile:   "hashicorp-packages.gz",
			expectedOrigin: "Artifactory",
			expectedPackageCount: 2574,
		},
		{
			name:           "Kubernetes",
			releaseFile:    "kubernetes-release.gz",
			packagesFile:   "kubernetes-packages.gz",
			expectedOrigin: "obs://build.opensuse.org/isv:kubernetes:core:stable:v1.28/deb",
			expectedPackageCount: 199,
		},
		{
			name:           "Microsoft",
			releaseFile:    "microsoft-release.gz",
			packagesFile:   "microsoft-packages.gz",
			expectedOrigin: "microsoft-ubuntu-jammy-prod jammy",
			expectedPackageCount: 1744,
		},
		{
			name:           "NodeSource",
			releaseFile:    "nodesource-release.gz",
			packagesFile:   "nodesource-packages.gz",
			expectedOrigin: "",
			expectedPackageCount: 1,
		},
		{
			name:           "PostgreSQL",
			releaseFile:    "postgresql-release.gz",
			packagesFile:   "postgresql-packages.gz",
			expectedOrigin: "apt.postgresql.org",
			expectedPackageCount: 2201,
		},
		{
			name:           "Signal",
			releaseFile:    "signal-release.gz",
			packagesFile:   "signal-packages.gz",
			expectedOrigin: ". xenial",
			expectedPackageCount: 467,
		},
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
				break // Get first record
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

func TestInvalidFieldLine(t *testing.T) {
	input := `This is not a valid field line`

	parser := NewParser()
	for _, err := range parser.ParseRecords(strings.NewReader(input)) {
		if err == nil {
			t.Error("Expected error for invalid field line")
		}
		break // Just check first result
	}
}

func TestContinuationWithoutField(t *testing.T) {
	input := ` This is a continuation line without a field`

	parser := NewParser()
	for _, err := range parser.ParseRecords(strings.NewReader(input)) {
		if err == nil {
			t.Error("Expected error for continuation line without field")
		}
		break // Just check first result
	}
}

func TestInvalidFieldNames(t *testing.T) {
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
				break // Just check first result
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
		break // Just check first result
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
	
	// Test that we can iterate and stop early
	count := 0
	for record, err := range parser.ParseRecords(strings.NewReader(input)) {
		if err != nil {
			t.Fatalf("Iterator failed: %v", err)
		}
		count++
		if count == 2 {
			break // Stop after 2 records
		}
		if !record.Has("Package") {
			t.Errorf("Record %d missing Package field", count)
		}
	}
	
	if count != 2 {
		t.Errorf("Expected to process 2 records, got %d", count)
	}
}