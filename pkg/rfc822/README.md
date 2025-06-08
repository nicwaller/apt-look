# rfc822 package

A Go package for parsing RFC 822 header sections from email-style messages.

## Overview

This package provides a pure RFC 822 parser that handles the header section of email messages. It parses field-value pairs according to RFC 822 specification and stops at the first blank line (which separates headers from message body in RFC 822).

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
- **`Record`**: A slice of Fields representing a single RFC 822 header section
- **`Parser`**: The main parser type

### Methods

- **`NewParser()`**: Create a new parser instance
- **`ParseHeader(r io.Reader) (Record, error)`**: Parse a single RFC 822 header section
- **`Record.Lookup(field string) (Field, bool)`**: Get field and existence check (case-insensitive)
- **`Record.Get(field string) string`**: Get field value as single string (case-insensitive)
- **`Record.GetLines(field string) []string`**: Get field value as lines (case-insensitive)
- **`Record.Has(field string) bool`**: Check if field exists (case-insensitive)
- **`Record.Fields() []string`**: List all field names in order
- **`Record.String() string`**: Convert record back to RFC 822 format
- **`FieldValues.Unfold() string`**: Convert field value to single logical line per RFC 822 unfolding rules

## Usage

```go
package main

import (
    "fmt"
    "strings"
    
    "github.com/nicwaller/apt-look/pkg/rfc822"
)

func main() {
    input := `From: user@example.com
To: recipient@example.com
Subject: Hello World
Date: Mon, 1 Jan 2024 12:00:00 +0000

This is the message body.`

    parser := rfc822.NewParser()
    header, err := parser.ParseHeader(strings.NewReader(input))
    if err != nil {
        panic(err)
    }

    fmt.Printf("From: %s\n", header.Get("From"))
    fmt.Printf("Subject: %s\n", header.Get("Subject"))
    fmt.Printf("Has Date: %t\n", header.Has("Date"))
}
```

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
