package apt

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicwaller/apt-look/pkg/apt/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMount_FileURL(t *testing.T) {
	// Get absolute path to our test repository
	testRepoPath, err := filepath.Abs("testdata/emptyrepo")
	require.NoError(t, err)

	// Create a source entry with file:// URL
	sourceLine := "deb file://" + testRepoPath + " stable main"
	entry, err := sources.ParseSourceLine(sourceLine, 1)
	require.NoError(t, err)

	// Test apt.Mount() - should succeed and validate repository exists
	repo, err := Mount(*entry)
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "file", repo.archiveRoot.Scheme)
	assert.Equal(t, testRepoPath, repo.archiveRoot.Path)
	assert.Contains(t, repo.components, "main")

	// Verify Release file was fetched during mount
	release := repo.Release()
	assert.NotNil(t, release)
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
	assert.Contains(t, release.Architectures, "amd64")
	assert.Contains(t, release.Components, "main")

	// Test that we can iterate over packages (should be empty)
	ctx := context.Background()
	var packageCount int
	for pkg, err := range repo.Packages(ctx) {
		require.NoError(t, err)
		packageCount++
		_ = pkg // Use the variable to avoid unused variable error
	}
	assert.Equal(t, 0, packageCount, "Empty repository should have no packages")
}

func TestMount_HTTPSURL(t *testing.T) {
	// Create a source entry with HTTPS URL pointing to our S3-hosted repository
	sourceLine := "deb https://nicwaller-apt.s3.ca-central-1.amazonaws.com stable main"
	entry, err := sources.ParseSourceLine(sourceLine, 1)
	require.NoError(t, err)

	// Test apt.Mount() - should succeed and validate repository exists
	repo, err := Mount(*entry)
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "https", repo.archiveRoot.Scheme)
	assert.Equal(t, "nicwaller-apt.s3.ca-central-1.amazonaws.com", repo.archiveRoot.Host)
	assert.Contains(t, repo.components, "main")

	// Verify Release file was fetched during mount
	release := repo.Release()
	assert.NotNil(t, release)
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
	assert.Contains(t, release.Architectures, "amd64")
	assert.Contains(t, release.Components, "main")

	// Test that we can iterate over packages (should be empty)
	ctx := context.Background()
	var packageCount int
	for pkg, err := range repo.Packages(ctx) {
		require.NoError(t, err)
		packageCount++
		_ = pkg // Use the variable to avoid unused variable error
	}
	assert.Equal(t, 0, packageCount, "Empty repository should have no packages")
}

func TestMountURL_FileURL(t *testing.T) {
	// Get absolute path to our test repository
	testRepoPath, err := filepath.Abs("testdata/emptyrepo")
	require.NoError(t, err)

	// Create URL
	repoURL, err := url.Parse("file://" + testRepoPath)
	require.NoError(t, err)

	// Test apt.MountURL() with default components - should succeed and validate repository
	repo, err := MountURL(repoURL, "stable")
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "file", repo.archiveRoot.Scheme)
	assert.Equal(t, testRepoPath, repo.archiveRoot.Path)
	assert.Contains(t, repo.components, "main") // should default to ["main"]
	assert.Len(t, repo.components, 1)

	// Verify Release file was fetched during mount
	release := repo.Release()
	assert.NotNil(t, release)
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
}

func TestMountURL_HTTPSURL(t *testing.T) {
	// Create URL for S3-hosted repository
	repoURL, err := url.Parse("https://nicwaller-apt.s3.ca-central-1.amazonaws.com")
	require.NoError(t, err)

	// Test apt.MountURL() with explicit components - should succeed and validate repository
	repo, err := MountURL(repoURL, "stable", WithComponents("main", "contrib"))
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "https", repo.archiveRoot.Scheme)
	assert.Equal(t, "nicwaller-apt.s3.ca-central-1.amazonaws.com", repo.archiveRoot.Host)
	assert.Contains(t, repo.components, "main")
	assert.Contains(t, repo.components, "contrib")
	assert.Len(t, repo.components, 2)

	// Verify Release file was fetched during mount
	release := repo.Release()
	assert.NotNil(t, release)
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
}

func TestDiscover_FileURL(t *testing.T) {
	// Get absolute path to our test repository
	testRepoPath, err := filepath.Abs("testdata/emptyrepo")
	require.NoError(t, err)

	archiveRoot := "file://" + testRepoPath

	// Test apt.Discover()
	entries, err := Discover(archiveRoot)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Should find our "stable" distribution
	var stableEntry *sources.Entry
	for i, entry := range entries {
		if entry.Distribution == "stable" {
			stableEntry = &entries[i]
			break
		}
	}
	require.NotNil(t, stableEntry, "Should discover 'stable' distribution")

	// Verify the discovered entry
	assert.Equal(t, sources.SourceTypeDeb, stableEntry.Type)
	assert.Equal(t, "file", stableEntry.ArchiveRoot.Scheme)
	assert.Equal(t, testRepoPath, stableEntry.ArchiveRoot.Path)
	assert.Contains(t, stableEntry.Components, "main")
}

func TestDiscover_HTTPSURL(t *testing.T) {
	archiveRoot := "https://nicwaller-apt.s3.ca-central-1.amazonaws.com"

	// Test apt.Discover()
	entries, err := Discover(archiveRoot)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Should find our "stable" distribution
	var stableEntry *sources.Entry
	for i, entry := range entries {
		if entry.Distribution == "stable" {
			stableEntry = &entries[i]
			break
		}
	}
	require.NotNil(t, stableEntry, "Should discover 'stable' distribution")

	// Verify the discovered entry
	assert.Equal(t, sources.SourceTypeDeb, stableEntry.Type)
	assert.Equal(t, "https", stableEntry.ArchiveRoot.Scheme)
	assert.Equal(t, "nicwaller-apt.s3.ca-central-1.amazonaws.com", stableEntry.ArchiveRoot.Host)
	assert.Contains(t, stableEntry.Components, "main")
}

func TestDiscover_InvalidURL(t *testing.T) {
	// Test with invalid URL
	entries, err := Discover("not-a-valid-url")
	assert.Error(t, err)
	assert.Nil(t, entries)
	// Could be either URL parsing error or transport error
	assert.True(t,
		strings.Contains(err.Error(), "invalid archive root URL") ||
			strings.Contains(err.Error(), "unsupported transport"),
		"Error should mention URL or transport issue, got: %s", err.Error())
}

func TestDiscover_NoValidDistributions(t *testing.T) {
	// Test with file URL that has no valid distributions (fast local failure)
	entries, err := Discover("file:///nonexistent/path")
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "no valid distributions found")
}

func TestDiscover_DistRootURL(t *testing.T) {
	// Get absolute path to our test repository
	testRepoPath, err := filepath.Abs("testdata/emptyrepo")
	require.NoError(t, err)

	// Test with a distribution root URL (includes /dists/)
	distRootURL := "file://" + testRepoPath + "/dists/stable"

	// Test apt.Discover()
	entries, err := Discover(distRootURL)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entry := entries[0]
	// Should correctly infer the archive root
	assert.Equal(t, sources.SourceTypeDeb, entry.Type)
	assert.Equal(t, "file", entry.ArchiveRoot.Scheme)
	assert.Equal(t, testRepoPath, entry.ArchiveRoot.Path)
	assert.Equal(t, "stable", entry.Distribution)
	assert.Contains(t, entry.Components, "main")
}

func TestDiscover_DistRootHTTPS(t *testing.T) {
	// Test with HTTPS distribution root URL
	distRootURL := "https://nicwaller-apt.s3.ca-central-1.amazonaws.com/dists/stable"

	// Test apt.Discover()
	entries, err := Discover(distRootURL)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entry := entries[0]
	// Should correctly infer the archive root
	assert.Equal(t, sources.SourceTypeDeb, entry.Type)
	assert.Equal(t, "https", entry.ArchiveRoot.Scheme)
	assert.Equal(t, "nicwaller-apt.s3.ca-central-1.amazonaws.com", entry.ArchiveRoot.Host)
	assert.Equal(t, "/", entry.ArchiveRoot.Path) // Root path
	assert.Equal(t, "stable", entry.Distribution)
	assert.Contains(t, entry.Components, "main")
}

func TestTryParseDistRoot(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectEntry     bool
		expectedArchive string
		expectedDist    string
	}{
		{
			name:            "Ubuntu distribution URL",
			input:           "https://archive.ubuntu.com/ubuntu/dists/jammy",
			expectEntry:     true,
			expectedArchive: "https://archive.ubuntu.com/ubuntu",
			expectedDist:    "jammy",
		},
		{
			name:            "Root level distribution",
			input:           "https://example.com/dists/stable",
			expectEntry:     true,
			expectedArchive: "https://example.com/",
			expectedDist:    "stable",
		},
		{
			name:            "Deep path distribution",
			input:           "https://repo.example.com/path/to/repo/dists/testing",
			expectEntry:     true,
			expectedArchive: "https://repo.example.com/path/to/repo",
			expectedDist:    "testing",
		},
		{
			name:        "Not a distribution URL",
			input:       "https://example.com/some/path",
			expectEntry: false,
		},
		{
			name:        "Empty distribution",
			input:       "https://example.com/dists/",
			expectEntry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputURL, err := url.Parse(tt.input)
			require.NoError(t, err)

			entry, archiveRoot := tryParseDistRoot(inputURL)

			if tt.expectEntry {
				require.NotNil(t, entry, "Expected to parse distribution URL")
				require.NotNil(t, archiveRoot, "Expected archive root URL")
				assert.Equal(t, tt.expectedArchive, archiveRoot.String())
				assert.Equal(t, tt.expectedDist, entry.Distribution)
				assert.Equal(t, sources.SourceTypeDeb, entry.Type)
				assert.Contains(t, entry.Components, "main")
			} else {
				assert.Nil(t, entry, "Should not parse as distribution URL")
				assert.Nil(t, archiveRoot, "Should not return archive root")
			}
		})
	}
}
