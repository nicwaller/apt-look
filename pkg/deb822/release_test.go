package deb822

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReleaseBasic(t *testing.T) {
	// Test with Spotify release file
	releaseFile, err := os.Open("testdata/spotify-release.gz")
	require.NoError(t, err)
	defer releaseFile.Close()

	gz, err := gzip.NewReader(releaseFile)
	require.NoError(t, err)
	defer gz.Close()

	release, err := ParseRelease(gz)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Verify mandatory fields
	assert.Equal(t, "stable", release.Suite)
	assert.Equal(t, "stable", release.Codename)
	assert.Equal(t, []string{"amd64", "i386"}, release.Architectures)
	assert.Equal(t, []string{"non-free"}, release.Components)

	// Verify date parsing
	expectedDate, _ := time.Parse(time.RFC1123, "Mon, 19 May 2025 10:00:02 UTC")
	assert.Equal(t, expectedDate, release.Date)

	// Verify optional fields
	assert.Equal(t, "Spotify LTD", release.Origin)
	assert.Equal(t, "Spotify Public Repository", release.Label)
	assert.Equal(t, "0.4", release.Version)

	// Verify hash entries
	require.NotEmpty(t, release.SHA256)
	assert.Len(t, release.SHA256, 9) // 9 files in the Spotify release

	// Check a specific hash entry
	found := false
	for _, entry := range release.SHA256 {
		if entry.Path == "non-free/binary-amd64/Packages" {
			assert.Equal(t, "c802b81dd9a61e383e63123d10be1fd4bfeb468f686102bec729cc38b0b0f75a", entry.Hash)
			assert.Equal(t, int64(4188), entry.Size)
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find non-free/binary-amd64/Packages entry")

	// Verify legacy hashes are also present
	assert.Len(t, release.MD5Sum, 9)
	assert.Len(t, release.SHA1, 9)
}

func TestParseReleaseDocker(t *testing.T) {
	// Test with Docker release file (more complex)
	releaseFile, err := os.Open("testdata/docker-release.gz")
	require.NoError(t, err)
	defer releaseFile.Close()

	gz, err := gzip.NewReader(releaseFile)
	require.NoError(t, err)
	defer gz.Close()

	release, err := ParseRelease(gz)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Verify mandatory fields
	assert.Equal(t, "jammy", release.Suite)
	assert.Equal(t, []string{"amd64", "arm64", "armhf", "s390x", "ppc64el"}, release.Architectures)
	assert.Equal(t, []string{"stable", "edge", "test", "nightly"}, release.Components)

	// Verify optional fields
	assert.Equal(t, "Docker", release.Origin)
	assert.Equal(t, "Docker CE", release.Label)

	// Verify date parsing
	expectedDate, _ := time.Parse(time.RFC1123, "Tue, 03 Jun 2025 13:05:08 +0000")
	assert.Equal(t, expectedDate, release.Date)

	// Docker release has many more files
	assert.Greater(t, len(release.SHA256), 100)
	assert.Greater(t, len(release.MD5Sum), 100)
	assert.Greater(t, len(release.SHA1), 100)
}

func TestHashEntryParsing(t *testing.T) {
	testCases := []struct {
		name     string
		lines    []string
		expected []HashEntry
		hasError bool
	}{
		{
			name: "valid single entry",
			lines: []string{
				"4c195df3750b6fdb056bd98d18542d25 4188 non-free/binary-amd64/Packages",
			},
			expected: []HashEntry{
				{Hash: "4c195df3750b6fdb056bd98d18542d25", Size: 4188, Path: "non-free/binary-amd64/Packages"},
			},
		},
		{
			name: "multiple entries with empty lines",
			lines: []string{
				"4c195df3750b6fdb056bd98d18542d25 4188 non-free/binary-amd64/Packages",
				"",
				"8d5b8cd9009767eb059099f617762d8e 1645 non-free/binary-amd64/Packages.gz",
			},
			expected: []HashEntry{
				{Hash: "4c195df3750b6fdb056bd98d18542d25", Size: 4188, Path: "non-free/binary-amd64/Packages"},
				{Hash: "8d5b8cd9009767eb059099f617762d8e", Size: 1645, Path: "non-free/binary-amd64/Packages.gz"},
			},
		},
		{
			name: "invalid format - missing fields",
			lines: []string{
				"4c195df3750b6fdb056bd98d18542d25 4188",
			},
			hasError: true,
		},
		{
			name: "invalid format - non-numeric size",
			lines: []string{
				"4c195df3750b6fdb056bd98d18542d25 invalid non-free/binary-amd64/Packages",
			},
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseHashEntries(tc.lines)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestParseBoolField(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"yes", true},
		{"YES", true},
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"no", false},
		{"false", false},
		{"0", false},
		{"", false},
		{"invalid", false},
		{" yes ", true}, // Test trimming
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := parseBoolField(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReleaseFieldAccess(t *testing.T) {
	// Test field access methods
	releaseFile, err := os.Open("testdata/spotify-release.gz")
	require.NoError(t, err)
	defer releaseFile.Close()

	gz, err := gzip.NewReader(releaseFile)
	require.NoError(t, err)
	defer gz.Close()

	release, err := ParseRelease(gz)
	require.NoError(t, err)

	// Test GetField
	assert.Equal(t, "Spotify LTD", release.GetField("Origin"))
	assert.Equal(t, "", release.GetField("NonExistentField"))

	// Test HasField
	assert.True(t, release.HasField("Origin"))
	assert.True(t, release.HasField("origin")) // case-insensitive
	assert.False(t, release.HasField("NonExistentField"))

	// Test Fields
	fields := release.Fields()
	assert.Contains(t, fields, "Origin")
	assert.Contains(t, fields, "Label")
	assert.Contains(t, fields, "Suite")
}

func TestReleaseJSONSerialization(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	releaseFile, err := os.Open("testdata/spotify-release.gz")
	require.NoError(t, err)
	defer releaseFile.Close()

	gz, err := gzip.NewReader(releaseFile)
	require.NoError(t, err)
	defer gz.Close()

	original, err := ParseRelease(gz)
	require.NoError(t, err)

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	assert.Equal(t, "stable", jsonMap["suite"])
	assert.Equal(t, "stable", jsonMap["codename"])
	assert.Equal(t, "Spotify LTD", jsonMap["origin"])
	assert.Equal(t, "Spotify Public Repository", jsonMap["label"])
	assert.Contains(t, jsonMap, "architectures")
	assert.Contains(t, jsonMap, "date")
	assert.Contains(t, jsonMap, "sha256")

	// Verify the record field is excluded
	assert.NotContains(t, jsonMap, "record")

	// Unmarshal back to struct
	var unmarshaled Release
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify key fields match (note: record field won't be preserved)
	assert.Equal(t, original.Suite, unmarshaled.Suite)
	assert.Equal(t, original.Codename, unmarshaled.Codename)
	assert.Equal(t, original.Origin, unmarshaled.Origin)
	assert.Equal(t, original.Label, unmarshaled.Label)
	assert.Equal(t, original.Architectures, unmarshaled.Architectures)
	assert.Equal(t, original.Components, unmarshaled.Components)
	assert.Equal(t, original.Date.Unix(), unmarshaled.Date.Unix()) // Compare Unix timestamps
	assert.Equal(t, len(original.SHA256), len(unmarshaled.SHA256))

	t.Logf("JSON output: %s", string(jsonData))
}

func TestAllReleaseFiles(t *testing.T) {
	// Test that all release files in testdata can be parsed
	testdataDir := "testdata"
	files, err := filepath.Glob(filepath.Join(testdataDir, "*-release.gz"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		t.Run(fileName, func(t *testing.T) {
			file, err := os.Open(filePath)
			require.NoError(t, err, "Failed to open %s", fileName)
			defer file.Close()

			gz, err := gzip.NewReader(file)
			require.NoError(t, err, "Failed to create gzip reader for %s", fileName)
			defer gz.Close()

			release, err := ParseRelease(gz)
			require.NoError(t, err, "Failed to parse %s", fileName)
			require.NotNil(t, release, "Release should not be nil for %s", fileName)

			// Basic validation - all releases should have these
			assert.NotEmpty(t, release.Architectures, "%s should have architectures", fileName)
			assert.False(t, release.Date.IsZero(), "%s should have a valid date", fileName)

			// Should have at least one of Suite or Codename
			assert.True(t, release.Suite != "" || release.Codename != "",
				"%s should have Suite or Codename", fileName)

			t.Logf("%s: parsed successfully (%d SHA256 entries)",
				fileName, len(release.SHA256))
		})
	}
}
