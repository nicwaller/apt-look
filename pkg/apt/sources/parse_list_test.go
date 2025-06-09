package sources

import (
	"strings"
	"testing"
)

func TestParseSourceLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		lineNumber int
		expected   *Entry
		wantErr    bool
	}{
		{
			name:       "basic deb line",
			line:       "deb http://archive.ubuntu.com/ubuntu jammy main",
			lineNumber: 1,
			expected: &Entry{
				Type:         SourceTypeDeb,
				Distribution: "jammy",
				Components:   []string{"main"},
				Options:      map[string]string{},
				LineNumber:   1,
			},
			wantErr: false,
		},
		{
			name:       "deb-src line with multiple components",
			line:       "deb-src https://deb.debian.org/debian bookworm main contrib non-free",
			lineNumber: 2,
			expected: &Entry{
				Type:         SourceTypeSrc,
				Distribution: "bookworm",
				Components:   []string{"main", "contrib", "non-free"},
				Options:      map[string]string{},
				LineNumber:   2,
			},
			wantErr: false,
		},
		{
			name:       "line with options",
			line:       "deb [arch=amd64 trusted=yes] http://ppa.launchpad.net/test/ppa/ubuntu focal main",
			lineNumber: 3,
			expected: &Entry{
				Type:         SourceTypeDeb,
				Distribution: "focal",
				Components:   []string{"main"},
				Options: map[string]string{
					"arch":    "amd64",
					"trusted": "yes",
				},
				LineNumber: 3,
			},
			wantErr: false,
		},
		{
			name:       "line with boolean option",
			line:       "deb [signed-by=/usr/share/keyrings/test.gpg] http://example.com/repo stable main",
			lineNumber: 4,
			expected: &Entry{
				Type:         SourceTypeDeb,
				Distribution: "stable",
				Components:   []string{"main"},
				Options: map[string]string{
					"signed-by": "/usr/share/keyrings/test.gpg",
				},
				LineNumber: 4,
			},
			wantErr: false,
		},
		{
			name:       "empty line",
			line:       "",
			lineNumber: 5,
			expected:   nil,
			wantErr:    true,
		},
		{
			name:       "commented line",
			line:       "# deb http://archive.ubuntu.com/ubuntu jammy main",
			lineNumber: 6,
			expected:   nil,
			wantErr:    true,
		},
		{
			name:       "invalid format - too few fields",
			line:       "deb http://archive.ubuntu.com/ubuntu",
			lineNumber: 7,
			expected:   nil,
			wantErr:    true,
		},
		{
			name:       "unknown source type",
			line:       "rpm http://example.com/repo stable main",
			lineNumber: 8,
			expected:   nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSourceLine(tt.line, tt.lineNumber)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSourceLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got.Type != tt.expected.Type {
				t.Errorf("ParseSourceLine() Type = %v, want %v", got.Type, tt.expected.Type)
			}
			// TODO: figure out how to test archiveRoot
			if got.Distribution != tt.expected.Distribution {
				t.Errorf("ParseSourceLine() Distribution = %v, want %v", got.Distribution, tt.expected.Distribution)
			}
			if len(got.Components) != len(tt.expected.Components) {
				t.Errorf("ParseSourceLine() Components length = %v, want %v", len(got.Components), len(tt.expected.Components))
			} else {
				for i, comp := range got.Components {
					if comp != tt.expected.Components[i] {
						t.Errorf("ParseSourceLine() Components[%d] = %v, want %v", i, comp, tt.expected.Components[i])
					}
				}
			}
			if len(got.Options) != len(tt.expected.Options) {
				t.Errorf("ParseSourceLine() Options length = %v, want %v", len(got.Options), len(tt.expected.Options))
			} else {
				for key, value := range tt.expected.Options {
					if got.Options[key] != value {
						t.Errorf("ParseSourceLine() Options[%s] = %v, want %v", key, got.Options[key], value)
					}
				}
			}
			if got.LineNumber != tt.expected.LineNumber {
				t.Errorf("ParseSourceLine() LineNumber = %v, want %v", got.LineNumber, tt.expected.LineNumber)
			}
		})
	}
}

func TestParseSourcesList(t *testing.T) {
	input := `# This is a comment
deb http://archive.ubuntu.com/ubuntu jammy main restricted
deb-src http://archive.ubuntu.com/ubuntu jammy main restricted

deb [arch=amd64] http://archive.ubuntu.com/ubuntu jammy universe multiverse
# Another comment
deb https://deb.debian.org/debian bookworm main`

	expected := []Entry{
		{
			Type:         SourceTypeDeb,
			Distribution: "jammy",
			Components:   []string{"main", "restricted"},
			Options:      map[string]string{},
			LineNumber:   2,
		},
		{
			Type:         SourceTypeSrc,
			Distribution: "jammy",
			Components:   []string{"main", "restricted"},
			Options:      map[string]string{},
			LineNumber:   3,
		},
		{
			Type:         SourceTypeDeb,
			Distribution: "jammy",
			Components:   []string{"universe", "multiverse"},
			Options: map[string]string{
				"arch": "amd64",
			},
			LineNumber: 5,
		},
		{
			Type:         SourceTypeDeb,
			Distribution: "bookworm",
			Components:   []string{"main"},
			Options:      map[string]string{},
			LineNumber:   7,
		},
	}

	got, err := ParseSourcesList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseSourcesList() error = %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("ParseSourcesList() returned %d entries, want %d", len(got), len(expected))
	}

	for i, entry := range got {
		exp := expected[i]
		if entry.Type != exp.Type {
			t.Errorf("Entry[%d] Type = %v, want %v", i, entry.Type, exp.Type)
		}
		// TODO: figure out how to test archiveRoot
		if entry.Distribution != exp.Distribution {
			t.Errorf("Entry[%d] Distribution = %v, want %v", i, entry.Distribution, exp.Distribution)
		}
		if len(entry.Components) != len(exp.Components) {
			t.Errorf("Entry[%d] Components length = %v, want %v", i, len(entry.Components), len(exp.Components))
		}
		if entry.LineNumber != exp.LineNumber {
			t.Errorf("Entry[%d] LineNumber = %v, want %v", i, entry.LineNumber, exp.LineNumber)
		}
	}
}

func TestParseSourcesListErrors(t *testing.T) {
	input := `deb http://archive.ubuntu.com/ubuntu jammy main
invalid line without proper format
deb-src http://archive.ubuntu.com/ubuntu jammy main`

	_, err := ParseSourcesList(strings.NewReader(input))
	if err == nil {
		t.Errorf("ParseSourcesList() expected error for invalid line, got nil")
	}
}

func TestParseDeb822Sources(t *testing.T) {
	input := `Types: deb deb-src
URIs: https://deb.debian.org/debian
Suites: bookworm bookworm-updates
Components: main non-free-firmware
Enabled: yes
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg

Types: deb deb-src
URIs: https://security.debian.org/debian-security
Suites: bookworm-security
Components: main non-free-firmware
Enabled: yes
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg`

	// The parser generates entries in order: types x uris x suites
	// So for "Types: deb deb-src", "URIs: debian", "Suites: bookworm bookworm-updates"
	// We get: deb+debian+bookworm, deb-src+debian+bookworm, deb+debian+bookworm-updates, deb-src+debian+bookworm-updates
	expected := []Entry{
		// First record - Types: deb deb-src, Suites: bookworm bookworm-updates
		{
			Type:         SourceTypeDeb,
			Distribution: "bookworm",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 1,
		},
		{
			Type:         SourceTypeDeb,
			Distribution: "bookworm-updates",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 1,
		},
		{
			Type:         SourceTypeSrc,
			Distribution: "bookworm",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 1,
		},
		{
			Type:         SourceTypeSrc,
			Distribution: "bookworm-updates",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 1,
		},
		// Second record - Types: deb deb-src, Suites: bookworm-security
		{
			Type:         SourceTypeDeb,
			Distribution: "bookworm-security",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 2,
		},
		{
			Type:         SourceTypeSrc,
			Distribution: "bookworm-security",
			Components:   []string{"main", "non-free-firmware"},
			Options: map[string]string{
				"enabled":   "yes",
				"signed-by": "/usr/share/keyrings/debian-archive-keyring.gpg",
			},
			LineNumber: 2,
		},
	}

	got, err := ParseDeb822SourcesList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseDeb822SourcesList() error = %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("ParseDeb822SourcesList() returned %d entries, want %d", len(got), len(expected))
	}

	for i, entry := range got {
		exp := expected[i]
		if entry.Type != exp.Type {
			t.Errorf("Entry[%d] Type = %v, want %v", i, entry.Type, exp.Type)
		}
		// TODO: figure out how to test archiveRoot
		if entry.Distribution != exp.Distribution {
			t.Errorf("Entry[%d] Distribution = %v, want %v", i, entry.Distribution, exp.Distribution)
		}
		if len(entry.Components) != len(exp.Components) {
			t.Errorf("Entry[%d] Components length = %v, want %v", i, len(entry.Components), len(exp.Components))
		} else {
			for j, comp := range entry.Components {
				if comp != exp.Components[j] {
					t.Errorf("Entry[%d] Components[%d] = %v, want %v", i, j, comp, exp.Components[j])
				}
			}
		}
		if len(entry.Options) != len(exp.Options) {
			t.Errorf("Entry[%d] Options length = %v, want %v", i, len(entry.Options), len(exp.Options))
		} else {
			for key, value := range exp.Options {
				if entry.Options[key] != value {
					t.Errorf("Entry[%d] Options[%s] = %v, want %v", i, key, entry.Options[key], value)
				}
			}
		}
		if entry.LineNumber != exp.LineNumber {
			t.Errorf("Entry[%d] LineNumber = %v, want %v", i, entry.LineNumber, exp.LineNumber)
		}
	}
}

func TestParseDeb822SourcesWithMultipleURIs(t *testing.T) {
	input := `Types: deb
URIs: https://deb.debian.org/debian https://mirror.example.com/debian
Suites: bookworm
Components: main`

	got, err := ParseDeb822SourcesList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseDeb822SourcesList() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ParseDeb822SourcesList() returned %d entries, want 2", len(got))
	}

	for i, entry := range got {
		// TODO: figure out how to test archiveRoot
		if entry.Type != SourceTypeDeb {
			t.Errorf("Entry[%d] Type = %v, want %v", i, entry.Type, SourceTypeDeb)
		}
		if entry.Distribution != "bookworm" {
			t.Errorf("Entry[%d] Distribution = %v, want bookworm", i, entry.Distribution)
		}
	}
}

func TestParseDeb822SourcesErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing Types field",
			input: `URIs: https://deb.debian.org/debian
Suites: bookworm`,
			wantErr: "missing required field 'Types'",
		},
		{
			name: "missing URIs field",
			input: `Types: deb
Suites: bookworm`,
			wantErr: "missing required field 'URIs'",
		},
		{
			name: "missing Suites field",
			input: `Types: deb
URIs: https://deb.debian.org/debian`,
			wantErr: "missing required field 'Suites'",
		},
		{
			name: "unknown source type",
			input: `Types: rpm
URIs: https://example.com/repo
Suites: stable`,
			wantErr: "unknown source type: rpm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDeb822SourcesList(strings.NewReader(tt.input))
			if err == nil {
				t.Errorf("ParseDeb822SourcesList() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseDeb822SourcesList() error = %v, want error containing %v", err, tt.wantErr)
			}
		})
	}
}
