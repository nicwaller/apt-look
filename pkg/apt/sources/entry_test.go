package sources

import (
	"testing"
)

func TestIsSourceLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "deb line",
			line: "deb http://archive.ubuntu.com/ubuntu jammy main",
			want: true,
		},
		{
			name: "deb-src line",
			line: "deb-src http://archive.ubuntu.com/ubuntu jammy main",
			want: true,
		},
		{
			name: "deb line with options",
			line: "[arch=amd64] deb http://archive.ubuntu.com/ubuntu jammy main",
			want: true,
		},
		{
			name: "comment line",
			line: "# deb http://archive.ubuntu.com/ubuntu jammy main",
			want: false,
		},
		{
			name: "empty line",
			line: "",
			want: false,
		},
		{
			name: "random text",
			line: "some random text",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSourceLine(tt.line); got != tt.want {
				t.Errorf("isSourceLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSourceType(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected SourceType
	}{
		{
			name:     "deb lowercase",
			typeStr:  "deb",
			expected: SourceTypeDeb,
		},
		{
			name:     "deb uppercase",
			typeStr:  "DEB",
			expected: SourceTypeDeb,
		},
		{
			name:     "deb-src lowercase",
			typeStr:  "deb-src",
			expected: SourceTypeSrc,
		},
		{
			name:     "deb-src uppercase",
			typeStr:  "DEB-SRC",
			expected: SourceTypeSrc,
		},
		{
			name:     "unknown type",
			typeStr:  "rpm",
			expected: SourceTypeUnknown,
		},
		{
			name:     "empty string",
			typeStr:  "",
			expected: SourceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSourceType(tt.typeStr); got != tt.expected {
				t.Errorf("parseSourceType() = %v, want %v", got, tt.expected)
			}
		})
	}
}
