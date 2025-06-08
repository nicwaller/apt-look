package deb822

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nicwaller/apt-look/pkg/rfc822"
)

// HashEntry represents a single hash entry in MD5Sum, SHA1, or SHA256 fields
type HashEntry struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// Release represents an APT Release file with all standardized fields
type Release struct {
	// Mandatory fields
	Suite         string      `json:"suite,omitempty"` // Suite or Codename (at least one required)
	Codename      string      `json:"codename,omitempty"`
	Architectures []string    `json:"architectures"`
	Components    []string    `json:"components"`
	Date          time.Time   `json:"date"`
	SHA256        []HashEntry `json:"sha256,omitempty"`

	// Optional metadata fields
	Origin                       string     `json:"origin,omitempty"`
	Label                        string     `json:"label,omitempty"`
	Version                      string     `json:"version,omitempty"`
	ValidUntil                   *time.Time `json:"valid_until,omitempty"`
	NotAutomatic                 bool       `json:"not_automatic,omitempty"`
	ButAutomaticUpgrades         bool       `json:"but_automatic_upgrades,omitempty"`
	AcquireByHash                bool       `json:"acquire_by_hash,omitempty"`
	SignedBy                     []string   `json:"signed_by,omitempty"`
	PackagesRequireAuthorization string     `json:"packages_require_authorization,omitempty"`
	Changelogs                   string     `json:"changelogs,omitempty"`
	Snapshots                    string     `json:"snapshots,omitempty"`
	NoSupportForArchitectureAll  bool       `json:"no_support_for_architecture_all,omitempty"`

	// Legacy hash fields (not for security)
	MD5Sum []HashEntry `json:"md5sum,omitempty"`
	SHA1   []HashEntry `json:"sha1,omitempty"`

	// Raw RFC822 record for access to non-standard fields
	record rfc822.Record `json:"-"`
}

// ParseRelease parses an APT Release file from the given reader
func ParseRelease(r io.Reader) (*Release, error) {
	parser := rfc822.NewParser()

	var record rfc822.Record
	found := false
	for rec, err := range parser.ParseRecords(r) {
		if err != nil {
			return nil, fmt.Errorf("parsing release file: %w", err)
		}
		record = rec
		found = true
		break // Release files contain only one record
	}

	if !found {
		return nil, fmt.Errorf("no records found in release file")
	}

	release := &Release{record: record}
	if err := release.parseFields(); err != nil {
		return nil, fmt.Errorf("parsing release fields: %w", err)
	}

	return release, nil
}

// parseFields extracts and validates all fields from the RFC822 record
func (r *Release) parseFields() error {
	// Parse mandatory/important fields
	r.Suite = r.record.Get("Suite")
	r.Codename = r.record.Get("Codename")

	// At least one of Suite or Codename must be present
	if r.Suite == "" && r.Codename == "" {
		return fmt.Errorf("release file must have either Suite or Codename field")
	}

	// Parse architectures (required)
	archField := r.record.Get("Architectures")
	if archField == "" {
		return fmt.Errorf("release file must have Architectures field")
	}
	r.Architectures = strings.Fields(archField)

	// Parse components (usually required, but some repos like Kubernetes don't have it)
	compField := r.record.Get("Components")
	if compField != "" {
		r.Components = strings.Fields(compField)
	}

	// Parse date (required)
	dateField := r.record.Get("Date")
	if dateField == "" {
		return fmt.Errorf("release file must have Date field")
	}
	date, err := parseRFC1123(dateField)
	if err != nil {
		return fmt.Errorf("invalid Date field: %w", err)
	}
	r.Date = date

	// Parse SHA256 (required for modern repositories)
	sha256Lines := r.record.GetLines("SHA256")
	if len(sha256Lines) > 0 {
		entries, err := parseHashEntries(sha256Lines)
		if err != nil {
			return fmt.Errorf("invalid SHA256 field: %w", err)
		}
		r.SHA256 = entries
	}

	// Parse optional fields
	r.Origin = r.record.Get("Origin")
	r.Label = r.record.Get("Label")
	r.Version = r.record.Get("Version")

	// Parse ValidUntil
	if validUntilField := r.record.Get("Valid-Until"); validUntilField != "" {
		validUntil, err := parseRFC1123(validUntilField)
		if err != nil {
			return fmt.Errorf("invalid Valid-Until field: %w", err)
		}
		r.ValidUntil = &validUntil
	}

	// Parse boolean fields
	r.NotAutomatic = parseBoolField(r.record.Get("NotAutomatic"))
	r.ButAutomaticUpgrades = parseBoolField(r.record.Get("ButAutomaticUpgrades"))
	r.AcquireByHash = parseBoolField(r.record.Get("Acquire-By-Hash"))
	r.NoSupportForArchitectureAll = parseBoolField(r.record.Get("No-Support-for-Architecture-all"))

	// Parse SignedBy
	if signedByField := r.record.Get("Signed-By"); signedByField != "" {
		r.SignedBy = strings.Split(signedByField, ",")
		for i := range r.SignedBy {
			r.SignedBy[i] = strings.TrimSpace(r.SignedBy[i])
		}
	}

	// Parse other optional fields
	r.PackagesRequireAuthorization = r.record.Get("Packages-Require-Authorization")
	r.Changelogs = r.record.Get("Changelogs")
	r.Snapshots = r.record.Get("Snapshots")

	// Parse legacy hash fields
	if md5Lines := r.record.GetLines("MD5Sum"); len(md5Lines) > 0 {
		entries, err := parseHashEntries(md5Lines)
		if err != nil {
			return fmt.Errorf("invalid MD5Sum field: %w", err)
		}
		r.MD5Sum = entries
	}

	if sha1Lines := r.record.GetLines("SHA1"); len(sha1Lines) > 0 {
		entries, err := parseHashEntries(sha1Lines)
		if err != nil {
			return fmt.Errorf("invalid SHA1 field: %w", err)
		}
		r.SHA1 = entries
	}

	return nil
}

// parseHashEntries parses hash field lines into HashEntry structs
// Each line format: "hash size path"
func parseHashEntries(lines []string) ([]HashEntry, error) {
	var entries []HashEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid hash entry format: %q (expected 3 fields)", line)
		}

		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid size in hash entry %q: %w", line, err)
		}

		entries = append(entries, HashEntry{
			Hash: parts[0],
			Size: size,
			Path: parts[2],
		})
	}

	return entries, nil
}

// parseRFC1123 parses APT date format (RFC 1123 with variations)
func parseRFC1123(dateStr string) (time.Time, error) {
	// Try standard RFC 1123 format first: "Mon, 02 Jan 2006 15:04:05 MST"
	if t, err := time.Parse(time.RFC1123, dateStr); err == nil {
		return t, nil
	}

	// Try with single-digit day (some repos use this): "Mon, 2 Jan 2006 15:04:05 MST"
	if t, err := time.Parse("Mon, 2 Jan 2006 15:04:05 MST", dateStr); err == nil {
		return t, nil
	}

	// Try without day of week (some old repos): "02 Jan 2006 15:04:05 MST"
	if t, err := time.Parse("02 Jan 2006 15:04:05 MST", dateStr); err == nil {
		return t, nil
	}

	// Try with single-digit day without weekday: "2 Jan 2006 15:04:05 MST"
	if t, err := time.Parse("2 Jan 2006 15:04:05 MST", dateStr); err == nil {
		return t, nil
	}

	// Try non-standard format used by some repos: "Mon Jan 2 15:04:05 2006"
	if t, err := time.Parse("Mon Jan 2 15:04:05 2006", dateStr); err == nil {
		return t, nil
	}

	// Try ANSIC format: "Mon Jan _2 15:04:05 2006" (with padding space)
	if t, err := time.Parse(time.ANSIC, dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q with any known APT date format", dateStr)
}

// parseBoolField parses APT boolean fields (yes/no, true/false, 1/0)
func parseBoolField(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "1":
		return true
	default:
		return false
	}
}

// GetField returns the raw field value from the underlying RFC822 record
func (r *Release) GetField(name string) string {
	return r.record.Get(name)
}

// HasField checks if a field exists in the underlying RFC822 record
func (r *Release) HasField(name string) bool {
	return r.record.Has(name)
}

// Fields returns all field names from the underlying RFC822 record
func (r *Release) Fields() []string {
	return r.record.Fields()
}
