package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	apttransport2 "github.com/nicwaller/apt-look/pkg/apt/apttransport"
	"github.com/nicwaller/apt-look/pkg/apt/sources"
)

func runStats(sources []sources.Entry, format string) error {
	if len(sources) != 1 {
		return fmt.Errorf("expected 1 source, got %d", len(sources))
	}
	source := sources[0]
	log.Info().Msgf("Getting statistics for: %v", source)

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

func calculateRepositoryStats(source sources.Entry) (*RepositoryStats, *apttransport2.Registry, error) {
	// TODO: implement this again
	panic("not yet implemented")
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
	labels := map[string]string{
		"host":         source.ArchiveRoot.Host,
		"path":         source.ArchiveRoot.Path,
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
