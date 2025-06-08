# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation

See DESIGN.md for complete project specification, detailed command examples, and output format samples.

## Project Overview

`apt-look` is a command-line tool written in Go for exploring remote APT repositories without requiring system configuration. It follows the UNIX philosophy of "do one thing and do it well" - serving as a repository lens that fetches and presents APT repository data in a structured, scriptable way.

## Architecture

This is a Go-based CLI application with the following planned components:
- HTTP client with proper User-Agent and caching
- Debian control file parser for Release/Packages files  
- Command-line interface using subcommands
- Output formatters for text/json/tsv/raw formats
- Source list parser for handling APT configuration files

## Caching Strategy

- Always fetch the latest Release file (no caching)
- Cache repository metadata files locally on disk in `$XDG_CACHE_HOME/apt-look/` (fallback to `~/.cache/apt-look/` if XDG_CACHE_HOME not set)
- **Cached file types**: Packages, Contents, Sources, and Translation files in all supported compression formats (.gz, .bz2, .xz)
- Use content-based naming: cache files are named using the MD5 hash of the plaintext contents
- Cache files should be gzip-compressed, but the name is still based on the plaintext contents.
- The `purge-cache` subcommand purges the apt-look cache
- The `--no-cache` flag disables use of the cache. This can be useful during troubleshooting, such as when troubleshooting a webserver and reviewing request logs. 
- When running on a Debian distribution (including Ubuntu), scan `/var/lib/apt/lists` at startup for any files modified more recently than the newest cache file. Any new files should be incorporated into the apt-look cache.

## GPG Verification

- Always attempt GPG verification of Release files
- Default "best effort" mode: use any keyrings available in standard locations, warn if verification fails but continue
- Strict mode via `--must-verify` flag: verification failures result in error and exit status 1
- `--keyring <path>` flag to specify additional keyring files
- `--keyfile <path>` flag to specify individual key files to trust

## Command Interface

The tool will provide these subcommands:
- `list <source>` - List all packages
- `info <source> <package>` - Show package details  
- `stats <source>` - Repository statistics
- `download <source> <package>` - Download specific package
- `search <source> <term>` - Search packages

Sources can be either full APT source lines or source list files.

## Development Requirements

- Use newest available Go version and packages
- Single binary with minimal dependencies
- Proper error handling for network/parsing issues
- Secure development practices and input validation
- Target western Canada cloud regions and data centres

## Output Formats

The tool should support multiple output formats:
- `text` (default): Human-readable tables with colors if TTY
- `json`: Structured JSON for tools like `jq`
- `tsv`: Tab-separated values for `cut`, `awk`, spreadsheet import  
- `raw`: Original Debian control format (pass-through)

## Release File Processing

The tool uses Release files as the authoritative source for repository metadata and file availability:

- **File Discovery**: Never guess at file paths. Always use the SHA256/SHA1/MD5Sum sections in Release files to determine which Packages files exist
- **Path Structure**: Release file entries follow the pattern `component/binary-architecture/Packages[.gz|.xz]` (e.g., `main/binary-amd64/Packages.gz`)
- **File Metadata**: Each Release file entry includes hash, size, and relative path from the `/dists/{distribution}/` directory
- **Compression Preference**: Prefer compressed versions (.gz, .xz) when multiple formats are available for the same content
- **Error Handling**: Release files may reference files that don't exist on the server for several reasons:
  - Repository synchronization issues between mirrors
  - **Ubuntu Architecture Separation**: Ubuntu maintains separate archives for different architectures (e.g., separate amd64 and arm64 archives), but both archives use Release files that misleadingly claim to support all common architectures. This means hash entries exist for files that don't actually exist on that specific archive.
  - This practice works for typical APT clients configured for a single architecture but breaks comprehensive tools
  - Handle these cases gracefully with warnings rather than silent failures
- **Repository Validation**: The tool validates that only files listed in Release metadata are fetched, preventing unnecessary 404 errors from path guessing

### Implementation Details

- The `deb822.Release` struct provides `GetAvailableFiles()` and `GetPackagesFiles(component, architecture)` methods
- File paths from Release files are used directly to construct download URLs: `{baseURL}/dists/{distribution}/{releasePath}`
- The `FileInfo` struct categorizes files by type (Packages, Sources, Contents), component, architecture, and compression status

### Known Repository Issues

**Ubuntu's Identical Release File Problem**: Ubuntu's official repositories have a serious architectural flaw where multiple architecture-specific archives publish **identical Release files** that falsely claim comprehensive architecture support:

- **archive.ubuntu.com**: Hosts amd64/i386, but Release file claims `amd64 arm64 armhf i386 ppc64el riscv64 s390x`
- **ports.ubuntu.com**: Hosts arm64/armhf/ppc64el/riscv64/s390x, but Release file claims `amd64 arm64 armhf i386 ppc64el riscv64 s390x`
- **Evidence**: `curl -s http://archive.ubuntu.com/ubuntu/dists/jammy/Release` and `curl -s http://ports.ubuntu.com/ubuntu-ports/dists/jammy/Release` return byte-for-byte identical files

This is a **violation of APT repository standards** where Release files should be authoritative manifests of actually available content.

- **Impact**: Any tool that trusts Release metadata (as it should) will encounter systematic 404 errors
- **Root Cause**: This design prioritizes compatibility with traditional APT clients (configured for specific architectures) over repository integrity
- **Workaround**: Attempt fetches based on Release metadata but handle 404s gracefully with warnings - the current approach is correct

## Design Philosophy

Delegates complex operations to existing tools and focuses on:
- Repository metadata access and presentation
- Simple package retrieval
- Structured output for pipeline integration
- No system configuration required

## Contributing

Always run "go fmt ./..." before committing
