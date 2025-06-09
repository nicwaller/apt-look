# apt-look Design Document

## Overview

`apt-look` is a command-line tool for exploring remote APT repositories without requiring system configuration. It follows the UNIX philosophy of "do one thing and do it well" - serving as a repository lens that fetches and presents APT repository data in a structured, scriptable way.

## Core Function

**Repository exploration and simple package retrieval:**
- Explore any APT repository by URL
- Query package metadata and statistics
- Download specific packages
- Output structured data for UNIX pipeline integration

## Command Interface

### Source Specification

`apt-look` accepts two types of source arguments:

**1. Full APT source line:**
```bash
apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main restricted"
apt-look list "http://archive.ubuntu.com/ubuntu/ jammy main"  # deb assumed
```

**2. Source list file:**
```bash
apt-look list /etc/apt/sources.list
apt-look list /etc/apt/sources.list.d/docker.list
```

### Commands (Subcommands)

All commands are mutually exclusive operations:

```bash
apt-look list <source> [options]           # List all packages
apt-look info <source> <package> [options] # Show package details
apt-look stats <source> [options]          # Repository statistics
apt-look download <source> <package> [options] # Download specific package
apt-look search <source> <term> [options]  # Search packages
```

### Global Options

**Architecture Selection:**
```bash
--arch=arm64                   # Default: detect current machine architecture
```

**Format Selection:**
```bash
--format=text|json|tsv|raw     # Default: text
```

**Format Behaviors:**
- `text` (default): Human-readable tables, colors if TTY
- `json`: Structured JSON for tools like `jq`
- `tsv`: Tab-separated values for `cut`, `awk`, spreadsheet import
- `raw`: Original Debian control format (pass-through)

**Multi-repository Filtering:**
```bash
--filter=pattern               # Filter repositories by pattern (for source files)
```

**Download Options:**
```bash
--output=path                  # Output path for downloaded packages
```

## Example Usage

### Basic Repository Exploration
```bash
# Explore repository structure
apt-look list http://archive.ubuntu.com/ubuntu/              # Show distributions
apt-look list http://archive.ubuntu.com/ubuntu/ jammy       # Show components  
apt-look list http://archive.ubuntu.com/ubuntu/ jammy main  # Show packages

# Get repository statistics
apt-look stats "deb http://archive.ubuntu.com/ubuntu/ jammy main restricted"

# Search for packages
apt-look search "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang
```

### Package Information and Retrieval
```bash
# Get package details
apt-look info "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21

# Download latest version
apt-look download "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21
```

### Working with Source Files
```bash
# Explore all configured repositories
apt-look list /etc/apt/sources.list

# Filter to specific repositories
apt-look list /etc/apt/sources.list --filter="docker"

# Search across all repos
apt-look search /etc/apt/sources.list golang --format=tsv | sort -k2
```

### Pipeline Integration
```bash
# Find largest packages
apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main" --format=json | \
  jq '.packages[] | select(.size > 1000000)' | jq -r '.name'

# Compare package versions across repos
apt-look list /etc/apt/sources.list --format=tsv | grep golang | cut -f1,2
```

## Output Examples

### Text Format (Default)
```
Package                Version        Size       Description
golang-1.21           1.21.5-1       45MB       Go programming language
python3-requests      2.28.1-1       156KB      HTTP library for Python
```

### JSON Format
```json
{
  "packages": [
    {
      "name": "golang-1.21",
      "version": "1.21.5-1",
      "size": 47185920,
      "description": "Go programming language"
    }
  ]
}
```

### TSV Format
```
golang-1.21	1.21.5-1	47185920	Go programming language
python3-requests	2.28.1-1	159744	HTTP library for Python
```

### Raw Format
```
Package: golang-1.21
Version: 1.21.5-1
Architecture: amd64
Maintainer: Go Compiler Team <team+go-compiler@tracker.debian.org>
Installed-Size: 45256
Depends: libc6 (>= 2.34)
Description: Go programming language
```

## Multi-Repository Output

When processing multiple repositories, output is prefixed with repository identifier:

```
[ubuntu/jammy/main] golang-1.21    1.21.5-1  45MB
[docker/stable]     containerd     1.6.24-1  28MB
```

## Technical Architecture (Go)

### Main Components
1. **HTTP client** with proper User-Agent and caching
2. **Debian control file parser** for Release/Packages files
3. **Command-line interface** using subcommands
4. **Output formatters** for text/json/tsv/raw
5. **Source list parser** for handling APT configuration files

### Compression Support

**APT Repository Compression Formats:**
APT repositories use various compression schemes for metadata files (Packages, Contents, Sources, Translation). Support priority is based on file size efficiency and native Go support:

**Tier 1 - Native Go Support (Preferred):**
- `.gz` (gzip) - Standard Go `compress/gzip`
- `.xz` (xzip/LZMA2) - Third-party Go package `github.com/ulikunitz/xz`
- `.zst` (Zstandard) - Third-party Go package `github.com/klauspost/compress/zstd`

**Tier 2 - System Binary Fallback:**
- `.bz2` (bzip2) - System `bunzip2` command
- `.lzma` (LZMA) - System `unlzma` command

**Compression Selection Strategy:**
When a Release file lists multiple compression formats for the same file:
1. **Choose smallest available format** that we support
2. **Prefer native Go support** over system binaries when sizes are similar (<10% difference)
3. **Fallback gracefully** if preferred format fails to decompress

**Implementation Notes:**
- Parse Release file entries to identify all available compression formats per file
- Sort by compressed file size (from Release metadata) ascending
- Attempt decompression with appropriate handler
- Cache decompressed content with filename based on uncompressed content hash

### Acquire-By-Hash Support

**APT Acquire-By-Hash Feature:**
Modern APT repositories support the Acquire-By-Hash mechanism for atomic updates and better mirror consistency. When `Acquire-By-Hash: yes` is present in the Release file, files are also available via their hash values.

**Hash-based URLs:**
Standard path: `main/binary-amd64/Packages.gz`
By-hash path: `main/binary-amd64/by-hash/SHA256/a1b2c3d4...`

**Implementation Strategy:**
1. **Check Release file** for `Acquire-By-Hash: yes` field
2. **Use strongest available hash** (SHA256 > SHA1 > MD5) from consolidated FileInfo
3. **Prefer by-hash URLs** when available for better mirror consistency
4. **Fallback to canonical paths** if by-hash fails or is unavailable
5. **Verify downloaded content** matches expected hash from Release file

**Benefits:**
- **Atomic updates**: Files referenced by hash are immutable
- **Mirror consistency**: Same hash always returns identical content
- **Integrity verification**: Built-in content validation
- **Concurrent safety**: Multiple apt-look instances can safely share cache

**URL Construction Example:**
```
Release file contains:
  Acquire-By-Hash: yes
  SHA256: a1b2c3d4ef56...890 1234567 main/binary-amd64/Packages.gz

Primary attempt:
  https://repo.example.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/a1b2c3d4ef56...890

Fallback attempt:
  https://repo.example.com/ubuntu/dists/jammy/main/binary-amd64/Packages.gz
```

### Package Version Sorting

**Debian Version Comparison Complexity:**
Debian package versions follow a complex format: `[epoch:]upstream-version[-debian-revision]` with intricate sorting rules that differ significantly from simple lexicographic or semantic version sorting.

**Example version comparisons:**
```
1:2.4.1-1ubuntu1 > 2.4.1-1ubuntu1  (epoch takes precedence)
2.4.1-1ubuntu2 > 2.4.1-1ubuntu1    (revision comparison)
2.4.1+dfsg-1 > 2.4.1-1             (upstream version rules)
2.4.10-1 > 2.4.2-1                 (numeric parts compared numerically)
```

**Implementation:**
- **Use `pault.ag/go/debian/version` package** for accurate Debian version parsing and comparison
- **Provides `version.Compare()` function** that handles all Debian version comparison rules
- **Supports epoch, upstream version, and revision parsing** with proper precedence
- **Essential for commands requiring "newest" package selection** (info, download, search results)

**Usage scenarios:**
- `apt-look info <source> <package>` - Show details for newest version
- `apt-look download <source> <package>` - Download newest version
- `apt-look search <source> <term>` - Sort results by version (newest first)
- Repository statistics - Identify version distributions

**Integration notes:**
- Parse version strings from Packages file `Version:` field
- Sort package slices using `version.Compare()` for consistent ordering
- Handle version comparison errors gracefully (malformed versions)
- Cache parsed version objects to avoid repeated parsing overhead

### APT Transport Plugin Integration

**System Transport Discovery:**
On Debian-based systems (including Ubuntu), APT provides transport plugins for accessing repositories via various protocols. These are particularly valuable for private/enterprise repositories.

**Common APT Transport Plugins:**
- `apt-transport-https` - HTTPS repositories (often built-in)
- `apt-transport-s3` - Amazon S3 private repositories
- `apt-transport-artifactory` - JFrog Artifactory integration
- `apt-transport-mirror` - Mirror method for redundancy
- `apt-transport-tor` - Tor network access

**Transport Plugin Discovery:**
```bash
# Standard locations for APT transport methods
/usr/lib/apt/methods/          # Primary location
/usr/local/lib/apt/methods/    # Local installations
/opt/*/lib/apt/methods/        # Third-party packages
```

**Implementation Strategy:**
1. **Detect Debian-based systems** via `/etc/os-release` or `/etc/debian_version`
2. **Scan standard APT method directories** for available transport plugins
3. **Wrap system transports** using our Transport interface
4. **Register discovered transports** in the transport registry
5. **Fallback to built-in transports** when system methods unavailable

**Transport Interface Integration:**
```go
type SystemTransport struct {
    scheme     string
    methodPath string  // e.g., "/usr/lib/apt/methods/s3"
}

func (t *SystemTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
    // Execute system APT method binary with APT protocol
    // Parse APT method response format
    // Return standardized AcquireResponse
}
```

**Benefits:**
- **Enterprise repository access**: S3, Artifactory, internal methods
- **Existing authentication**: Leverage system APT configuration
- **Plugin ecosystem**: Automatic support for third-party transport methods
- **Zero configuration**: Uses existing APT setup transparently

**Usage Examples:**
```bash
# Private S3 repository (requires apt-transport-s3)
apt-look list "deb s3://company-apt-bucket/ubuntu/ jammy main"

# Artifactory repository (requires apt-transport-artifactory)
apt-look list "deb artifactory+https://artifacts.company.com/ubuntu/ jammy main"

# Standard repositories continue working with built-in transports
apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main"
```

### Key Considerations
- Single binary with minimal dependencies
- Support for western Canada cloud regions and data centres
- Secure development practices and input validation
- Proper error handling for network/parsing issues
- Use newest available Go version and packages
- Intelligent compression format selection for bandwidth efficiency
- Seamless integration with system APT transport infrastructure

## Design Philosophy

Following UNIX philosophy, `apt-look` delegates complex operations to existing tools:

**Delegated to other tools:**
- Complex dependency resolution → `apt-cache depends`, `apt-rdepends`
- Text processing → `grep`, `awk`, `sed`, `jq`
- Package installation → `apt`, `aptitude`
- GPG verification → `gpg`, `apt-key`

**Core focus:**
- Repository metadata access and presentation
- Simple package retrieval
- Structured output for pipeline integration
- No system configuration required

## Compression Format Examples

**Real-world APT repository compression usage:**

```bash
# Debian repositories often provide multiple formats:
# main/binary-amd64/Packages      (uncompressed - largest)
# main/binary-amd64/Packages.gz   (gzip - widely supported)
# main/binary-amd64/Packages.xz   (xz - smallest, modern)

# Ubuntu repositories typically provide:
# main/binary-amd64/Packages.gz   (primary format)
# main/binary-amd64/Packages.xz   (smaller alternative)

# Some newer repositories may include:
# main/binary-amd64/Packages.zst  (Zstandard - fastest decompression)
```

**Selection algorithm example:**
```
Available: Packages.xz (1.2MB), Packages.gz (2.1MB), Packages (8.4MB)
Selection: Packages.xz (smallest + native Go support)

Available: Packages.bz2 (1.8MB), Packages.gz (2.1MB) 
Selection: Packages.bz2 (smaller despite system binary requirement)

Available: Packages.zst (1.5MB), Packages.xz (1.2MB)
Selection: Packages.xz (smaller wins over compression speed)
```

**Acquire-By-Hash integration example:**
```
Repository supports Acquire-By-Hash: yes
Selected: main/binary-amd64/Packages.xz (1.2MB, SHA256: abc123...)

Primary URL (by-hash):
  /dists/jammy/main/binary-amd64/by-hash/SHA256/abc123def456...

Fallback URL (canonical):
  /dists/jammy/main/binary-amd64/Packages.xz

Cache key: Content-based hash of decompressed Packages data
Verification: Downloaded content SHA256 must match abc123def456...
```

**Package version sorting example:**
```go
// Multiple versions of same package found across components/architectures
packages := []Package{
    {Name: "golang-1.21", Version: "1.21.5-1ubuntu1"},
    {Name: "golang-1.21", Version: "1:1.21.4-1"},       // epoch 1
    {Name: "golang-1.21", Version: "1.21.5-1ubuntu2"},
    {Name: "golang-1.21", Version: "1.21.10-1"},        // 10 > 5 numerically
}

// Sort by version using Debian comparison rules
sort.Slice(packages, func(i, j int) bool {
    return version.Compare(packages[i].Version, packages[j].Version) > 0
})

// Result (newest first):
// 1:1.21.4-1 (epoch wins despite lower number)
// 1.21.10-1  (numeric comparison: 10 > 5)
// 1.21.5-1ubuntu2 (revision: ubuntu2 > ubuntu1)
// 1.21.5-1ubuntu1
```

**APT transport plugin integration example:**
```bash
# System discovery on Ubuntu
$ ls /usr/lib/apt/methods/
http  https  ftp  file  copy  gzip  bzip2  lzma  s3

# apt-look automatically registers available transports
Transport registry initialized:
- http (built-in)
- https (built-in) 
- file (built-in)
- s3 (system: /usr/lib/apt/methods/s3)

# Private S3 repository access works transparently
$ apt-look stats "deb s3://company-apt/ubuntu/ jammy main"
Repository Statistics
====================
Origin: Company Internal
Total Packages: 1,247
...
```

## Future Considerations

- stdin is reserved for potential future features
- Tool designed to be composable with standard UNIX utilities
- Extensible architecture for additional output formats or repository types
- Potential support for additional compression formats as they emerge (e.g., Brotli)
- Enhanced APT method protocol support for advanced authentication schemes
