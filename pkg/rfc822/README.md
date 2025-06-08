# rfc822 package

A Go package for parsing RFC 822 header sections from email-style messages.

## Overview

This package provides a partial implementation of [RFC 822](https://datatracker.ietf.org/doc/html/rfc822) "Standard for the format of ARPA Internet text messages", focusing specifically on header field parsing. It parses field-value pairs according to RFC 822 specification and stops at the first blank line (which separates headers from message body in RFC 822).

This implementation includes only the subset of RFC 822 needed to support the `deb822` package, which uses RFC 822-style syntax for APT repository metadata files.

For parsing multiple records separated by blank lines (as used in APT repository files), see the `deb822` package which builds upon this foundation.

## Features

- **RFC 822 compliant**: Follows RFC 822 header parsing specification
- **Single header parsing**: Parses one header section, stopping at first blank line
- **Field validation**: Rejects invalid field names per RFC 822 rules
- **Duplicate detection**: Prevents duplicate fields within the same header
- **Case-insensitive access**: Retrieve fields regardless of case
- **Multi-line support**: Handles continuation lines and folded fields per RFC 822
- **Field ordering preservation**: Maintains original field order for round-trip conversion

## API

### Types

- **`Field`**: A single header field with Name and Value ([]string for line-based handling)
- **`Header`**: A slice of Fields representing a single RFC 822 header section
- **`Parser`**: The main parser type

### Methods

- **`NewParser()`**: Create a new parser instance
- **`ParseHeader(r io.Reader) (Header, error)`**: Parse a single RFC 822 header section
- **`Header.Lookup(field string) (Field, bool)`**: Get field and existence check (case-insensitive)
- **`Header.Get(field string) string`**: Get field value as single string (case-insensitive)
- **`Header.GetLines(field string) []string`**: Get field value as lines (case-insensitive)
- **`Header.Has(field string) bool`**: Check if field exists (case-insensitive)
- **`Header.Fields() []string`**: List all field names in order
- **`Header.String() string`**: Convert header back to RFC 822 format
- **`FieldValues.Unfold() string`**: Convert field value to single logical line per RFC 822 unfolding rules

## Validation

The parser enforces RFC 822 header format specification:

- Field names cannot be empty
- Field names cannot start with '-' 
- Field names must use US-ASCII characters excluding control chars, spaces, and colons
- Duplicate fields within the same header are rejected
- Parsing stops at the first blank line (RFC 822 header/body separator)

## Testing

Run tests with:

```bash
go test ./pkg/rfc822
```

The test suite validates RFC 822 compliance and proper header parsing behavior.
