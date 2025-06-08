# aptrepo

This package provides Go types and parsers for APT repository metadata files, specifically focusing on Release files that describe repository structure and integrity information.

## Features

- **Release File Parsing**: Complete support for APT Release files with all standardized fields
- **Hash Verification**: Parse and access MD5Sum, SHA1, and SHA256 hash entries for repository integrity
- **Flexible Date Handling**: Robust parsing of various date formats found in real-world repositories
- **Type Safety**: Structured Go types for all Release file data with proper validation
- **Real-World Compatibility**: Successfully tested against Release files from major repositories

## Usage

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/nicwaller/apt-look/pkg/aptrepo"
)

func main() {
    file, err := os.Open("Release")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    
    release, err := aptrepo.ParseRelease(file)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Repository: %s %s\n", release.Origin, release.Label)
    fmt.Printf("Suite: %s, Codename: %s\n", release.Suite, release.Codename)
    fmt.Printf("Architectures: %v\n", release.Architectures)
    fmt.Printf("Components: %v\n", release.Components)
    fmt.Printf("Date: %s\n", release.Date)
    
    // Access hash information
    for _, entry := range release.SHA256 {
        fmt.Printf("SHA256: %s %d %s\n", entry.Hash, entry.Size, entry.Path)
    }
}
```

## Supported Fields

### Mandatory Fields
- **Suite/Codename**: Repository version identifier
- **Architectures**: Supported architectures (amd64, arm64, etc.)
- **Components**: Repository areas (main, contrib, non-free, etc.)
- **Date**: Release file creation timestamp
- **SHA256**: Cryptographic hashes for repository files

### Optional Fields
- **Origin**: Repository origin description
- **Label**: Repository label
- **Version**: Repository version string
- **ValidUntil**: Expiration timestamp
- **NotAutomatic**: Automatic installation restriction flag
- **ButAutomaticUpgrades**: Upgrade behavior flag
- **AcquireByHash**: Hash-based file retrieval support
- **SignedBy**: OpenPGP key fingerprints
- **PackagesRequireAuthorization**: Authorization requirements
- **Changelogs**: Package changelog URL template
- **Snapshots**: Archive snapshot URL template
- **NoSupportForArchitectureAll**: Architecture handling flags

### Legacy Fields
- **MD5Sum**: MD5 hashes (not for security, compatibility only)
- **SHA1**: SHA1 hashes (not for security, compatibility only)

## Specification

This implementation follows the official Debian Repository Format specification:

**[Debian Repository Format - Release Files](https://wiki.debian.org/DebianRepository/Format#A.22Release.22_files)**

The specification defines the structure, required fields, and semantic meaning of APT Release files used across the Debian ecosystem.

## Architecture

Built on the solid foundation of the `rfc822` package, this parser:

1. **Leverages RFC822 parsing** for robust field extraction and validation
2. **Adds APT-specific semantics** with proper type conversion and validation
3. **Handles real-world variations** in date formats and field presence
4. **Provides both high-level and low-level access** to parsed data

## Testing

The parser is thoroughly tested against real Release files from major repositories including:
- Spotify, Docker, Google Chrome, Brave Browser
- HashiCorp, Kubernetes, Microsoft, NodeSource
- PostgreSQL, Signal Desktop

All tests verify both parsing correctness and compatibility with the diverse formats found in production repositories.