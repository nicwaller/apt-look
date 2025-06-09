package apt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nicwaller/apt-look/pkg/apt/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_FileURL(t *testing.T) {
	// Get absolute path to our test repository
	testRepoPath, err := filepath.Abs("testdata/emptyrepo")
	require.NoError(t, err)

	// Create a source entry with file:// URL
	sourceLine := "deb file://" + testRepoPath + " stable main"
	entry, err := sources.ParseSourceLine(sourceLine, 1)
	require.NoError(t, err)

	// Test apt.Open()
	repo, err := Open(*entry)
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "file", repo.archiveRoot.Scheme)
	assert.Equal(t, testRepoPath, repo.archiveRoot.Path)
	assert.Contains(t, repo.components, "main")

	// Test that we can fetch the Release file
	ctx := context.Background()
	release, err := repo.Update(ctx)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Verify Release file content
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
	assert.Contains(t, release.Architectures, "amd64")
	assert.Contains(t, release.Components, "main")

	// Test that we can iterate over packages (should be empty)
	var packageCount int
	for pkg, err := range repo.Packages(ctx) {
		require.NoError(t, err)
		packageCount++
		_ = pkg // Use the variable to avoid unused variable error
	}
	assert.Equal(t, 0, packageCount, "Empty repository should have no packages")
}

func TestOpen_HTTPSURL(t *testing.T) {
	// Create a source entry with HTTPS URL pointing to our S3-hosted repository
	sourceLine := "deb https://nicwaller-apt.s3.ca-central-1.amazonaws.com stable main"
	entry, err := sources.ParseSourceLine(sourceLine, 1)
	require.NoError(t, err)

	// Test apt.Open()
	repo, err := Open(*entry)
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify the repository was opened correctly
	assert.Equal(t, "https", repo.archiveRoot.Scheme)
	assert.Equal(t, "nicwaller-apt.s3.ca-central-1.amazonaws.com", repo.archiveRoot.Host)
	assert.Contains(t, repo.components, "main")

	// Test that we can fetch the Release file
	ctx := context.Background()
	release, err := repo.Update(ctx)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Verify Release file content matches our uploaded repository
	assert.Equal(t, "Test Repository", release.Origin)
	assert.Equal(t, "stable", release.Suite)
	assert.Contains(t, release.Architectures, "amd64")
	assert.Contains(t, release.Components, "main")

	// Test that we can iterate over packages (should be empty)
	var packageCount int
	for pkg, err := range repo.Packages(ctx) {
		require.NoError(t, err)
		packageCount++
		_ = pkg // Use the variable to avoid unused variable error
	}
	assert.Equal(t, 0, packageCount, "Empty repository should have no packages")
}