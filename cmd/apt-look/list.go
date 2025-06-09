package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/nicwaller/apt-look/pkg/apt"
	"github.com/nicwaller/apt-look/pkg/deb822"
)

// Implementation functions (stubs for demonstration)
func runList(source, format string) error {
	log.Info().Msgf("Listing packages from: %s", source)
	log.Info().Msgf("Format: %s", format)

	// TODO: Implement actual repository parsing and package listing
	// This would involve:
	// 1. Parsing the source (source line vs file)
	// 2. Fetching Release and Packages files
	// 3. Parsing package metadata
	// 4. Formatting output according to --format flag

	sourceList, err := parseSourceInput(source)
	if err != nil {
		return fmt.Errorf("failed to parse source input: %w", err)
	}

	packageNames := make(map[string]bool) // for deduplication

	for _, src := range sourceList {
		repo, err := apt.Mount(src)
		if err != nil {
			return fmt.Errorf("failed to mount repository: %w", err)
		}
		ctx := context.TODO()

		count := 0
		for pkg, err := range repo.Packages(ctx) {
			if err != nil {
				return fmt.Errorf("failed to list packages: %w", err)
			}
			if !packageNames[pkg.Package] {
				if err := outputPackage(pkg, format); err != nil {
					return fmt.Errorf("failed to output package: %w", err)
				}
				packageNames[pkg.Package] = true
				count++
			}
		}
		log.Info().Msgf("%d packages found in %s", count, repo.DistributionRoot().String())
	}

	// Check if no packages were found and warn about architecture mismatch
	if len(packageNames) == 0 {
		for _, src := range sourceList {
			repo, err := apt.Mount(src)
			if err != nil {
				continue
			}

			availableArchs := repo.GetAvailableArchitectures(src.Components)
			if len(availableArchs) > 0 {
				log.Warn().Msgf("No packages found for current architecture. Available architectures: %v", availableArchs)
				break
			}
		}
	}

	return nil
}

// outputPackage outputs a single package in the specified format
func outputPackage(pkg *deb822.Package, format string) error {
	switch format {
	case "text":
		fmt.Printf("%s\n", pkg.Package)
	case "json":
		data, err := json.Marshal(pkg)
		if err != nil {
			return fmt.Errorf("failed to marshal package to JSON: %w", err)
		}
		fmt.Printf("%s\n", string(data))
	case "tsv":
		// TSV format: Package\tVersion\tArchitecture\tSection\tDescription
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
			pkg.Package,
			pkg.Version,
			pkg.Architecture,
			pkg.Section,
			strings.ReplaceAll(pkg.Description, "\n", " "))
	case "raw":
		// Output the raw RFC822 format
		fmt.Printf("Package: %s\n", pkg.Package)
		if pkg.Version != "" {
			fmt.Printf("Version: %s\n", pkg.Version)
		}
		if pkg.Architecture != "" {
			fmt.Printf("Architecture: %s\n", pkg.Architecture)
		}
		if pkg.Section != "" {
			fmt.Printf("Section: %s\n", pkg.Section)
		}
		if pkg.Priority != "" {
			fmt.Printf("Priority: %s\n", pkg.Priority)
		}
		if pkg.Maintainer != "" {
			fmt.Printf("Maintainer: %s\n", pkg.Maintainer)
		}
		if pkg.Size > 0 {
			fmt.Printf("Size: %d\n", pkg.Size)
		}
		if pkg.InstalledSize > 0 {
			fmt.Printf("Installed-Size: %d\n", pkg.InstalledSize)
		}
		if pkg.Homepage != "" {
			fmt.Printf("Homepage: %s\n", pkg.Homepage)
		}
		if pkg.Description != "" {
			fmt.Printf("Description: %s\n", pkg.Description)
		}
		if pkg.Filename != "" {
			fmt.Printf("Filename: %s\n", pkg.Filename)
		}
		if pkg.SHA256 != "" {
			fmt.Printf("SHA256: %s\n", pkg.SHA256)
		}
		if pkg.MD5sum != "" {
			fmt.Printf("MD5sum: %s\n", pkg.MD5sum)
		}
		if pkg.SHA1 != "" {
			fmt.Printf("SHA1: %s\n", pkg.SHA1)
		}
		// Add dependency fields if they exist
		if pkg.Depends != "" {
			fmt.Printf("Depends: %s\n", pkg.Depends)
		}
		if pkg.Recommends != "" {
			fmt.Printf("Recommends: %s\n", pkg.Recommends)
		}
		if pkg.Suggests != "" {
			fmt.Printf("Suggests: %s\n", pkg.Suggests)
		}
		if pkg.Conflicts != "" {
			fmt.Printf("Conflicts: %s\n", pkg.Conflicts)
		}
		if pkg.Provides != "" {
			fmt.Printf("Provides: %s\n", pkg.Provides)
		}
		fmt.Printf("\n") // Blank line between packages
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	return nil
}
