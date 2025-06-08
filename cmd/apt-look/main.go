package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nicwaller/apt-look/pkg/aptrepo"
	"github.com/nicwaller/apt-look/pkg/apttransport"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var options struct {
	format string
	output string
}

// Root command
var rootCmd = &cobra.Command{
	Use:   "apt-look",
	Short: "Explore APT repositories without system configuration",
	Long: `apt-look is a tool for exploring remote APT repositories.
It allows you to list packages, get repository statistics, search for packages,
and download specific packages without requiring system APT configuration.`,
	Example: `  apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main"
  apt-look info /etc/apt/sources.list golang-1.21
  apt-look stats "deb http://archive.ubuntu.com/ubuntu/ jammy main"`,
}

// List command
var listCmd = &cobra.Command{
	Use:   "list <source>",
	Short: "List all packages in the repository",
	Long: `List all packages available in the specified APT repository.
Source can be either a full APT source line or a path to a sources.list file.`,
	Args: cobra.ExactArgs(1),
	Example: `  apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main"
  apt-look list /etc/apt/sources.list
  apt-look list /etc/apt/sources.list.d/docker.list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		return runList(source, options.format)
	},
}

// Info command
var infoCmd = &cobra.Command{
	Use:   "info <source> <package>",
	Short: "Show detailed information about a specific package",
	Long: `Display detailed metadata for a specific package including version,
dependencies, description, and other available information.`,
	Args: cobra.ExactArgs(2),
	Example: `  apt-look info "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21
  apt-look info /etc/apt/sources.list python3-requests --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		packageName := args[1]
		return runInfo(source, packageName, options.format)
	},
}

// Stats command
var statsCmd = &cobra.Command{
	Use:   "stats <source>",
	Short: "Show repository statistics",
	Long: `Display statistics about the repository including total number of packages,
total size, breakdown by component, and other metadata.`,
	Args: cobra.ExactArgs(1),
	Example: `  apt-look stats "deb http://archive.ubuntu.com/ubuntu/ jammy main"
  apt-look stats /etc/apt/sources.list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		return runStats(source, options.format)
	},
}

// Download command
var downloadCmd = &cobra.Command{
	Use:   "download <source> <package>",
	Short: "Download the latest version of a package",
	Long: `Download the latest version of the specified package from the repository.
The package will be saved to the current directory or the path specified with --output.`,
	Args: cobra.ExactArgs(2),
	Example: `  apt-look download "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21
  apt-look download /etc/apt/sources.list containerd --output=/tmp/packages/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		packageName := args[1]
		return runDownload(source, packageName, options.output)
	},
}

// Search command
var searchCmd = &cobra.Command{
	Use:   "search <source> <term>",
	Short: "Search for packages matching a term",
	Long: `Search for packages whose names or descriptions contain the specified term.
The search is case-insensitive and matches partial strings.`,
	Args: cobra.ExactArgs(2),
	Example: `  apt-look search "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang
  apt-look search /etc/apt/sources.list python --format=tsv`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		searchTerm := args[1]
		return runSearch(source, searchTerm, options.format)
	},
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVarP(&options.format, "format", "f", "text",
		"Output format (text, json, tsv, raw)")

	// Command-specific flags
	downloadCmd.Flags().StringVarP(&options.output, "output", "o", ".",
		"Output directory for downloaded packages")

	// Add validation for format flag
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		validFormats := []string{"text", "json", "tsv", "raw"}
		for _, validFormat := range validFormats {
			if options.format == validFormat {
				return nil
			}
		}
		return fmt.Errorf("invalid format '%s'. Valid formats: %s",
			options.format, strings.Join(validFormats, ", "))
	}

	// Add subcommands to root
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(searchCmd)
}

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

	return nil
}

func runInfo(source, packageName, format string) error {
	log.Info().Msgf("Getting info for package '%s' from: %s", packageName, source)
	log.Info().Msgf("Format: %s", format)

	// TODO: Implement package info retrieval
	// This would involve:
	// 1. Finding the package in the repository
	// 2. Extracting detailed metadata
	// 3. Formatting according to --format flag

	return nil
}

// RepositoryStats holds statistics about a repository
type RepositoryStats struct {
	Repository struct {
		Origin        string    `json:"origin,omitempty"`
		Label         string    `json:"label,omitempty"`
		Suite         string    `json:"suite,omitempty"`
		Codename      string    `json:"codename,omitempty"`
		Date          time.Time `json:"date"`
		Architectures []string  `json:"architectures"`
		Components    []string  `json:"components"`
	} `json:"repository"`
	
	Packages struct {
		Total           int   `json:"total"`
		TotalSize       int64 `json:"total_size_bytes"`
		TotalSizeMB     int64 `json:"total_size_mb"`
		ByArchitecture  map[string]int `json:"by_architecture"`
		ByComponent     map[string]int `json:"by_component"`
		BySection       map[string]int `json:"by_section"`
		ByPriority      map[string]int `json:"by_priority"`
	} `json:"packages"`
}

func runStats(source, format string) error {
	log.Info().Msgf("Getting statistics for: %s", source)

	// Parse source input
	sources, err := parseSourceInput(source)
	if err != nil {
		return fmt.Errorf("failed to parse source: %w", err)
	}

	if len(sources) == 0 {
		return fmt.Errorf("no enabled sources found")
	}

	// Use first enabled source for stats
	sourceEntry := sources[0]
	if !sourceEntry.Enabled {
		return fmt.Errorf("source is disabled")
	}

	// Calculate statistics
	stats, err := calculateRepositoryStats(sourceEntry)
	if err != nil {
		return fmt.Errorf("failed to calculate statistics: %w", err)
	}

	// Format and output results
	return outputStats(stats, format)
}

func parseSourceInput(source string) ([]aptrepo.SourceEntry, error) {
	// Check if it's a file path
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		file, err := os.Open(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open sources file: %w", err)
		}
		defer file.Close()

		sourcesList, err := aptrepo.ParseSourcesList(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sources file: %w", err)
		}

		return sourcesList.GetEnabledEntries(), nil
	}

	// Parse as single source line
	var entries []aptrepo.SourceEntry
	for entry, err := range aptrepo.ParseSources(strings.NewReader(source)) {
		if err != nil {
			return nil, fmt.Errorf("failed to parse source line: %w", err)
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

func calculateRepositoryStats(source aptrepo.SourceEntry) (*RepositoryStats, error) {
	stats := &RepositoryStats{}
	
	// Initialize transport
	transport := apttransport.NewHTTPTransport()

	// Fetch Release file
	releaseURL := strings.TrimSuffix(source.URI, "/") + "/dists/" + source.Distribution + "/Release"
	parsedURL, err := url.Parse(releaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Release URL %s: %w", releaseURL, err)
	}
	
	ctx := context.Background()
	resp, err := transport.Acquire(ctx, &apttransport.AcquireRequest{
		URI:     parsedURL,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Release file from %s: %w", releaseURL, err)
	}
	defer resp.Content.Close()

	release, err := aptrepo.ParseRelease(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Release file: %w", err)
	}

	// Fill repository info from Release file
	stats.Repository.Origin = release.Origin
	stats.Repository.Label = release.Label
	stats.Repository.Suite = release.Suite
	stats.Repository.Codename = release.Codename
	stats.Repository.Date = release.Date
	stats.Repository.Architectures = release.Architectures
	stats.Repository.Components = release.Components

	// Initialize counters
	stats.Packages.ByArchitecture = make(map[string]int)
	stats.Packages.ByComponent = make(map[string]int)
	stats.Packages.BySection = make(map[string]int)
	stats.Packages.ByPriority = make(map[string]int)

	// Fetch and parse Packages files for each component/architecture
	for _, component := range source.Components {
		if component == "" {
			continue
		}
		
		for _, arch := range release.Architectures {
			if arch == "source" {
				continue // Skip source architecture for binary package stats
			}

			err := processPackagesFile(transport, source, component, arch, stats)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to process packages for %s/%s", component, arch)
				continue
			}
		}
	}

	// Calculate derived statistics
	stats.Packages.TotalSizeMB = stats.Packages.TotalSize / (1024 * 1024)

	return stats, nil
}

func processPackagesFile(transport apttransport.Transport, source aptrepo.SourceEntry, component, arch string, stats *RepositoryStats) error {
	// Try different possible Packages file locations
	possiblePaths := []string{
		fmt.Sprintf("/dists/%s/%s/binary-%s/Packages.gz", source.Distribution, component, arch),
		fmt.Sprintf("/dists/%s/%s/binary-%s/Packages", source.Distribution, component, arch),
	}

	for _, path := range possiblePaths {
		packagesURL := strings.TrimSuffix(source.URI, "/") + path
		
		parsedURL, err := url.Parse(packagesURL)
		if err != nil {
			continue // Try next path
		}
		
		ctx := context.Background()
		resp, err := transport.Acquire(ctx, &apttransport.AcquireRequest{
			URI:     parsedURL,
			Timeout: 30 * time.Second,
		})
		if err != nil {
			continue // Try next path
		}
		defer resp.Content.Close()

		// Handle gzipped files
		var reader = resp.Content
		if strings.HasSuffix(path, ".gz") {
			gzReader, err := gzip.NewReader(resp.Content)
			if err != nil {
				resp.Content.Close()
				continue
			}
			defer gzReader.Close()
			reader = gzReader
		}

		// Parse packages and update stats
		packageCount := 0
		for pkg, err := range aptrepo.ParsePackages(reader) {
			if err != nil {
				log.Warn().Err(err).Msg("Error parsing package")
				continue
			}

			packageCount++
			stats.Packages.Total++
			stats.Packages.TotalSize += pkg.Size

			// Count by architecture
			if pkg.Architecture != "" {
				stats.Packages.ByArchitecture[pkg.Architecture]++
			}

			// Count by component
			stats.Packages.ByComponent[component]++

			// Count by section
			if pkg.Section != "" {
				stats.Packages.BySection[pkg.Section]++
			}

			// Count by priority
			if pkg.Priority != "" {
				stats.Packages.ByPriority[pkg.Priority]++
			}
		}

		log.Debug().Msgf("Processed %d packages from %s", packageCount, packagesURL)
		return nil // Successfully processed this path
	}

	return fmt.Errorf("could not fetch Packages file for %s/%s", component, arch)
}

func outputStats(stats *RepositoryStats, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)

	case "tsv":
		return outputStatsTSV(stats)

	case "raw":
		return outputStatsRaw(stats)

	case "text":
		fallthrough
	default:
		return outputStatsText(stats)
	}
}

func outputStatsText(stats *RepositoryStats) error {
	fmt.Printf("Repository Statistics\n")
	fmt.Printf("====================\n\n")

	// Repository information
	fmt.Printf("Repository Information:\n")
	if stats.Repository.Origin != "" {
		fmt.Printf("  Origin: %s\n", stats.Repository.Origin)
	}
	if stats.Repository.Label != "" {
		fmt.Printf("  Label: %s\n", stats.Repository.Label)
	}
	if stats.Repository.Suite != "" {
		fmt.Printf("  Suite: %s\n", stats.Repository.Suite)
	}
	if stats.Repository.Codename != "" {
		fmt.Printf("  Codename: %s\n", stats.Repository.Codename)
	}
	fmt.Printf("  Date: %s\n", stats.Repository.Date.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("  Architectures: %s\n", strings.Join(stats.Repository.Architectures, ", "))
	fmt.Printf("  Components: %s\n", strings.Join(stats.Repository.Components, ", "))

	// Package statistics
	fmt.Printf("\nPackage Statistics:\n")
	fmt.Printf("  Total Packages: %d\n", stats.Packages.Total)
	fmt.Printf("  Total Size: %d bytes (%.1f MB)\n", stats.Packages.TotalSize, float64(stats.Packages.TotalSize)/(1024*1024))

	if len(stats.Packages.ByArchitecture) > 0 {
		fmt.Printf("\n  By Architecture:\n")
		for arch, count := range stats.Packages.ByArchitecture {
			fmt.Printf("    %s: %d packages\n", arch, count)
		}
	}

	if len(stats.Packages.ByComponent) > 0 {
		fmt.Printf("\n  By Component:\n")
		for component, count := range stats.Packages.ByComponent {
			fmt.Printf("    %s: %d packages\n", component, count)
		}
	}

	if len(stats.Packages.BySection) > 0 {
		fmt.Printf("\n  By Section:\n")
		for section, count := range stats.Packages.BySection {
			fmt.Printf("    %s: %d packages\n", section, count)
		}
	}

	if len(stats.Packages.ByPriority) > 0 {
		fmt.Printf("\n  By Priority:\n")
		for priority, count := range stats.Packages.ByPriority {
			fmt.Printf("    %s: %d packages\n", priority, count)
		}
	}

	return nil
}

func outputStatsTSV(stats *RepositoryStats) error {
	fmt.Printf("field\tvalue\n")
	fmt.Printf("origin\t%s\n", stats.Repository.Origin)
	fmt.Printf("label\t%s\n", stats.Repository.Label)
	fmt.Printf("suite\t%s\n", stats.Repository.Suite)
	fmt.Printf("codename\t%s\n", stats.Repository.Codename)
	fmt.Printf("date\t%s\n", stats.Repository.Date.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("architectures\t%s\n", strings.Join(stats.Repository.Architectures, ","))
	fmt.Printf("components\t%s\n", strings.Join(stats.Repository.Components, ","))
	fmt.Printf("total_packages\t%d\n", stats.Packages.Total)
	fmt.Printf("total_size_bytes\t%d\n", stats.Packages.TotalSize)
	fmt.Printf("total_size_mb\t%d\n", stats.Packages.TotalSizeMB)

	for arch, count := range stats.Packages.ByArchitecture {
		fmt.Printf("arch_%s\t%d\n", arch, count)
	}

	for component, count := range stats.Packages.ByComponent {
		fmt.Printf("component_%s\t%d\n", component, count)
	}

	return nil
}

func outputStatsRaw(stats *RepositoryStats) error {
	fmt.Printf("Origin: %s\n", stats.Repository.Origin)
	fmt.Printf("Label: %s\n", stats.Repository.Label)
	fmt.Printf("Suite: %s\n", stats.Repository.Suite)
	fmt.Printf("Codename: %s\n", stats.Repository.Codename)
	fmt.Printf("Date: %s\n", stats.Repository.Date.Format("Mon, 02 Jan 2006 15:04:05 MST"))
	fmt.Printf("Architectures: %s\n", strings.Join(stats.Repository.Architectures, " "))
	fmt.Printf("Components: %s\n", strings.Join(stats.Repository.Components, " "))
	fmt.Printf("Total-Packages: %d\n", stats.Packages.Total)
	fmt.Printf("Total-Size: %d\n", stats.Packages.TotalSize)

	return nil
}

func runDownload(source, packageName, outputPath string) error {
	log.Info().Msgf("Downloading package '%s' from: %s", packageName, source)
	log.Info().Msgf("Output path: %s", outputPath)

	// TODO: Implement package download
	// This would involve:
	// 1. Finding the package and its download URL
	// 2. Downloading the .deb file
	// 3. Saving to the specified output path
	// 4. Showing progress during download

	return nil
}

func runSearch(source, searchTerm, format string) error {
	log.Info().Msgf("Searching for '%s' in: %s", searchTerm, source)
	log.Info().Msgf("Format: %s", format)

	// TODO: Implement package search
	// This would involve:
	// 1. Parsing package names and descriptions
	// 2. Filtering by search term
	// 3. Formatting results according to --format flag

	return nil
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:     os.Stderr,
		NoColor: false,
	})
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msgf("%v", err)
	}
}
