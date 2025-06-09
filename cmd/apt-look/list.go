package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/rs/zerolog/log"

	"github.com/nicwaller/apt-look/pkg/apt"
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

	for _, src := range sourceList {
		repo, err := apt.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}
		count := 0
		ctx := context.TODO()
		_, err = repo.Update(ctx)
		if err != nil {
			return fmt.Errorf("failed to update repository: %w", err)
		}
		packageNames := make([]string, 0)
		for pkg, err := range repo.Packages(ctx) {
			if err != nil {
				return fmt.Errorf("failed to list packages: %w", err)
			}
			if !slices.Contains(packageNames, pkg.Package) {
				count++
				packageNames = append(packageNames, pkg.Package)
			}
		}
		slices.Sort(packageNames)
		for _, pkgName := range packageNames {
			_, _ = fmt.Fprintf(os.Stdout, "%s\n", pkgName)
		}
		log.Info().Msgf("%d packages found in %s", count, repo.DistributionRoot().String())
	}

	return nil
}
