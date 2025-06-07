# apt-look

A command-line tool for exploring remote APT repositories without requiring system configuration. Following the UNIX philosophy of "do one thing and do it well", `apt-look` serves as a repository lens that fetches and presents APT repository data in a structured, scriptable way.

## Features

- Explore any APT repository by URL
- Query package metadata and statistics  
- Download specific packages
- Multiple output formats (text, JSON, TSV, raw)
- GPG verification with configurable strictness
- Local caching for performance
- Pipeline-friendly structured output

## Usage

```bash
# Basic repository exploration
apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main"
apt-look stats "deb http://archive.ubuntu.com/ubuntu/ jammy main"
apt-look search "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang

# Package operations
apt-look info "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21
apt-look download "deb http://archive.ubuntu.com/ubuntu/ jammy main" golang-1.21

# Work with source files
apt-look list /etc/apt/sources.list --filter="docker"

# Pipeline integration
apt-look list "deb http://archive.ubuntu.com/ubuntu/ jammy main" --format=json | jq '.packages[].name'
```

## Documentation

See [DESIGN.md](DESIGN.md) for complete specification, detailed examples, and technical architecture.
