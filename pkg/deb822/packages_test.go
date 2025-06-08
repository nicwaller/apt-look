package deb822

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackagesBasic(t *testing.T) {
	// Test with Spotify packages file
	packagesFile, err := os.Open("testdata/spotify-packages.gz")
	require.NoError(t, err)
	defer packagesFile.Close()

	gz, err := gzip.NewReader(packagesFile)
	require.NoError(t, err)
	defer gz.Close()

	var packages []*Package
	for pkg, err := range ParsePackages(gz) {
		require.NoError(t, err)
		packages = append(packages, pkg)
	}

	require.Len(t, packages, 4) // Spotify has 4 packages

	// Test first package (spotify-client)
	pkg1 := packages[0]
	assert.Equal(t, "spotify-client", pkg1.Package)
	assert.Equal(t, "amd64", pkg1.Architecture)
	assert.Equal(t, "1:1.2.60.564.gcc6305cb", pkg1.Version)
	assert.Equal(t, "optional", pkg1.Priority)
	assert.Equal(t, "sound", pkg1.Section)
	assert.Equal(t, "Spotify <tux@spotify.com>", pkg1.Maintainer)
	assert.Equal(t, "https://www.spotify.com", pkg1.Homepage)
	assert.Equal(t, int64(144421884), pkg1.Size)
	assert.Equal(t, "43737ac2124c8935ddb62e3fd1a637efdd4595c5e1cb9fc5e2f746e385ad4a67", pkg1.SHA256)
	assert.Equal(t, "80cb7469ead84fd2080f3fe4be7cf8ab35f66159", pkg1.SHA1)
	assert.Equal(t, "7a9c4e388c3607a8733525ba8a3343b7", pkg1.MD5sum)
	assert.Equal(t, "pool/non-free/s/spotify-client/spotify-client_1.2.60.564.gcc6305cb_amd64.deb", pkg1.Filename)
	assert.Contains(t, pkg1.Description, "Spotify streaming music client")
	assert.Contains(t, pkg1.Depends, "libc6")
	assert.Contains(t, pkg1.Recommends, "libavcodec")

	// Test second package has different fields
	pkg2 := packages[1]
	assert.Equal(t, "spotify-client-0.9.17", pkg2.Package)
	assert.Greater(t, pkg2.InstalledSize, int64(0))
	assert.NotEmpty(t, pkg2.Provides)
	assert.NotEmpty(t, pkg2.Conflicts)
	assert.NotEmpty(t, pkg2.Replaces)
}

func TestParsePackagesDocker(t *testing.T) {
	// Test with Docker packages file (more complex)
	packagesFile, err := os.Open("testdata/docker-packages.gz")
	require.NoError(t, err)
	defer packagesFile.Close()

	gz, err := gzip.NewReader(packagesFile)
	require.NoError(t, err)
	defer gz.Close()

	var packages []*Package
	count := 0
	for pkg, err := range ParsePackages(gz) {
		require.NoError(t, err)
		packages = append(packages, pkg)
		count++
		if count >= 10 { // Just test first 10 packages for performance
			break
		}
	}

	require.Greater(t, len(packages), 5)

	// Test that all packages have mandatory fields
	for i, pkg := range packages {
		assert.NotEmpty(t, pkg.Package, "Package %d should have Package field", i)
		assert.NotEmpty(t, pkg.Filename, "Package %d should have Filename field", i)
		assert.Greater(t, pkg.Size, int64(0), "Package %d should have positive Size", i)
	}

	// Test first package fields
	pkg1 := packages[0]
	assert.Equal(t, "containerd.io", pkg1.Package)
	assert.Equal(t, "amd64", pkg1.Architecture)
	assert.Equal(t, "optional", pkg1.Priority)
	assert.Equal(t, "devel", pkg1.Section)
	assert.Contains(t, pkg1.Maintainer, "Containerd team")
	assert.NotEmpty(t, pkg1.SHA256)
	assert.NotEmpty(t, pkg1.SHA512)
	assert.Greater(t, pkg1.InstalledSize, int64(0))
}

func TestParsePackagesMicrosoft(t *testing.T) {
	// Test with Microsoft packages file
	packagesFile, err := os.Open("testdata/microsoft-packages.gz")
	require.NoError(t, err)
	defer packagesFile.Close()

	gz, err := gzip.NewReader(packagesFile)
	require.NoError(t, err)
	defer gz.Close()

	var packages []*Package
	count := 0
	for pkg, err := range ParsePackages(gz) {
		require.NoError(t, err)
		packages = append(packages, pkg)
		count++
		if count >= 5 { // Test first 5 packages
			break
		}
	}

	require.Greater(t, len(packages), 3)

	// Find a dotnet package to test specific fields
	var dotnetPkg *Package
	for _, pkg := range packages {
		if pkg.Package == "dotnet-runtime-deps-6.0" {
			dotnetPkg = pkg
			break
		}
	}

	if dotnetPkg != nil {
		assert.Equal(t, "libs", dotnetPkg.Section)
		assert.Equal(t, "standard", dotnetPkg.Priority)
		assert.Contains(t, dotnetPkg.Description, ".NET")
		assert.Contains(t, dotnetPkg.Homepage, "github.com")
		assert.Contains(t, dotnetPkg.Depends, "libgcc1")
	}
}

func TestPackageFieldValidation(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expectErr bool
		errMsg    string
	}{
		{
			name: "missing package field",
			input: `Filename: test.deb
Size: 1000`,
			expectErr: true,
			errMsg:    "Package field",
		},
		{
			name: "missing filename field",
			input: `Package: test
Size: 1000`,
			expectErr: true,
			errMsg:    "Filename field",
		},
		{
			name: "missing size field",
			input: `Package: test
Filename: test.deb`,
			expectErr: true,
			errMsg:    "Size field",
		},
		{
			name: "invalid size field",
			input: `Package: test
Filename: test.deb
Size: invalid`,
			expectErr: true,
			errMsg:    "invalid Size field",
		},
		{
			name: "invalid installed size",
			input: `Package: test
Filename: test.deb
Size: 1000
Installed-Size: invalid`,
			expectErr: true,
			errMsg:    "invalid Installed-Size field",
		},
		{
			name: "invalid phased update percentage",
			input: `Package: test
Filename: test.deb
Size: 1000
Phased-Update-Percentage: 150`,
			expectErr: true,
			errMsg:    "Phased-Update-Percentage must be 0-100",
		},
		{
			name: "valid minimal package",
			input: `Package: test
Filename: test.deb
Size: 1000`,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			packages := ParsePackages(strings.NewReader(tc.input))

			var pkg *Package
			var err error
			for p, e := range packages {
				pkg = p
				err = e
				break
			}

			if tc.expectErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, pkg)
				assert.Equal(t, "test", pkg.Package)
			}
		})
	}
}

func TestPackageDependencyParsing(t *testing.T) {
	input := `Package: test-pkg
Filename: test.deb
Size: 1000
Depends: libc6 (>= 2.30), libssl1.1
Recommends: curl | wget
Suggests: documentation
Conflicts: old-package
Provides: virtual-package`

	var pkg *Package
	for p, err := range ParsePackages(strings.NewReader(input)) {
		require.NoError(t, err)
		pkg = p
		break
	}

	require.NotNil(t, pkg)

	deps := pkg.GetDependencies()

	// Test dependency parsing
	assert.Contains(t, deps, "depends")
	assert.Contains(t, deps["depends"], "libc6 (>= 2.30)")
	assert.Contains(t, deps["depends"], "libssl1.1")

	assert.Contains(t, deps, "recommends")
	assert.Contains(t, deps["recommends"], "curl | wget")

	assert.Contains(t, deps, "suggests")
	assert.Contains(t, deps["suggests"], "documentation")

	assert.Contains(t, deps, "conflicts")
	assert.Contains(t, deps["conflicts"], "old-package")

	assert.Contains(t, deps, "provides")
	assert.Contains(t, deps["provides"], "virtual-package")
}

func TestPackageJSONSerialization(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	packagesFile, err := os.Open("testdata/spotify-packages.gz")
	require.NoError(t, err)
	defer packagesFile.Close()

	gz, err := gzip.NewReader(packagesFile)
	require.NoError(t, err)
	defer gz.Close()

	var original *Package
	for pkg, err := range ParsePackages(gz) {
		require.NoError(t, err)
		original = pkg
		break // Just test first package
	}

	require.NotNil(t, original)

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	assert.Equal(t, "spotify-client", jsonMap["package"])
	assert.Equal(t, "amd64", jsonMap["architecture"])
	assert.Contains(t, jsonMap, "filename")
	assert.Contains(t, jsonMap, "size")
	assert.Contains(t, jsonMap, "sha256")

	// Verify the record field is excluded
	assert.NotContains(t, jsonMap, "record")

	// Unmarshal back to struct
	var unmarshaled Package
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify key fields match (note: record field won't be preserved)
	assert.Equal(t, original.Package, unmarshaled.Package)
	assert.Equal(t, original.Architecture, unmarshaled.Architecture)
	assert.Equal(t, original.Version, unmarshaled.Version)
	assert.Equal(t, original.Filename, unmarshaled.Filename)
	assert.Equal(t, original.Size, unmarshaled.Size)
	assert.Equal(t, original.SHA256, unmarshaled.SHA256)

	t.Logf("JSON output: %s", string(jsonData))
}

func TestPackageFieldAccess(t *testing.T) {
	// Test field access methods
	packagesFile, err := os.Open("testdata/spotify-packages.gz")
	require.NoError(t, err)
	defer packagesFile.Close()

	gz, err := gzip.NewReader(packagesFile)
	require.NoError(t, err)
	defer gz.Close()

	var pkg *Package
	for p, err := range ParsePackages(gz) {
		require.NoError(t, err)
		pkg = p
		break
	}

	require.NotNil(t, pkg)

	// Test GetField
	assert.Equal(t, "spotify-client", pkg.GetField("Package"))
	assert.Equal(t, "", pkg.GetField("NonExistentField"))

	// Test HasField
	assert.True(t, pkg.HasField("Package"))
	assert.True(t, pkg.HasField("package")) // case-insensitive
	assert.False(t, pkg.HasField("NonExistentField"))

	// Test Fields
	fields := pkg.Fields()
	assert.Contains(t, fields, "Package")
	assert.Contains(t, fields, "Architecture")
	assert.Contains(t, fields, "Filename")
}

func TestAllPackagesFiles(t *testing.T) {
	// Test that all packages files in testdata can be parsed
	testdataDir := "testdata"
	files, err := filepath.Glob(filepath.Join(testdataDir, "*-packages.gz"))
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

			var packageCount int
			for pkg, err := range ParsePackages(gz) {
				require.NoError(t, err, "Failed to parse package %d in %s", packageCount+1, fileName)
				require.NotNil(t, pkg, "Package should not be nil for %s", fileName)

				// Basic validation - all packages should have these
				assert.NotEmpty(t, pkg.Package, "%s package %d should have Package field", fileName, packageCount+1)
				assert.NotEmpty(t, pkg.Filename, "%s package %d should have Filename field", fileName, packageCount+1)
				assert.Greater(t, pkg.Size, int64(0), "%s package %d should have positive Size", fileName, packageCount+1)

				packageCount++

				// For performance, limit testing to first 50 packages per file
				if packageCount >= 50 {
					break
				}
			}

			require.Greater(t, packageCount, 0, "No packages found in %s", fileName)
			t.Logf("%s: parsed %d package(s) successfully", fileName, packageCount)
		})
	}
}
