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

	// update control
	PhasedUpdatePercentage int `json:"phased_update_percentage,omitempty"`

	// License and origin information
	License string `json:"license,omitempty"`
	Vendor  string `json:"vendor,omitempty"`

	// Build information
	BuildDepends      string `json:"build_depends,omitempty"`
	BuildDependsIndep string `json:"build_depends_indep,omitempty"`
	BuildConflicts    string `json:"build_conflicts,omitempty"`

	// Raw RFC822 header for access to non-standard fields
	header rfc822.Header `json:"-"`
}

// ParsePackages parses an APT Packages file and returns an iterator over Package entries
func ParsePackages(r io.Reader) iter.Seq2[*Package, error] {
	return func(yield func(*Package, error) bool) {
		for header, err := range ParseRecords(r) {
			if err != nil {
				yield(nil, fmt.Errorf("parsing packages file: %w", err))
				return
			}

			pkg := &Package{header: header}
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

// parseFields extracts and validates all fields from the RFC822 header
func (p *Package) parseFields() error {
	// Parse mandatory fields
	p.Package = p.header.Get("Package")
	if p.Package == "" {
		return fmt.Errorf("package record must have Package field")
	}

	p.Filename = p.header.Get("Filename")
	if p.Filename == "" {
		return fmt.Errorf("package record must have Filename field")
	}

	sizeField := p.header.Get("Size")
	if sizeField == "" {
		return fmt.Errorf("package record must have Size field")
	}
	size, err := strconv.ParseInt(sizeField, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Size field: %w", err)
	}
	p.Size = size

	// Parse highly recommended fields
	p.Architecture = p.header.Get("Architecture")
	p.Version = p.header.Get("Version")
	p.SHA256 = p.header.Get("SHA256")
	p.Description = p.header.Get("Description")

	// Parse hash fields
	p.MD5sum = p.header.Get("MD5sum")
	p.SHA1 = p.header.Get("SHA1")
	p.SHA512 = p.header.Get("SHA512")

	// Parse control fields
	p.Priority = p.header.Get("Priority")
	p.Section = p.header.Get("Section")
	p.Source = p.header.Get("Source")
	p.Maintainer = p.header.Get("Maintainer")
	p.Homepage = p.header.Get("Homepage")

	// Parse Installed-Size with validation
	if installedSizeField := p.header.Get("Installed-Size"); installedSizeField != "" {
		installedSize, err := strconv.ParseInt(installedSizeField, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid Installed-Size field: %w", err)
		}
		p.InstalledSize = installedSize
	}

	// Parse dependency fields
	p.Depends = p.header.Get("Depends")
	p.PreDepends = p.header.Get("Pre-Depends")
	p.Recommends = p.header.Get("Recommends")
	p.Suggests = p.header.Get("Suggests")
	p.Enhances = p.header.Get("Enhances")
	p.Breaks = p.header.Get("Breaks")
	p.Conflicts = p.header.Get("Conflicts")
	p.Provides = p.header.Get("Provides")
	p.Replaces = p.header.Get("Replaces")

	// Parse multi-arch field
	p.MultiArch = p.header.Get("Multi-Arch")

	// Parse boolean fields
	p.Essential = parseBoolField(p.header.Get("Essential"))

	// Parse additional metadata
	p.Tag = p.header.Get("Tag")
	p.Task = p.header.Get("Task")
	p.DescriptionMd5 = p.header.Get("Description-md5")

	// Parse phased update percentage
	if phasedField := p.header.Get("Phased-update-Percentage"); phasedField != "" {
		phased, err := strconv.Atoi(phasedField)
		if err != nil {
			return fmt.Errorf("invalid Phased-update-Percentage field: %w", err)
		}
		if phased < 0 || phased > 100 {
			return fmt.Errorf("Phased-update-Percentage must be 0-100, got %d", phased)
		}
		p.PhasedUpdatePercentage = phased
	}

	// Parse additional fields
	p.License = p.header.Get("License")
	p.Vendor = p.header.Get("Vendor")
	p.BuildDepends = p.header.Get("Build-Depends")
	p.BuildDependsIndep = p.header.Get("Build-Depends-Indep")
	p.BuildConflicts = p.header.Get("Build-Conflicts")

	return nil
}

// GetField returns the raw field value from the underlying RFC822 header
func (p *Package) GetField(name string) string {
	return p.header.Get(name)
}

// HasField checks if a field exists in the underlying RFC822 header
func (p *Package) HasField(name string) bool {
	return p.header.Has(name)
}

// Fields returns all field names from the underlying RFC822 header
func (p *Package) Fields() []string {
	return p.header.Fields()
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
