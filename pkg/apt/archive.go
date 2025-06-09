package apt

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nicwaller/apt-look/pkg/apt/apttransport"
	"github.com/nicwaller/apt-look/pkg/apt/sources"
	"github.com/nicwaller/apt-look/pkg/deb822"
)

// https://www.debian.org/doc/manuals/debian-reference/ch02.en.html#_debian_archive_basics
type Repository struct {
	transport   apttransport.Transport
	archiveRoot *url.URL
	distRoot    *url.URL

	// stuff we get from apt-get update
	release  *deb822.Release // nil until Update
	packages []deb822.Package
	// file filtering
	components    []string
	architectures []string
}

// curiously, a single source line with multiple components can yield
// multiple repositories, each with their own Release file

// MountOptions contains configuration options for mounting a repository
type MountOptions struct {
	Architectures []string
	Components    []string
	Transport     apttransport.Transport
	Registry      *apttransport.Registry
}

// MountOption is a functional option for configuring Mount behavior
type MountOption func(*MountOptions)

// WithArchitectures sets the target architectures for the repository
func WithArchitectures(architectures ...string) MountOption {
	return func(opts *MountOptions) {
		opts.Architectures = architectures
	}
}

// WithTransport sets a specific transport to use for the repository
func WithTransport(transport apttransport.Transport) MountOption {
	return func(opts *MountOptions) {
		opts.Transport = transport
	}
}

// WithRegistry sets a specific transport registry to use for the repository
func WithRegistry(registry *apttransport.Registry) MountOption {
	return func(opts *MountOptions) {
		opts.Registry = registry
	}
}

func Mount(source sources.Entry, optFns ...MountOption) (*Repository, error) {
	opts := &MountOptions{}
	for _, fn := range optFns {
		fn(opts)
	}

	// Use provided architectures or detect from system
	architectures := opts.Architectures
	if len(architectures) == 0 {
		architectures = detectDebianArch()
	}

	// Use provided transport, or select from registry, or use default registry
	var err error
	var tpt apttransport.Transport

	if opts.Transport != nil {
		tpt = opts.Transport
	} else {
		scheme := source.ArchiveRoot.Scheme
		registry := opts.Registry
		if registry == nil {
			registry = apttransport.DefaultRegistry
		}
		tpt, err = registry.Select(scheme)
		if err != nil {
			return nil, fmt.Errorf("unsupported transport %q: %w", scheme, err)
		}
	}

	var distRoot *url.URL
	if slices.Contains([]string{".", "/"}, source.Distribution) {
		// aha! this is a rare case called "Flat Repository Format" described here:
		// https://wiki.debian.org/DebianRepository/Format
		// I've only seen it once in the wild:
		// deb https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /
		distRoot = source.ArchiveRoot.JoinPath(source.Distribution)
	} else {
		// this is the common case
		distRoot = source.ArchiveRoot.JoinPath("dists", source.Distribution)
	}

	// Fetch the Release file as part of mounting to validate the repository exists
	ctx := context.Background()
	resp, err := tpt.Acquire(ctx, &apttransport.AcquireRequest{
		// TODO: add support for InRelease file
		URI:     distRoot.JoinPath("Release"),
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Release file: %w", err)
	}

	// Parse the Release file
	release, err := deb822.ParseRelease(resp.Content)
	resp.Content.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Release file: %w", err)
	}

	r := &Repository{
		transport:     tpt,
		archiveRoot:   source.ArchiveRoot,
		distRoot:      distRoot,
		release:       release, // Now populated during mount
		components:    slices.Clone(source.Components),
		architectures: architectures,
	}

	return r, nil
}

// Release returns the Release metadata for the repository.
// The Release file is fetched during mounting, so this should always return a valid result.
func (r *Repository) Release() *deb822.Release {
	return r.release
}

// WithComponents sets the components for MountURL (adds to MountOptions)
func WithComponents(components ...string) MountOption {
	return func(opts *MountOptions) {
		opts.Components = components
	}
}

// MountURL is a convenience function that creates a Repository from basic parameters.
// It creates a "deb" type source entry with the specified options.
func MountURL(archiveRoot *url.URL, distribution string, optFns ...MountOption) (*Repository, error) {
	opts := &MountOptions{}
	for _, fn := range optFns {
		fn(opts)
	}

	components := opts.Components
	if len(components) == 0 {
		components = []string{"main"}
	}

	entry := sources.Entry{
		Type:         sources.SourceTypeDeb,
		ArchiveRoot:  archiveRoot,
		Distribution: distribution,
		Components:   components,
		Options:      make(map[string]string),
	}

	return Mount(entry, optFns...)
}

// Discover attempts to find valid distributions and components in an APT repository
// by making educated guesses based on common patterns. It tries to balance making
// fewer requests while returning multiple results when possible.
//
// The input can be either an archive root URL (e.g., "https://example.com/ubuntu")
// or a distribution root URL (e.g., "https://example.com/ubuntu/dists/jammy").
// If a distribution URL is detected, it will be used directly and the archive root
// will be inferred.
func Discover(archiveRoot string) ([]sources.Entry, error) {
	repoURL, err := url.Parse(archiveRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid archive root URL: %w", err)
	}

	// Check if this looks like a distribution root URL (contains /dists/)
	if distEntry, actualArchiveRoot := tryParseDistRoot(repoURL); distEntry != nil {
		// This is a distribution URL, try to mount it directly
		repo, err := Mount(*distEntry)
		if err != nil {
			return nil, fmt.Errorf("failed to mount distribution URL: %w", err)
		}

		// Try to fetch the Release file to validate and get components
		ctx := context.Background()
		release, err := repo.Update(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Release file from distribution URL: %w", err)
		}

		// Use components from Release file if available, otherwise use defaults
		components := distEntry.Components
		if len(release.Components) > 0 {
			components = release.Components
		}

		// Create the validated entry with the actual archive root
		entry := sources.Entry{
			Type:         sources.SourceTypeDeb,
			ArchiveRoot:  actualArchiveRoot,
			Distribution: distEntry.Distribution,
			Components:   components,
			Options:      make(map[string]string),
		}

		return []sources.Entry{entry}, nil
	}

	// This appears to be an archive root URL, proceed with discovery
	// Get transport for this URL scheme
	tpt, err := apttransport.DefaultRegistry.Select(repoURL.Scheme)
	if err != nil {
		return nil, fmt.Errorf("unsupported transport %q: %w", repoURL.Scheme, err)
	}

	ctx := context.Background()
	var foundEntries []sources.Entry

	// Define candidate distributions to try, ordered by likelihood
	candidates := getDistributionCandidates(archiveRoot)

	for _, candidate := range candidates {
		// Try to fetch Release file for this distribution
		distURL := repoURL.JoinPath("dists", candidate.distribution)
		releaseURL := distURL.JoinPath("Release")

		resp, err := tpt.Acquire(ctx, &apttransport.AcquireRequest{
			URI:     releaseURL,
			Timeout: 5 * time.Second, // Add timeout to avoid hanging tests
		})
		if err != nil {
			// Release file doesn't exist for this distribution, skip it
			continue
		}

		// Parse the Release file to get actual components and architectures
		release, err := deb822.ParseRelease(resp.Content)
		resp.Content.Close()
		if err != nil {
			// Invalid Release file, skip this distribution
			continue
		}

		// Create entries based on the Release file content
		components := candidate.components
		if len(release.Components) > 0 {
			// Use actual components from Release file if available
			components = release.Components
		}

		entry := sources.Entry{
			Type:         sources.SourceTypeDeb,
			ArchiveRoot:  repoURL,
			Distribution: candidate.distribution,
			Components:   components,
			Options:      make(map[string]string),
		}

		foundEntries = append(foundEntries, entry)

		// If we've found entries and this looks like a major distribution,
		// we might want to stop here to avoid making too many requests
		if len(foundEntries) >= 3 {
			break
		}
	}

	if len(foundEntries) == 0 {
		return nil, fmt.Errorf("no valid distributions found in repository")
	}

	return foundEntries, nil
}

// distributionCandidate represents a guess about what might be in a repository
type distributionCandidate struct {
	distribution string
	components   []string
	priority     int // higher = try first
}

// getDistributionCandidates returns likely distribution/component combinations
// based on the repository URL and common patterns
func getDistributionCandidates(archiveRoot string) []distributionCandidate {
	var candidates []distributionCandidate

	// Analyze the URL for hints about what kind of repository this might be
	lowerURL := strings.ToLower(archiveRoot)

	// Third-party repository patterns (higher priority since they're more specific)
	switch {
	case strings.Contains(lowerURL, "docker"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"stable"}, 100})
		candidates = append(candidates, distributionCandidate{"/", []string{}, 90}) // Flat repository format
	case strings.Contains(lowerURL, "kubernetes") || strings.Contains(lowerURL, "k8s"):
		candidates = append(candidates, distributionCandidate{"/", []string{}, 100}) // k8s uses flat format
		candidates = append(candidates, distributionCandidate{"kubernetes-1.28", []string{"main"}, 90})
		candidates = append(candidates, distributionCandidate{"kubernetes-1.29", []string{"main"}, 85})
	case strings.Contains(lowerURL, "microsoft"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 100})
		candidates = append(candidates, distributionCandidate{"prod", []string{"main"}, 90})
	case strings.Contains(lowerURL, "google") || strings.Contains(lowerURL, "chrome"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 100})
	case strings.Contains(lowerURL, "spotify"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"non-free"}, 100})
	case strings.Contains(lowerURL, "signal"):
		candidates = append(candidates, distributionCandidate{"xenial", []string{"main"}, 100})
	case strings.Contains(lowerURL, "brave"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 100})
	case strings.Contains(lowerURL, "hashicorp"):
		candidates = append(candidates, distributionCandidate{"any", []string{"main"}, 100})
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 90})
	case strings.Contains(lowerURL, "postgresql"):
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 100})
		candidates = append(candidates, distributionCandidate{"pgdg", []string{"main"}, 90})
	case strings.Contains(lowerURL, "node"):
		candidates = append(candidates, distributionCandidate{"nodistro", []string{"main"}, 100})
		candidates = append(candidates, distributionCandidate{"stable", []string{"main"}, 90})
	}

	// Official Ubuntu/Debian patterns (medium priority)
	if strings.Contains(lowerURL, "ubuntu") {
		ubuntuReleases := []string{"noble", "jammy", "focal", "bionic", "xenial"}
		for i, release := range ubuntuReleases {
			candidates = append(candidates, distributionCandidate{
				release, []string{"main", "restricted", "universe", "multiverse"}, 80 - i,
			})
		}
	}

	if strings.Contains(lowerURL, "debian") {
		debianReleases := []string{"bookworm", "bullseye", "buster", "stretch"}
		for i, release := range debianReleases {
			candidates = append(candidates, distributionCandidate{
				release, []string{"main", "contrib", "non-free"}, 70 - i,
			})
		}
		// Also try testing/unstable
		candidates = append(candidates, distributionCandidate{"testing", []string{"main", "contrib", "non-free"}, 65})
		candidates = append(candidates, distributionCandidate{"unstable", []string{"main", "contrib", "non-free"}, 60})
	}

	// Generic fallbacks (lower priority)
	genericCandidates := []distributionCandidate{
		{"stable", []string{"main"}, 50},
		{"release", []string{"main"}, 45},
		{"latest", []string{"main"}, 40},
		{"current", []string{"main"}, 35},
		{"/", []string{}, 30}, // Flat repository format
		{".", []string{}, 25}, // Alternative flat format
		{"main", []string{"main"}, 20},
		{"stable", []string{"stable"}, 15},
		{"prod", []string{"main"}, 10},
		{"production", []string{"main"}, 5},
	}

	candidates = append(candidates, genericCandidates...)

	// Sort by priority (highest first)
	slices.SortFunc(candidates, func(a, b distributionCandidate) int {
		return b.priority - a.priority
	})

	return candidates
}

// tryParseDistRoot attempts to parse a URL that might be a distribution root
// (e.g., https://example.com/ubuntu/dists/jammy). If successful, it returns
// a sources.Entry for the distribution and the inferred archive root URL.
func tryParseDistRoot(distURL *url.URL) (*sources.Entry, *url.URL) {
	path := strings.TrimSuffix(distURL.Path, "/")

	// Look for /dists/ pattern in the URL path
	distsIndex := strings.LastIndex(path, "/dists/")
	if distsIndex == -1 {
		return nil, nil
	}

	// Extract archive root path (everything before /dists/)
	archiveRootPath := path[:distsIndex]
	if archiveRootPath == "" {
		archiveRootPath = "/"
	}

	// Extract distribution name (everything after /dists/)
	distPath := path[distsIndex+7:] // +7 to skip "/dists/"
	if distPath == "" {
		return nil, nil
	}

	// Parse distribution path - could be just "jammy" or "jammy/main" etc.
	distParts := strings.Split(distPath, "/")
	distribution := distParts[0]

	// Handle flat repository format
	if distribution == "" || distribution == "." {
		distribution = "/"
	}

	// Create archive root URL
	archiveRoot := &url.URL{
		Scheme:   distURL.Scheme,
		Host:     distURL.Host,
		Path:     archiveRootPath,
		RawQuery: distURL.RawQuery,
		Fragment: distURL.Fragment,
	}

	// Create sources entry with default main component
	entry := &sources.Entry{
		Type:         sources.SourceTypeDeb,
		ArchiveRoot:  archiveRoot,
		Distribution: distribution,
		Components:   []string{"main"}, // Default, will be updated from Release file
		Options:      make(map[string]string),
	}

	return entry, archiveRoot
}

func detectDebianArch() []string {
	switch runtime.GOARCH {
	case "amd64":
		return []string{"amd64", "i386"}
	case "386":
		return []string{"i386"}
	case "arm64":
		return []string{"arm64"}
	case "arm":
		return []string{"arm", "armhf"}
	default:
		// whatever, just use all of them
		return nil
	}
}

func (r *Repository) DistributionRoot() *url.URL {
	return r.distRoot
}

func (r *Repository) Transport() apttransport.Transport {
	return r.transport
}

func (r *Repository) Update(ctx context.Context) (*deb822.Release, error) {
	resp, err := r.transport.Acquire(ctx, &apttransport.AcquireRequest{
		// TODO: add support for InRelease file
		URI: r.distRoot.JoinPath("Release"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Release file: %w", err)
	}

	// TODO: protect this with a mutex?
	r.release, err = deb822.ParseRelease(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Release file: %w", err)
	}

	return r.release, nil
}

// THINK: should fetch happen at the Transport layer?
func (r *Repository) Fetch(ctx context.Context, loc *url.URL) (io.Reader, *apttransport.AcquireResponse, error) {
	if loc == nil {
		return nil, nil, errors.New("invalid URL")
	}
	acr, err := r.transport.Acquire(ctx, &apttransport.AcquireRequest{
		URI: loc,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch repository: %w", err)
	}
	rdr := acr.Content

	// this is where we handle decompression
	switch filepath.Ext(loc.Path) {
	// TODO: support more compression types
	case ".gz":
		rdr, err = gzip.NewReader(acr.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read gzipped file: %w", err)
		}
	default:
		// don't change the rdr
	}

	return rdr, acr, err
}

func (r *Repository) Packages(ctx context.Context) iter.Seq2[*deb822.Package, error] {
	return func(yield func(*deb822.Package, error) bool) {
		if r.release == nil {
			_, err := r.Update(ctx)
			if err != nil {
				yield(nil, err)
				return
			}
		}

		for _, fi := range r.indexes() {
			if fi.Type == "Packages" {
				rdr, _, err := r.Fetch(ctx, r.distRoot.JoinPath(fi.Path))
				if err != nil {
					yield(nil, fmt.Errorf("failed to fetch Packages file %s: %w", fi.Path, err))
					return
				}
				for pkg, err := range deb822.ParsePackages(rdr) {
					if err != nil {
						yield(nil, fmt.Errorf("failed to parse Packages file %s: %w", fi.Path, err))
						return
					}
					yield(pkg, nil)
				}
			}
		}
	}
}

func (r *Repository) indexes() []deb822.FileInfo {
	if r.release == nil {
		panic("release not initialized")
	}

	if len(r.components) == 0 {
		return r.release.GetAvailableFiles()
	}

	var files []deb822.FileInfo
	for _, fi := range r.release.GetAvailableFiles() {
		if !slices.Contains(r.components, fi.Component) {
			continue
		}
		if r.architectures != nil && len(r.architectures) > 0 {
			if !slices.Contains(r.architectures, fi.Architecture) {
				continue
			}
		}
		files = append(files, fi)
	}

	return files
}

// GetAvailableArchitectures returns all architectures available for the specified components
func (r *Repository) GetAvailableArchitectures(components []string) []string {
	if r.release == nil {
		return nil
	}

	archSet := make(map[string]bool)
	for _, fi := range r.release.GetAvailableFiles() {
		if fi.Type == "Packages" {
			// If components are specified, filter by them
			if len(components) > 0 {
				if !slices.Contains(components, fi.Component) {
					continue
				}
			}
			archSet[fi.Architecture] = true
		}
	}

	var archs []string
	for arch := range archSet {
		archs = append(archs, arch)
	}
	slices.Sort(archs)
	return archs
}
