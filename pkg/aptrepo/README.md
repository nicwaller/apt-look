# aptrepo

This package provides Go types and parsers for APT repository metadata files, including Release files that describe repository structure and Packages files that contain package metadata.

## Features

- **Release File Parsing**: Complete support for APT Release files with all standardized fields
- **Packages File Parsing**: Full support for APT Packages files with comprehensive package metadata
- **Hash Verification**: Parse and access MD5Sum, SHA1, and SHA256 hash entries for repository integrity
- **Dependency Parsing**: Extract and structure package dependency relationships
- **Flexible Date Handling**: Robust parsing of various date formats found in real-world repositories
- **JSON Serialization**: Built-in JSON support for structured output and API integration
- **Type Safety**: Structured Go types for all APT metadata with proper validation
- **Real-World Compatibility**: Successfully tested against files from major repositories

## Usage

### Release Files

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

### Packages Files

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/nicwaller/apt-look/pkg/aptrepo"
)

func main() {
    file, err := os.Open("Packages")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    
    for pkg, err := range aptrepo.ParsePackages(file) {
        if err != nil {
            panic(err)
        }
        
        fmt.Printf("Package: %s\n", pkg.Package)
        fmt.Printf("Version: %s\n", pkg.Version)
        fmt.Printf("Architecture: %s\n", pkg.Architecture)
        fmt.Printf("Size: %d bytes\n", pkg.Size)
        fmt.Printf("Description: %s\n", pkg.Description)
        
        // Access dependency information
        deps := pkg.GetDependencies()
        if depends, ok := deps["depends"]; ok {
            fmt.Printf("Depends: %v\n", depends)
        }
        
        fmt.Println("---")
    }
}
```

## Supported Fields

### Release File Fields

**Mandatory Fields:**
- **Suite/Codename**: Repository version identifier
- **Architectures**: Supported architectures (amd64, arm64, etc.)
- **Components**: Repository areas (main, contrib, non-free, etc.)
- **Date**: Release file creation timestamp
- **SHA256**: Cryptographic hashes for repository files

**Optional Fields:**
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

**Legacy Fields:**
- **MD5Sum**: MD5 hashes (not for security, compatibility only)
- **SHA1**: SHA1 hashes (not for security, compatibility only)

### Package File Fields

**Mandatory Fields:**
- **Package**: Package name
- **Filename**: Path to package file relative to repository root
- **Size**: Package file size in bytes

**Highly Recommended Fields:**
- **Architecture**: Target architecture (amd64, arm64, all, etc.)
- **Version**: Package version string
- **SHA256**: Package file cryptographic hash
- **Description**: Package description

**Dependency Fields:**
- **Depends**: Required dependencies
- **Pre-Depends**: Pre-installation dependencies
- **Recommends**: Recommended packages
- **Suggests**: Suggested packages
- **Enhances**: Enhanced packages
- **Breaks**: Conflicting packages (breaks)
- **Conflicts**: Conflicting packages
- **Provides**: Virtual packages provided
- **Replaces**: Replaced packages

**Control Fields:**
- **Priority**: Package priority (required, important, standard, optional, extra)
- **Section**: Package category (admin, devel, libs, etc.)
- **Source**: Source package name
- **Maintainer**: Package maintainer
- **Homepage**: Project homepage URL
- **InstalledSize**: Installed size in KB

**Additional Fields:**
- **Essential**: Critical system package flag
- **MultiArch**: Multi-architecture support
- **Tag**: Package tags for categorization
- **License**: License information
- **PhasedUpdatePercentage**: Gradual rollout percentage

## Specification

This implementation follows the official Debian Repository Format specification:

- **[Debian Repository Format - Release Files](https://wiki.debian.org/DebianRepository/Format#A.22Release.22_files)**
- **[Debian Repository Format - Packages Files](https://wiki.debian.org/DebianRepository/Format#A.22Packages.22_files)**

The specification defines the structure, required fields, and semantic meaning of APT metadata files used across the Debian ecosystem.

## Architecture

Built on the solid foundation of the `rfc822` package, this parser:

1. **Leverages RFC822 parsing** for robust field extraction and validation
2. **Adds APT-specific semantics** with proper type conversion and validation
3. **Handles real-world variations** in date formats and field presence
4. **Provides both high-level and low-level access** to parsed data

## Testing

Both parsers are thoroughly tested against real APT metadata files from major repositories including:
- Spotify, Docker, Google Chrome, Brave Browser
- HashiCorp, Kubernetes, Microsoft, NodeSource
- PostgreSQL, Signal Desktop

All tests verify both parsing correctness and compatibility with the diverse formats found in production repositories. The package successfully parses:
- **10 Release files** with varying field combinations and date formats
- **Thousands of Package records** across different repository types and architectures