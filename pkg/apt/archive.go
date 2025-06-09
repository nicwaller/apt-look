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

func Open(source sources.Entry) (*Repository, error) {
	var err error
	var tpt apttransport.Transport

	scheme := source.ArchiveRoot.Scheme
	tpt, err = apttransport.DefaultRegistry.Select(scheme)
	if err != nil {
		return nil, fmt.Errorf("unsupported transport %q: %w", scheme, err)
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

	// Should we call Update() here? I'm undecided.

	r := &Repository{
		transport:     tpt,
		archiveRoot:   source.ArchiveRoot,
		distRoot:      distRoot,
		release:       nil,
		components:    slices.Clone(source.Components),
		architectures: detectDebianArch(), // TODO: allow override with --arch flag
	}

	return r, nil
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

	if r.components == nil {
		return r.release.GetAvailableFiles()
	}

	var files []deb822.FileInfo
	for _, fi := range r.release.GetAvailableFiles() {
		if r.components != nil {
			if !slices.Contains(r.components, fi.Component) {
				continue
			}
		}
		if r.architectures != nil {
			if !slices.Contains(r.architectures, fi.Architecture) {
				continue
			}
		}
		files = append(files, fi)
	}

	return files
}
