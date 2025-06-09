package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/rs/zerolog/log"
	"pault.ag/go/debian/version"

	"github.com/nicwaller/apt-look/pkg/apt"
	"github.com/nicwaller/apt-look/pkg/deb822"
)

// PackageKey represents the unique identifier for a package (name, architecture)
type PackageKey struct {
	Name         string
	Architecture string
}

// runLatest shows the latest version of each package grouped by (name, architecture)
func runLatest(source, format string) error {
	log.Info().Msgf("Finding latest packages from: %s", source)
	log.Info().Msgf("Format: %s", format)

	sourceList, err := parseSourceInput(source)
	if err != nil {
		return fmt.Errorf("failed to parse source input: %w", err)
	}

	// Map to store the latest version for each (name, architecture) pair
	latestPackages := make(map[PackageKey]*deb822.Package)

	for _, src := range sourceList {
		repo, err := apt.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}
		ctx := context.TODO()
		_, err = repo.Update(ctx)
		if err != nil {
			return fmt.Errorf("failed to update repository: %w", err)
		}

		count := 0
		for pkg, err := range repo.Packages(ctx) {
			if err != nil {
				return fmt.Errorf("failed to list packages: %w", err)
			}

			key := PackageKey{
				Name:         pkg.Package,
				Architecture: pkg.Architecture,
			}

			// Check if this is the first package with this key or has a higher version
			if existing, exists := latestPackages[key]; !exists || isNewerVersion(pkg.Version, existing.Version) {
				latestPackages[key] = pkg
				if !exists {
					count++
				}
			}
		}
		log.Info().Msgf("%d unique packages found in %s", count, repo.DistributionRoot().String())
	}

	// Convert to slice and sort by package name, then architecture
	packages := make([]*deb822.Package, 0, len(latestPackages))
	for _, pkg := range latestPackages {
		packages = append(packages, pkg)
	}

	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Package != packages[j].Package {
			return packages[i].Package < packages[j].Package
		}
		return packages[i].Architecture < packages[j].Architecture
	})

	// Output all packages in the requested format
	for _, pkg := range packages {
		if err := outputPackage(pkg, format); err != nil {
			return fmt.Errorf("failed to output package: %w", err)
		}
	}

	return nil
}

// isNewerVersion compares two Debian version strings and returns true if v1 is newer than v2
// Uses the proper Debian version comparison from pault.ag/go/debian/version
func isNewerVersion(v1, v2 string) bool {
	if v1 == v2 {
		return false
	}
	if v1 == "" {
		return false
	}
	if v2 == "" {
		return true
	}

	ver1, err1 := version.Parse(v1)
	if err1 != nil {
		// Fall back to string comparison if parsing fails
		return v1 > v2
	}

	ver2, err2 := version.Parse(v2)
	if err2 != nil {
		// Fall back to string comparison if parsing fails
		return v1 > v2
	}

	return version.Compare(ver1, ver2) > 0
}