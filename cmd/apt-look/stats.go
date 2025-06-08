package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nicwaller/apt-look/pkg/apt/sources"
	"github.com/nicwaller/apt-look/pkg/apttransport"
	"github.com/nicwaller/apt-look/pkg/deb822"
)

func runStats(sources []sources.Entry, format string) error {
	if len(sources) != 1 {
		return fmt.Errorf("expected 1 source, got %d", len(sources))
	}
	source := sources[0]
	log.Info().Msgf("Getting statistics for: %s", source.String())

	if !source.Enabled {
		return fmt.Errorf("source is disabled")
	}

	// Calculate statistics
	stats, registry, err := calculateRepositoryStats(source)
	if err != nil {
		return fmt.Errorf("failed to calculate statistics: %w", err)
	}

	// Format and output results
	err = outputStats(source, stats, format)
	if err != nil {
		return err
	}

	// Display cache statistics
	hits, misses, hitRatio := registry.GetCacheStats()
	if hits > 0 || misses > 0 {
		log.Info().
			Int64("cache_hits", hits).
			Int64("cache_misses", misses).
			Float64("hit_ratio", hitRatio).
			Msgf("Cache performance: %d hits, %d misses (%.1f%% hit ratio)",
				hits, misses, hitRatio*100)
	}

	return nil
}

// RepositoryStats holds statistics about a repository
// TODO: should size stats be calculated per-architecture and per-component? I think yes.
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
		Total          int            `json:"total"`
		TotalSize      int64          `json:"total_size_bytes"`
		TotalSizeMB    int64          `json:"total_size_mb"`
		ByArchitecture map[string]int `json:"by_architecture"`
		ByComponent    map[string]int `json:"by_component"`
		BySection      map[string]int `json:"by_section"`
		ByPriority     map[string]int `json:"by_priority"`
	} `json:"packages"`
}

func calculateRepositoryStats(source sources.Entry) (*RepositoryStats, *apttransport.Registry, error) {
	stats := &RepositoryStats{}

	// Use the transport registry with caching
	registry := loadTransports()

	// Fetch Release file
	releaseURL := strings.TrimSuffix(source.URI, "/") + "/dists/" + source.Distribution + "/Release"
	parsedURL, err := url.Parse(releaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse Release URL %s: %w", releaseURL, err)
	}

	ctx := context.Background()
	resp, err := registry.Acquire(ctx, &apttransport.AcquireRequest{
		URI:     parsedURL,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Release file from %s: %w", releaseURL, err)
	}
	defer resp.Content.Close()

	release, err := deb822.ParseRelease(resp.Content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse Release file: %w", err)
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

	// Fetch and parse Packages files based on what's actually available in the Release file
	for _, component := range source.Components {
		if component == "" {
			continue
		}

		for _, arch := range release.Architectures {
			if arch == "source" {
				continue // Skip source architecture for binary package stats
			}

			// Check if Packages files exist for this component/architecture combination
			packagesFiles := release.GetPackagesFiles(component, arch)
			if len(packagesFiles) == 0 {
				log.Debug().Msgf("No Packages files found for %s/%s", component, arch)
				continue
			}

			// Process the first available Packages file (prefer compressed)
			var fileToProcess *deb822.FileInfo
			for _, file := range packagesFiles {
				if file.Compressed {
					fileToProcess = &file
					break
				}
			}
			if fileToProcess == nil {
				fileToProcess = &packagesFiles[0] // Use first available if no compressed version
			}

			err := processPackagesFileFromRelease(registry, source, *fileToProcess, stats)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to process packages for %s/%s", component, arch)
				continue
			}
		}
	}

	// Calculate derived statistics
	stats.Packages.TotalSizeMB = stats.Packages.TotalSize / (1024 * 1024)

	return stats, registry, nil
}

func outputStats(source sources.Entry, stats *RepositoryStats, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)

	case "tsv":
		return outputStatsTSV(stats)

	case "prom":
		return outputStatsPrometheus(source, stats)

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

func formatPrometheusMetric(name string, labels map[string]string, value float64) string {
	var sb strings.Builder
	sb.WriteString(name)
	if labels != nil && len(labels) > 0 {
		sb.WriteRune('{')
		parts := make([]string, 0, len(labels))
		for k, v := range labels {
			parts = append(parts, fmt.Sprintf(`%s=%q`, k, v))
		}
		sb.WriteString(strings.Join(parts, ","))
		sb.WriteRune('}')
	}
	sb.WriteRune(' ')
	sb.WriteString(fmt.Sprintf("%f", value))
	return sb.String()
}

func outputStatsPrometheus(source sources.Entry, stats *RepositoryStats) error {
	purl, err := url.Parse(source.URI)
	if err != nil {
		return fmt.Errorf("failed to parse source.URI: %w", err)
	}

	labels := map[string]string{
		"host":         purl.Host,
		"path":         purl.Path,
		"distribution": source.Distribution,
		"origin":       stats.Repository.Origin,
		"label":        stats.Repository.Label,
		"suite":        stats.Repository.Suite,
	}

	// TODO: HELP and TYPE lines
	//# HELP http_requests_total The total number of HTTP requests
	//# TYPE http_requests_total counter
	// maybe I should import a real prom module?

	var metrics []string
	labels["arch"] = "combined"
	metrics = append(metrics, formatPrometheusMetric("apt_repo_total_bytes", labels,
		float64(stats.Packages.TotalSize)))
	metrics = append(metrics, formatPrometheusMetric("apt_repo_total_packages", labels,
		float64(stats.Packages.Total)))

	for arch, pkgCount := range stats.Packages.ByArchitecture {
		labels["arch"] = arch
		metrics = append(metrics, formatPrometheusMetric("apt_repo_total_packages", labels,
			float64(pkgCount)))
	}
	delete(labels, "arch")

	for arch, pkgCount := range stats.Packages.ByArchitecture {
		labels["component"] = arch
		metrics = append(metrics, formatPrometheusMetric("apt_repo_total_packages", labels,
			float64(pkgCount)))
	}
	delete(labels, "component")

	for _, metric := range metrics {
		_, _ = os.Stdout.WriteString(metric + "\n")
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
