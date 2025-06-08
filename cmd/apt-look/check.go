package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nicwaller/apt-look/pkg/apt/sources"
	"github.com/nicwaller/apt-look/pkg/apttransport"
	"github.com/nicwaller/apt-look/pkg/deb822"
)

// CheckResult represents the results of a repository integrity check
type CheckResult struct {
	Repository struct {
		Origin     string    `json:"origin,omitempty"`
		Label      string    `json:"label,omitempty"`
		Suite      string    `json:"suite,omitempty"`
		Codename   string    `json:"codename,omitempty"`
		Date       time.Time `json:"date"`
		BaseURL    string    `json:"base_url"`
		Components []string  `json:"components"`
	} `json:"repository"`

	Summary struct {
		TotalFiles      int `json:"total_files"`
		ExistingFiles   int `json:"existing_files"`
		MissingFiles    int `json:"missing_files"`
		NetworkErrors   int `json:"network_errors"`
		IntegrityIssues int `json:"integrity_issues"`
	} `json:"summary"`

	MissingFiles    []FileCheckResult `json:"missing_files,omitempty"`
	NetworkErrors   []FileCheckResult `json:"network_errors,omitempty"`
	IntegrityIssues []FileCheckResult `json:"integrity_issues,omitempty"`
}

// FileCheckResult represents the result of checking a single file
type FileCheckResult struct {
	deb822.FileInfo
	URL         string `json:"url"`
	StatusCode  int    `json:"status_code,omitempty"`
	Error       string `json:"error,omitempty"`
	ActualSize  int64  `json:"actual_size,omitempty"`
	SizeMatches bool   `json:"size_matches,omitempty"`
}

func runCheck(sourceStr, format string) error {
	// Parse source
	sources, err := parseSourceInput(sourceStr)
	if err != nil {
		return fmt.Errorf("failed to parse sources: %w", err)
	}

	if len(sources) != 1 {
		return fmt.Errorf("expected 1 source, got %d", len(sources))
	}
	source := sources[0]
	log.Info().Msgf("Checking repository integrity: %s", source.String())

	if !source.Enabled {
		return fmt.Errorf("source is disabled")
	}

	// Perform the integrity check
	result, registry, err := performIntegrityCheck(source)
	if err != nil {
		return fmt.Errorf("failed to perform integrity check: %w", err)
	}

	// Format and output results
	err = outputCheckResults(result, format)
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

func performIntegrityCheck(source sources.SourceEntry) (*CheckResult, *apttransport.Registry, error) {
	result := &CheckResult{}

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

	// Fill repository info
	result.Repository.Origin = release.Origin
	result.Repository.Label = release.Label
	result.Repository.Suite = release.Suite
	result.Repository.Codename = release.Codename
	result.Repository.Date = release.Date
	result.Repository.BaseURL = strings.TrimSuffix(source.URI, "/") + "/dists/" + source.Distribution
	result.Repository.Components = source.Components

	// Get all files from Release metadata
	allFiles := release.GetAvailableFiles()
	result.Summary.TotalFiles = len(allFiles)

	log.Info().Msgf("Checking %d files listed in Release metadata", len(allFiles))

	// Check each file
	for _, fileInfo := range allFiles {
		checkResult := checkFile(registry, result.Repository.BaseURL, fileInfo)

		switch {
		case checkResult.StatusCode == http.StatusNotFound:
			result.MissingFiles = append(result.MissingFiles, checkResult)
			result.Summary.MissingFiles++
		case checkResult.Error != "":
			result.NetworkErrors = append(result.NetworkErrors, checkResult)
			result.Summary.NetworkErrors++
		case !checkResult.SizeMatches:
			result.IntegrityIssues = append(result.IntegrityIssues, checkResult)
			result.Summary.IntegrityIssues++
		default:
			result.Summary.ExistingFiles++
		}
	}

	return result, registry, nil
}

func checkFile(registry *apttransport.Registry, baseURL string, fileInfo deb822.FileInfo) FileCheckResult {
	checkResult := FileCheckResult{
		FileInfo: fileInfo,
		URL:      baseURL + "/" + fileInfo.Path,
	}

	parsedURL, err := url.Parse(checkResult.URL)
	if err != nil {
		checkResult.Error = fmt.Sprintf("invalid URL: %v", err)
		return checkResult
	}

	// Use GET request to check existence and get size
	ctx := context.Background()
	req := &apttransport.AcquireRequest{
		URI:     parsedURL,
		Timeout: 10 * time.Second,
	}

	resp, err := registry.Acquire(ctx, req)
	if err != nil {
		// Try to extract status code from HTTP errors
		if strings.Contains(err.Error(), "HTTP 404") {
			checkResult.StatusCode = http.StatusNotFound
		} else {
			checkResult.Error = err.Error()
		}
		return checkResult
	}
	defer resp.Content.Close()

	checkResult.StatusCode = http.StatusOK
	checkResult.ActualSize = resp.Size
	checkResult.SizeMatches = checkResult.ActualSize == fileInfo.Size

	return checkResult
}

func outputCheckResults(result *CheckResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case "tsv":
		return outputCheckResultsTSV(result)

	case "text":
		fallthrough
	default:
		return outputCheckResultsText(result)
	}
}

func outputCheckResultsText(result *CheckResult) error {
	fmt.Printf("Repository Integrity Check\n")
	fmt.Printf("=========================\n\n")

	// Repository information
	fmt.Printf("Repository Information:\n")
	if result.Repository.Origin != "" {
		fmt.Printf("  Origin: %s\n", result.Repository.Origin)
	}
	if result.Repository.Label != "" {
		fmt.Printf("  Label: %s\n", result.Repository.Label)
	}
	if result.Repository.Suite != "" {
		fmt.Printf("  Suite: %s\n", result.Repository.Suite)
	}
	if result.Repository.Codename != "" {
		fmt.Printf("  Codename: %s\n", result.Repository.Codename)
	}
	fmt.Printf("  Date: %s\n", result.Repository.Date.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("  Base URL: %s\n", result.Repository.BaseURL)
	fmt.Printf("  Components: %s\n", strings.Join(result.Repository.Components, ", "))

	// Summary
	fmt.Printf("\nIntegrity Summary:\n")
	fmt.Printf("  Total Files: %d\n", result.Summary.TotalFiles)
	fmt.Printf("  Existing Files: %d\n", result.Summary.ExistingFiles)
	fmt.Printf("  Missing Files: %d\n", result.Summary.MissingFiles)
	fmt.Printf("  Network Errors: %d\n", result.Summary.NetworkErrors)
	fmt.Printf("  Integrity Issues: %d\n", result.Summary.IntegrityIssues)

	// Missing files
	if len(result.MissingFiles) > 0 {
		fmt.Printf("\nMissing Files:\n")
		for _, file := range result.MissingFiles {
			fmt.Printf("  - %s (type: %s, component: %s, arch: %s)\n",
				file.Path, file.Type, file.Component, file.Architecture)
		}
	}

	// Network errors
	if len(result.NetworkErrors) > 0 {
		fmt.Printf("\nNetwork Errors:\n")
		for _, file := range result.NetworkErrors {
			fmt.Printf("  - %s: %s\n", file.Path, file.Error)
		}
	}

	// Integrity issues
	if len(result.IntegrityIssues) > 0 {
		fmt.Printf("\nIntegrity Issues:\n")
		for _, file := range result.IntegrityIssues {
			fmt.Printf("  - %s: size mismatch (expected: %d, actual: %d)\n",
				file.Path, file.Size, file.ActualSize)
		}
	}

	return nil
}

func outputCheckResultsTSV(result *CheckResult) error {
	fmt.Printf("field\tvalue\n")
	fmt.Printf("origin\t%s\n", result.Repository.Origin)
	fmt.Printf("label\t%s\n", result.Repository.Label)
	fmt.Printf("suite\t%s\n", result.Repository.Suite)
	fmt.Printf("codename\t%s\n", result.Repository.Codename)
	fmt.Printf("date\t%s\n", result.Repository.Date.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("base_url\t%s\n", result.Repository.BaseURL)
	fmt.Printf("components\t%s\n", strings.Join(result.Repository.Components, ","))
	fmt.Printf("total_files\t%d\n", result.Summary.TotalFiles)
	fmt.Printf("existing_files\t%d\n", result.Summary.ExistingFiles)
	fmt.Printf("missing_files\t%d\n", result.Summary.MissingFiles)
	fmt.Printf("network_errors\t%d\n", result.Summary.NetworkErrors)
	fmt.Printf("integrity_issues\t%d\n", result.Summary.IntegrityIssues)

	return nil
}
