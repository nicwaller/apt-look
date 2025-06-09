package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/nicwaller/apt-look/pkg/apt"
	apttransport2 "github.com/nicwaller/apt-look/pkg/apt/apttransport"
	"github.com/nicwaller/apt-look/pkg/apt/sources"
)

var options struct {
	format string
	output string
	debug  bool
	arch   []string
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

		// TODO: how to share this among all subcommands?
		// Parse source input
		sources, err := parseSourceInput(source)
		if err != nil {
			return fmt.Errorf("failed to parse source: %w", err)
		}

		return runStats(sources, options.format)
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

// Latest command
var latestCmd = &cobra.Command{
	Use:   "latest <source>",
	Short: "Show the latest version of each package",
	Long: `Show information about the highest version available for each package.
Packages are grouped by (name, architecture) tuple and only the latest version is shown.`,
	Args: cobra.ExactArgs(1),
	Example: `  apt-look latest "deb http://archive.ubuntu.com/ubuntu/ jammy main"
  apt-look latest /etc/apt/sources.list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		return runLatest(source, options.format)
	},
}

// Check command
var checkCmd = &cobra.Command{
	Use:   "check <source>",
	Short: "Verify repository integrity",
	Long: `Perform integrity checks on APT repositories by verifying that files listed
in Release file hash sections actually exist on the server. Reports missing files,
broken references, and other repository integrity issues.`,
	Args: cobra.ExactArgs(1),
	Example: `  apt-look check "deb http://archive.ubuntu.com/ubuntu/ jammy main"
  apt-look check /etc/apt/sources.list --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		return runCheck(source, options.format)
	},
}

// Purge-cache command
var purgeCacheCmd = &cobra.Command{
	Use:   "purge-cache",
	Short: "Remove all cached repository files",
	Long: `Remove all cached repository files from the apt-look cache directory.
This forces fresh downloads of all repository metadata on subsequent operations.`,
	Args:    cobra.NoArgs,
	Example: `  apt-look purge-cache`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPurgeCache()
	},
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVarP(&options.format, "format", "f", "text",
		"Output format (text, json, tsv, raw)")
	rootCmd.PersistentFlags().BoolVar(&options.debug, "debug", false,
		"Enable debug logging")
	rootCmd.PersistentFlags().StringSliceVar(&options.arch, "arch", nil,
		"Target architectures (e.g., amd64,arm64). Defaults to current system architecture.")

	// Command-specific flags
	downloadCmd.Flags().StringVarP(&options.output, "output", "o", ".",
		"Output directory for downloaded packages")

	// Add validation for format flag
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Set log level based on debug flag
		if options.debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}

		validFormats := []string{"text", "json", "tsv", "prom", "raw"}
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
	rootCmd.AddCommand(latestCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(purgeCacheCmd)
}

func loadTransports() *apttransport2.Registry {
	// Configure caching (enabled by default)
	cacheConfig := apttransport2.CacheConfig{
		Disabled: false, // TODO: add --no-cache flag
	}

	r := apttransport2.NewRegistryWithCache(cacheConfig)
	r.Register(apttransport2.NewHTTPTransport())
	r.Register(apttransport2.NewFileTransport())
	// TODO: on Debian systems, register transports for all available plugins
	return r
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

func parseSourceInput(source string) ([]sources.Entry, error) {
	// Check if it's a file path
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		file, err := os.Open(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open sources file: %w", err)
		}
		defer file.Close()

		sourcesList, err := sources.ParseSourcesList(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sources file: %w", err)
		}

		return sourcesList, nil
	}

	// Check if it's a valid URL
	if parsedURL, err := url.Parse(source); err == nil && parsedURL.Scheme != "" && parsedURL.Host != "" {
		// Use apt.Discover to find available distributions and components
		entries, err := apt.Discover(source)
		if err != nil {
			return nil, fmt.Errorf("failed to discover repository structure: %w", err)
		}
		return entries, nil
	}

	// Parse as single source line
	entry, err := sources.ParseSourceLine(source, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source line: %w", err)
	}

	return []sources.Entry{*entry}, nil
}

// buildMountOptions creates mount options from global flags
func buildMountOptions() []apt.MountOption {
	var opts []apt.MountOption
	if len(options.arch) > 0 {
		opts = append(opts, apt.WithArchitectures(options.arch...))
	}
	return opts
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

func runPurgeCache() error {
	log.Info().Msg("Purging apt-look cache")

	// Load transport registry to access cache functionality
	registry := loadTransports()

	// Purge the cache
	err := registry.PurgeCache()
	if err != nil {
		return fmt.Errorf("failed to purge cache: %w", err)
	}

	log.Info().Msg("Cache purged successfully")
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
