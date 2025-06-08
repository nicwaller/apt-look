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
				URI:          "http://archive.ubuntu.com/ubuntu",
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
				URI:          "https://deb.debian.org/debian",
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
				URI:          "http://ppa.launchpad.net/test/ppa/ubuntu",
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
				URI:          "http://example.com/repo",
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
			got, err := parseSourceLine(tt.line, tt.lineNumber)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSourceLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr {
				return
			}
			
			if got.Type != tt.expected.Type {
				t.Errorf("parseSourceLine() Type = %v, want %v", got.Type, tt.expected.Type)
			}
			if got.URI != tt.expected.URI {
				t.Errorf("parseSourceLine() URI = %v, want %v", got.URI, tt.expected.URI)
			}
			if got.Distribution != tt.expected.Distribution {
				t.Errorf("parseSourceLine() Distribution = %v, want %v", got.Distribution, tt.expected.Distribution)
			}
			if len(got.Components) != len(tt.expected.Components) {
				t.Errorf("parseSourceLine() Components length = %v, want %v", len(got.Components), len(tt.expected.Components))
			} else {
				for i, comp := range got.Components {
					if comp != tt.expected.Components[i] {
						t.Errorf("parseSourceLine() Components[%d] = %v, want %v", i, comp, tt.expected.Components[i])
					}
				}
			}
			if len(got.Options) != len(tt.expected.Options) {
				t.Errorf("parseSourceLine() Options length = %v, want %v", len(got.Options), len(tt.expected.Options))
			} else {
				for key, value := range tt.expected.Options {
					if got.Options[key] != value {
						t.Errorf("parseSourceLine() Options[%s] = %v, want %v", key, got.Options[key], value)
					}
				}
			}
			if got.LineNumber != tt.expected.LineNumber {
				t.Errorf("parseSourceLine() LineNumber = %v, want %v", got.LineNumber, tt.expected.LineNumber)
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
			URI:          "http://archive.ubuntu.com/ubuntu",
			Distribution: "jammy",
			Components:   []string{"main", "restricted"},
			Options:      map[string]string{},
			LineNumber:   2,
		},
		{
			Type:         SourceTypeSrc,
			URI:          "http://archive.ubuntu.com/ubuntu",
			Distribution: "jammy",
			Components:   []string{"main", "restricted"},
			Options:      map[string]string{},
			LineNumber:   3,
		},
		{
			Type:         SourceTypeDeb,
			URI:          "http://archive.ubuntu.com/ubuntu",
			Distribution: "jammy",
			Components:   []string{"universe", "multiverse"},
			Options: map[string]string{
				"arch": "amd64",
			},
			LineNumber: 5,
		},
		{
			Type:         SourceTypeDeb,
			URI:          "https://deb.debian.org/debian",
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
		if entry.URI != exp.URI {
			t.Errorf("Entry[%d] URI = %v, want %v", i, entry.URI, exp.URI)
		}
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

func TestParseSources(t *testing.T) {
	input := `deb http://archive.ubuntu.com/ubuntu jammy main
invalid line without proper format
deb-src http://archive.ubuntu.com/ubuntu jammy main`

	reader := strings.NewReader(input)
	entries := make([]*Entry, 0)
	var errors []error

	for entry, err := range ParseSources(reader) {
		if err != nil {
			errors = append(errors, err)
		} else {
			entries = append(entries, entry)
		}
	}

	if len(errors) != 1 {
		t.Errorf("ParseSources() expected 1 error, got %d", len(errors))
	}

	if len(entries) != 1 {
		t.Errorf("ParseSources() expected 1 valid entry, got %d", len(entries))
	}

	if len(entries) > 0 {
		entry := entries[0]
		if entry.Type != SourceTypeDeb {
			t.Errorf("ParseSources() first entry Type = %v, want %v", entry.Type, SourceTypeDeb)
		}
		if entry.URI != "http://archive.ubuntu.com/ubuntu" {
			t.Errorf("ParseSources() first entry URI = %v, want http://archive.ubuntu.com/ubuntu", entry.URI)
		}
	}
}