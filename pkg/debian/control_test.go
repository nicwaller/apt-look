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
	record, err := parser.ParseRecord(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRecord failed: %v", err)
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
	records, err := parser.ParseRecords(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRecords failed: %v", err)
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
	record, err := parser.ParseRecord(gz)
	if err != nil {
		t.Fatalf("ParseRecord failed: %v", err)
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
	records, err := parser.ParseRecords(gz)
	if err != nil {
		t.Fatalf("ParseRecords failed: %v", err)
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
		name         string
		releaseFile  string
		packagesFile string
		expectedOrigin string
	}{
		{
			name:         "PostgreSQL",
			releaseFile:  "postgresql-release.gz",
			packagesFile: "postgresql-packages.gz",
			expectedOrigin: "apt.postgresql.org",
		},
		{
			name:         "HashiCorp",
			releaseFile:  "hashicorp-release.gz",
			packagesFile: "hashicorp-packages.gz",
			expectedOrigin: "Artifactory",
		},
		{
			name:         "Docker",
			releaseFile:  "docker-release.gz",
			packagesFile: "docker-packages.gz",
			expectedOrigin: "Docker",
		},
		{
			name:         "Microsoft",
			releaseFile:  "microsoft-release.gz",
			packagesFile: "microsoft-packages.gz",
			expectedOrigin: "microsoft-ubuntu-jammy-prod jammy",
		},
		{
			name:         "Kubernetes",
			releaseFile:  "kubernetes-release.gz",
			packagesFile: "kubernetes-packages.gz",
			expectedOrigin: "obs://build.opensuse.org/isv:kubernetes:core:stable:v1.28/deb",
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

			release, err := parser.ParseRecord(gz)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", fixture.releaseFile, err)
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

			packages, err := parser.ParseRecords(pgz)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", fixture.packagesFile, err)
			}

			if len(packages) == 0 {
				t.Errorf("%s: Expected at least one package", fixture.name)
				return
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
	_, err := parser.ParseRecord(strings.NewReader(input))
	if err == nil {
		t.Error("Expected error for invalid field line")
	}
}

func TestContinuationWithoutField(t *testing.T) {
	input := ` This is a continuation line without a field`

	parser := NewParser()
	_, err := parser.ParseRecord(strings.NewReader(input))
	if err == nil {
		t.Error("Expected error for continuation line without field")
	}
}