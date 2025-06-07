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

### Key Considerations
- Single binary with minimal dependencies
- Support for western Canada cloud regions and data centres
- Secure development practices and input validation
- Proper error handling for network/parsing issues
- Use newest available Go version and packages

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

## Future Considerations

- stdin is reserved for potential future features
- Tool designed to be composable with standard UNIX utilities
- Extensible architecture for additional output formats or repository types
