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
- Cache Packages files locally on disk in `$XDG_CACHE_HOME/apt-look/` (fallback to `~/.cache/apt-look/` if XDG_CACHE_HOME not set)
- Use content-based naming: cache files are named using the MD5 hash of the plaintext contents
- Cache files should be gzip-compressed, but the name is still based on the plaintext contents.
- The `purge-cache` subcommand purges the apt-look cache
- The `--no-cache` flag disables use of the cache. This can be useful during troubleshooting, such as when troubleshooting a webserver and reviewing request logs. 

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

## Design Philosophy

Delegates complex operations to existing tools and focuses on:
- Repository metadata access and presentation
- Simple package retrieval
- Structured output for pipeline integration
- No system configuration required
