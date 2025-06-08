package deb822

import (
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"

	"github.com/nicwaller/apt-look/pkg/rfc822"
)

// Package represents a single package entry from an APT Packages file
type Package struct {
	// Mandatory fields
	Package  string `json:"package"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`

	// Highly recommended fields
	Architecture string `json:"architecture,omitempty"`
	Version      string `json:"version,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	Description  string `json:"description,omitempty"`

	// Additional hash fields
	MD5sum string `json:"md5sum,omitempty"`
	SHA1   string `json:"sha1,omitempty"`
	SHA512 string `json:"sha512,omitempty"`

	// Control fields (must match .deb control file if present)
	Priority      string `json:"priority,omitempty"`
	Section       string `json:"section,omitempty"`
	Source        string `json:"source,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	InstalledSize int64  `json:"installed_size,omitempty"`
	Homepage      string `json:"homepage,omitempty"`

	// Dependency fields
	Depends    string `json:"depends,omitempty"`
	PreDepends string `json:"pre_depends,omitempty"`
	Recommends string `json:"recommends,omitempty"`
	Suggests   string `json:"suggests,omitempty"`
	Enhances   string `json:"enhances,omitempty"`
	Breaks     string `json:"breaks,omitempty"`
	Conflicts  string `json:"conflicts,omitempty"`
	Provides   string `json:"provides,omitempty"`
	Replaces   string `json:"replaces,omitempty"`

	// Multi-architecture support
	MultiArch string `json:"multi_arch,omitempty"`

	// Additional metadata
	Essential bool   `json:"essential,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Task      string `json:"task,omitempty"`

	// Translation and localization
	DescriptionMd5 string `json:"description_md5,omitempty"`

	// Update control
	PhasedUpdatePercentage int `json:"phased_update_percentage,omitempty"`

	// License and origin information
	License string `json:"license,omitempty"`
	Vendor  string `json:"vendor,omitempty"`

	// Build information
	BuildDepends      string `json:"build_depends,omitempty"`
	BuildDependsIndep string `json:"build_depends_indep,omitempty"`
	BuildConflicts    string `json:"build_conflicts,omitempty"`

	// Raw RFC822 record for access to non-standard fields
	record rfc822.Record `json:"-"`
}

// ParsePackages parses an APT Packages file and returns an iterator over Package entries
func ParsePackages(r io.Reader) iter.Seq2[*Package, error] {
	parser := rfc822.NewParser()

	return func(yield func(*Package, error) bool) {
		for record, err := range parser.ParseRecords(r) {
			if err != nil {
				yield(nil, fmt.Errorf("parsing packages file: %w", err))
				return
			}

			pkg := &Package{record: record}
			if err := pkg.parseFields(); err != nil {
				yield(nil, fmt.Errorf("parsing package fields: %w", err))
				return
			}

			if !yield(pkg, nil) {
				return // Stop iteration if yield returns false
			}
		}
	}
}

// parseFields extracts and validates all fields from the RFC822 record
func (p *Package) parseFields() error {
	// Parse mandatory fields
	p.Package = p.record.Get("Package")
	if p.Package == "" {
		return fmt.Errorf("package record must have Package field")
	}

	p.Filename = p.record.Get("Filename")
	if p.Filename == "" {
		return fmt.Errorf("package record must have Filename field")
	}

	sizeField := p.record.Get("Size")
	if sizeField == "" {
		return fmt.Errorf("package record must have Size field")
	}
	size, err := strconv.ParseInt(sizeField, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Size field: %w", err)
	}
	p.Size = size

	// Parse highly recommended fields
	p.Architecture = p.record.Get("Architecture")
	p.Version = p.record.Get("Version")
	p.SHA256 = p.record.Get("SHA256")
	p.Description = p.record.Get("Description")

	// Parse hash fields
	p.MD5sum = p.record.Get("MD5sum")
	p.SHA1 = p.record.Get("SHA1")
	p.SHA512 = p.record.Get("SHA512")

	// Parse control fields
	p.Priority = p.record.Get("Priority")
	p.Section = p.record.Get("Section")
	p.Source = p.record.Get("Source")
	p.Maintainer = p.record.Get("Maintainer")
	p.Homepage = p.record.Get("Homepage")

	// Parse Installed-Size with validation
	if installedSizeField := p.record.Get("Installed-Size"); installedSizeField != "" {
		installedSize, err := strconv.ParseInt(installedSizeField, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid Installed-Size field: %w", err)
		}
		p.InstalledSize = installedSize
	}

	// Parse dependency fields
	p.Depends = p.record.Get("Depends")
	p.PreDepends = p.record.Get("Pre-Depends")
	p.Recommends = p.record.Get("Recommends")
	p.Suggests = p.record.Get("Suggests")
	p.Enhances = p.record.Get("Enhances")
	p.Breaks = p.record.Get("Breaks")
	p.Conflicts = p.record.Get("Conflicts")
	p.Provides = p.record.Get("Provides")
	p.Replaces = p.record.Get("Replaces")

	// Parse multi-arch field
	p.MultiArch = p.record.Get("Multi-Arch")

	// Parse boolean fields
	p.Essential = parseBoolField(p.record.Get("Essential"))

	// Parse additional metadata
	p.Tag = p.record.Get("Tag")
	p.Task = p.record.Get("Task")
	p.DescriptionMd5 = p.record.Get("Description-md5")

	// Parse phased update percentage
	if phasedField := p.record.Get("Phased-Update-Percentage"); phasedField != "" {
		phased, err := strconv.Atoi(phasedField)
		if err != nil {
			return fmt.Errorf("invalid Phased-Update-Percentage field: %w", err)
		}
		if phased < 0 || phased > 100 {
			return fmt.Errorf("Phased-Update-Percentage must be 0-100, got %d", phased)
		}
		p.PhasedUpdatePercentage = phased
	}

	// Parse additional fields
	p.License = p.record.Get("License")
	p.Vendor = p.record.Get("Vendor")
	p.BuildDepends = p.record.Get("Build-Depends")
	p.BuildDependsIndep = p.record.Get("Build-Depends-Indep")
	p.BuildConflicts = p.record.Get("Build-Conflicts")

	return nil
}

// GetField returns the raw field value from the underlying RFC822 record
func (p *Package) GetField(name string) string {
	return p.record.Get(name)
}

// HasField checks if a field exists in the underlying RFC822 record
func (p *Package) HasField(name string) bool {
	return p.record.Has(name)
}

// Fields returns all field names from the underlying RFC822 record
func (p *Package) Fields() []string {
	return p.record.Fields()
}

// GetDependencies parses and returns dependency relationships as structured data
func (p *Package) GetDependencies() map[string][]string {
	deps := make(map[string][]string)

	if p.Depends != "" {
		deps["depends"] = parseDependencyList(p.Depends)
	}
	if p.PreDepends != "" {
		deps["pre-depends"] = parseDependencyList(p.PreDepends)
	}
	if p.Recommends != "" {
		deps["recommends"] = parseDependencyList(p.Recommends)
	}
	if p.Suggests != "" {
		deps["suggests"] = parseDependencyList(p.Suggests)
	}
	if p.Enhances != "" {
		deps["enhances"] = parseDependencyList(p.Enhances)
	}
	if p.Breaks != "" {
		deps["breaks"] = parseDependencyList(p.Breaks)
	}
	if p.Conflicts != "" {
		deps["conflicts"] = parseDependencyList(p.Conflicts)
	}
	if p.Provides != "" {
		deps["provides"] = parseDependencyList(p.Provides)
	}
	if p.Replaces != "" {
		deps["replaces"] = parseDependencyList(p.Replaces)
	}

	return deps
}

// parseDependencyList parses APT dependency strings into individual package references
func parseDependencyList(depString string) []string {
	// Basic parsing - split on commas and clean up
	// Note: This is a simplified parser. Full APT dependency parsing is quite complex
	// with version constraints, alternatives (|), and architecture specifications
	var deps []string
	parts := strings.Split(depString, ",")
	for _, part := range parts {
		dep := strings.TrimSpace(part)
		if dep != "" {
			deps = append(deps, dep)
		}
	}
	return deps
}
