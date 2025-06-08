package aptrepo

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourcesBasic(t *testing.T) {
	input := `# Main repository
deb http://deb.debian.org/debian bookworm main contrib non-free
deb-src http://deb.debian.org/debian bookworm main contrib non-free

# Security updates
deb http://security.debian.org/debian-security bookworm-security main
# deb-src http://security.debian.org/debian-security bookworm-security main

# Commented out repository
# deb http://example.com/repo unstable main`

	var entries []*SourceEntry
	for entry, err := range ParseSources(strings.NewReader(input)) {
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	require.Len(t, entries, 5)

	// Test first entry (enabled deb)
	entry1 := entries[0]
	assert.Equal(t, SourceTypeDeb, entry1.Type)
	assert.Equal(t, "http://deb.debian.org/debian", entry1.URI)
	assert.Equal(t, "bookworm", entry1.Distribution)
	assert.Equal(t, []string{"main", "contrib", "non-free"}, entry1.Components)
	assert.True(t, entry1.Enabled)
	assert.Equal(t, 2, entry1.LineNumber)

	// Test second entry (enabled deb-src)
	entry2 := entries[1]
	assert.Equal(t, SourceTypeSrc, entry2.Type)
	assert.Equal(t, "http://deb.debian.org/debian", entry2.URI)
	assert.Equal(t, "bookworm", entry2.Distribution)
	assert.Equal(t, []string{"main", "contrib", "non-free"}, entry2.Components)
	assert.True(t, entry2.Enabled)

	// Test third entry (security updates)
	entry3 := entries[2]
	assert.Equal(t, SourceTypeDeb, entry3.Type)
	assert.Equal(t, "http://security.debian.org/debian-security", entry3.URI)
	assert.Equal(t, "bookworm-security", entry3.Distribution)
	assert.Equal(t, []string{"main"}, entry3.Components)
	assert.True(t, entry3.Enabled)

	// Test fourth entry (commented out deb-src)
	entry4 := entries[3]
	assert.Equal(t, SourceTypeSrc, entry4.Type)
	assert.Equal(t, "http://security.debian.org/debian-security", entry4.URI)
	assert.Equal(t, "bookworm-security", entry4.Distribution)
	assert.Equal(t, []string{"main"}, entry4.Components)
	assert.False(t, entry4.Enabled) // This one is commented out

	// Test fifth entry (commented out deb)
	entry5 := entries[4]
	assert.Equal(t, SourceTypeDeb, entry5.Type)
	assert.Equal(t, "http://example.com/repo", entry5.URI)
	assert.Equal(t, "unstable", entry5.Distribution)
	assert.Equal(t, []string{"main"}, entry5.Components)
	assert.False(t, entry5.Enabled) // This one is commented out
}

func TestParseSourcesWithOptions(t *testing.T) {
	input := `deb [arch=amd64,arm64 trusted=yes] http://example.com/repo stable main
deb [arch=amd64] https://secure.example.com/repo testing main contrib
deb [signed-by=/etc/apt/keyrings/example.gpg] http://example.com/repo unstable main`

	var entries []*SourceEntry
	for entry, err := range ParseSources(strings.NewReader(input)) {
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	require.Len(t, entries, 3)

	// Test first entry with multiple options
	entry1 := entries[0]
	assert.Equal(t, SourceTypeDeb, entry1.Type)
	assert.Equal(t, "http://example.com/repo", entry1.URI)
	assert.Equal(t, "stable", entry1.Distribution)
	assert.Equal(t, []string{"main"}, entry1.Components)
	assert.Equal(t, "amd64,arm64", entry1.Options["arch"])
	assert.Equal(t, "yes", entry1.Options["trusted"])
	assert.True(t, entry1.Enabled)

	// Test second entry
	entry2 := entries[1]
	assert.Equal(t, "amd64", entry2.Options["arch"])
	assert.Equal(t, 1, len(entry2.Options)) // Only arch option

	// Test third entry with signed-by option
	entry3 := entries[2]
	assert.Equal(t, "/etc/apt/keyrings/example.gpg", entry3.Options["signed-by"])
}

func TestParseSourcesRealWorld(t *testing.T) {
	// Test with the actual sources.list from the project
	input := `deb http://repository.spotify.com/ stable non-free
deb https://apt.postgresql.org/pub/repos/apt/ jammy-pgdg main
deb https://apt.releases.hashicorp.com/ jammy main
deb https://brave-browser-apt-release.s3.brave.com/ stable main
deb https://deb.nodesource.com/node_20.x/ jammy main
deb https://dl.google.com/linux/chrome/deb/ stable main
deb https://download.docker.com/linux/ubuntu/ jammy stable
deb https://packages.microsoft.com/ubuntu/22.04/prod/ jammy main
deb https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /
deb https://updates.signal.org/desktop/apt/ xenial main`

	sourcesList, err := ParseSourcesList(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, sourcesList.Entries, 10)

	// Test Spotify entry
	spotify := sourcesList.Entries[0]
	assert.Equal(t, SourceTypeDeb, spotify.Type)
	assert.Equal(t, "http://repository.spotify.com/", spotify.URI)
	assert.Equal(t, "stable", spotify.Distribution)
	assert.Equal(t, []string{"non-free"}, spotify.Components)
	assert.True(t, spotify.Enabled)

	// Test Kubernetes entry (special case with root distribution)
	k8s := sourcesList.Entries[8]
	assert.Equal(t, SourceTypeDeb, k8s.Type)
	assert.Equal(t, "https://pkgs.k8s.io/core:/stable:/v1.28/deb/", k8s.URI)
	assert.Equal(t, "/", k8s.Distribution)
	assert.Empty(t, k8s.Components) // No components specified

	// Test that all entries are enabled
	for i, entry := range sourcesList.Entries {
		assert.True(t, entry.Enabled, "Entry %d should be enabled", i)
		assert.Equal(t, SourceTypeDeb, entry.Type, "Entry %d should be deb type", i)
	}
}

func TestParseSourcesEdgeCases(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "missing fields",
			input:     "deb http://example.com/",
			expectErr: true,
			errMsg:    "expected at least 3 fields",
		},
		{
			name:      "unknown source type",
			input:     "rpm http://example.com/repo stable main",
			expectErr: true,
			errMsg:    "unknown source type",
		},
		{
			name:      "missing URI",
			input:     "deb stable",
			expectErr: true,
			errMsg:    "expected at least 3 fields",
		},
		{
			name:      "valid minimal entry",
			input:     "deb http://example.com/repo stable",
			expectErr: false,
		},
		{
			name:      "valid with root distribution",
			input:     "deb http://example.com/repo /",
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			var entry *SourceEntry
			for e, er := range ParseSources(strings.NewReader(tc.input)) {
				entry = e
				err = er
				break
			}

			if tc.expectErr {
				assert.Error(t, err)
				if tc.errMsg != "" && err != nil {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if err == nil {
					assert.NotNil(t, entry)
				}
			}
		})
	}
}

func TestSourcesListFiltering(t *testing.T) {
	input := `deb http://deb.debian.org/debian bookworm main
deb-src http://deb.debian.org/debian bookworm main
# deb http://example.com/repo unstable main
deb http://example.com/repo stable main`

	sourcesList, err := ParseSourcesList(strings.NewReader(input))
	require.NoError(t, err)

	// Test GetEnabledEntries
	enabled := sourcesList.GetEnabledEntries()
	assert.Len(t, enabled, 3)

	// Test GetDisabledEntries
	disabled := sourcesList.GetDisabledEntries()
	assert.Len(t, disabled, 1)
	assert.Equal(t, "http://example.com/repo", disabled[0].URI)

	// Test GetByType
	debEntries := sourcesList.GetByType(SourceTypeDeb)
	assert.Len(t, debEntries, 3) // 2 enabled + 1 disabled

	srcEntries := sourcesList.GetByType(SourceTypeSrc)
	assert.Len(t, srcEntries, 1)

	// Test GetByURI
	debianEntries := sourcesList.GetByURI("http://deb.debian.org/debian")
	assert.Len(t, debianEntries, 2) // One deb, one deb-src
}

func TestSourceEntryMethods(t *testing.T) {
	entry := &SourceEntry{
		Type:         SourceTypeDeb,
		URI:          "http://example.com/repo",
		Distribution: "stable",
		Components:   []string{"main", "contrib", "non-free"},
		Options: map[string]string{
			"arch":    "amd64",
			"trusted": "yes",
		},
		Enabled: true,
	}

	// Test HasComponent
	assert.True(t, entry.HasComponent("main"))
	assert.True(t, entry.HasComponent("contrib"))
	assert.False(t, entry.HasComponent("universe"))

	// Test GetOption
	assert.Equal(t, "amd64", entry.GetOption("arch", ""))
	assert.Equal(t, "yes", entry.GetOption("trusted", "no"))
	assert.Equal(t, "default", entry.GetOption("nonexistent", "default"))

	// Test HasOption
	assert.True(t, entry.HasOption("arch"))
	assert.True(t, entry.HasOption("trusted"))
	assert.False(t, entry.HasOption("nonexistent"))
}

func TestSourceEntryString(t *testing.T) {
	testCases := []struct {
		name     string
		entry    SourceEntry
		expected string
	}{
		{
			name: "simple enabled entry",
			entry: SourceEntry{
				Type:         SourceTypeDeb,
				URI:          "http://example.com/repo",
				Distribution: "stable",
				Components:   []string{"main"},
				Enabled:      true,
			},
			expected: "deb http://example.com/repo stable main",
		},
		{
			name: "disabled entry",
			entry: SourceEntry{
				Type:         SourceTypeDeb,
				URI:          "http://example.com/repo",
				Distribution: "stable",
				Components:   []string{"main"},
				Enabled:      false,
			},
			expected: "# deb http://example.com/repo stable main",
		},
		{
			name: "entry with options",
			entry: SourceEntry{
				Type:         SourceTypeDeb,
				URI:          "http://example.com/repo",
				Distribution: "stable",
				Components:   []string{"main", "contrib"},
				Options: map[string]string{
					"arch":    "amd64",
					"trusted": "yes",
				},
				Enabled: true,
			},
			expected: "[arch=amd64 trusted=yes] deb http://example.com/repo stable main contrib",
		},
		{
			name: "deb-src with no components",
			entry: SourceEntry{
				Type:         SourceTypeSrc,
				URI:          "http://example.com/repo",
				Distribution: "/",
				Enabled:      true,
			},
			expected: "deb-src http://example.com/repo /",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entry.String()
			// Note: Options order might vary, so we check that all parts are present
			if len(tc.entry.Options) > 0 {
				// For entries with options, just check the main parts
				assert.Contains(t, result, string(tc.entry.Type))
				assert.Contains(t, result, tc.entry.URI)
				assert.Contains(t, result, tc.entry.Distribution)
				for key, value := range tc.entry.Options {
					if value == "true" {
						assert.Contains(t, result, key)
					} else {
						assert.Contains(t, result, fmt.Sprintf("%s=%s", key, value))
					}
				}
			} else {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestSourcesJSONSerialization(t *testing.T) {
	input := `deb [arch=amd64 trusted=yes] http://example.com/repo stable main contrib
# deb-src http://example.com/repo stable main`

	sourcesList, err := ParseSourcesList(strings.NewReader(input))
	require.NoError(t, err)

	// Marshal to JSON
	jsonData, err := json.Marshal(sourcesList)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	assert.Contains(t, jsonMap, "entries")
	entries := jsonMap["entries"].([]interface{})
	assert.Len(t, entries, 2)

	// Check first entry
	entry1 := entries[0].(map[string]interface{})
	assert.Equal(t, "deb", entry1["type"])
	assert.Equal(t, "http://example.com/repo", entry1["uri"])
	assert.Equal(t, "stable", entry1["distribution"])
	assert.True(t, entry1["enabled"].(bool))

	// Check second entry (disabled)
	entry2 := entries[1].(map[string]interface{})
	assert.Equal(t, "deb-src", entry2["type"])
	assert.False(t, entry2["enabled"].(bool))

	// Unmarshal back to struct
	var unmarshaled SourcesList
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify key fields match
	assert.Len(t, unmarshaled.Entries, 2)
	assert.Equal(t, sourcesList.Entries[0].Type, unmarshaled.Entries[0].Type)
	assert.Equal(t, sourcesList.Entries[0].URI, unmarshaled.Entries[0].URI)
	assert.Equal(t, sourcesList.Entries[0].Enabled, unmarshaled.Entries[0].Enabled)

	t.Logf("JSON output: %s", string(jsonData))
}

func TestParseSourcesComments(t *testing.T) {
	input := `# This is a comment
deb http://example.com/repo stable main

# Another comment
# deb http://disabled.example.com/repo stable main

# Just a comment without source info
# Some explanation about repositories

deb-src http://example.com/repo stable main`

	var entries []*SourceEntry
	for entry, err := range ParseSources(strings.NewReader(input)) {
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	// Should only find 3 entries: 2 enabled, 1 disabled
	require.Len(t, entries, 3)

	assert.True(t, entries[0].Enabled)  // deb entry
	assert.False(t, entries[1].Enabled) // disabled deb entry
	assert.True(t, entries[2].Enabled)  // deb-src entry
}