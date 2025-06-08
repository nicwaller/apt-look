# deb822

This package provides Go types and parsers for APT repository metadata files, including Release files that describe repository structure, Packages files that contain package metadata, and sources.list files that define repository configurations.

## deb822 file format specification

[deb822](https://manpages.ubuntu.com/manpages/focal/man5/deb822.5.html) is a file format that is foundational to the Debian/APT package ecosystem. It was originally specified for use in `control` files that provide metadata for Debian packages (`.deb` files). The syntax and structure was later reused for `Release` and `Packages` files in Apt repositories. And starting with APT 1.1 in 2016, the deb822 file format is also now being used for source lists.

The official specification is available here:

https://manpages.ubuntu.com/manpages/focal/man5/deb822.5.html

## Relationship with RFC 822

[RFC 822](https://datatracker.ietf.org/doc/html/rfc822) "Standard for the format of ARPA internet text messages" originates from 1982 and defines the message format for email.

Although deb822 borrows several key concepts from RFC 822, it has some key differences. Files using deb822 syntax and structure cannot always be parsed using a strict implementation of RFC 822. Consider these example messages.

### Examples

An RFC 822 email message starts with a sequence of header lines followed by a message body. Note the `Subject` header field is using the folding technique described by RFC 822.

```
From: user@example.com
To: recipient@example.com
Subject: Quarterly Business Review Meeting - Q2 2025 Financial Results
 and Strategic Planning Session
Date: Sat, 7 Jun 2025 10:00:00 -0700

This is the message body.
```

deb822 was originally specified to provide package metadata in a control file. This is a valid message according to RFC822, but the message body is omitted. 

```deb822
Package: alpha
Version: 1.0.0-1
Architecture: amd64
Description: An example package
 This is a longer description that continues
 on multiple lines with proper indentation.
```

In an Apt repository, the `Release` file reuses deb822 syntax and structure. However, `MD5Sum` and the other hash fields specify different whitespace handling; instead of unfolding the lines according to RFC822, each line should be considered individually. One clue available to the parser is the omission of field value content on the `MD5Sum` line.

```deb822
Label: Brave Browser
Codename: stable
Architectures: amd64 arm64
Components: main
MD5Sum:
 02fe9a3926ce03cd7c3e2201a26629ef    12617 Contents-amd64
 d6601756eda716c30a6cc85d732dd800     1127 Contents-amd64.gz
 02fe9a3926ce03cd7c3e2201a26629ef    12617 Contents-arm64
 d6601756eda716c30a6cc85d732dd800     1127 Contents-arm64.gz
```

In an Apt repository, the `Packages` file can be interpreted as a sequence of RFC822 header blocks.

```deb822
Package: alpha
Version: 1.0.0-1
Architecture: amd64
Description: An example package
 This is a longer description that continues
 on multiple lines with proper indentation.

Package: bravo
Version: 1.0.0-1
Architecture: amd64
```

## Features

- **Release File Parsing**: Complete support for APT Release files with all standardized fields
- **Packages File Parsing**: Full support for APT Packages files with comprehensive package metadata
- **Sources.list Parsing**: Complete parser for APT sources.list format with options support
- **Hash Verification**: Parse and access MD5Sum, SHA1, and SHA256 hash entries for repository integrity
- **Dependency Parsing**: Extract and structure package dependency relationships
- **Options Handling**: Full support for APT source options (arch, trusted, signed-by, etc.)
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

### Sources.list Files

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/nicwaller/apt-look/pkg/aptrepo"
)

func main() {
    file, err := os.Open("/etc/apt/sources.list")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    
    sourcesList, err := aptrepo.ParseSourcesList(file)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Found %d source entries\n", len(sourcesList.Entries))
    
    // Show enabled entries
    enabled := sourcesList.GetEnabledEntries()
    fmt.Printf("Enabled entries: %d\n", len(enabled))
    
    for _, entry := range enabled {
        fmt.Printf("Type: %s\n", entry.Type)
        fmt.Printf("URI: %s\n", entry.URI)
        fmt.Printf("Distribution: %s\n", entry.Distribution)
        fmt.Printf("Components: %v\n", entry.Components)
        
        // Check for options
        if arch := entry.GetOption("arch", ""); arch != "" {
            fmt.Printf("Architecture: %s\n", arch)
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

### Sources.list File Fields

**Core Fields:**
- **Type**: Entry type (`deb` for binaries, `deb-src` for sources)
- **URI**: Repository URL or file path
- **Distribution**: Suite name or codename (e.g., `stable`, `jammy`, `bookworm`)
- **Components**: Repository components (e.g., `main`, `contrib`, `non-free`)

**Options (in square brackets):**
- **arch**: Target architectures (e.g., `amd64`, `arm64`)
- **trusted**: Skip signature verification (`yes`/`no`)
- **signed-by**: Path to GPG keyring file
- **lang**: Language preferences for descriptions
- **target**: Specify download targets
- **pdiffs**: Enable/disable partial index files

**Metadata:**
- **Enabled**: Whether the entry is active (not commented out)
- **OriginalLine**: Preserved original source line text
- **LineNumber**: Line number in the source file

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
