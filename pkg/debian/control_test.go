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