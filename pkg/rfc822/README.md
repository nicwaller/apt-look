# rfc822 package

A Go package for parsing RFC822-style messages, including APT repository Release and Packages files.

## Features

- **Spec-compliant parsing**: Follows RFC822-style message format specification
- **Comment handling**: Skips lines starting with '#'
- **Field validation**: Rejects invalid field names per RFC822 rules
- **Duplicate detection**: Prevents duplicate fields within the same record
- **Case-insensitive access**: Retrieve fields regardless of case
- **Multi-line support**: Handles continuation lines and folded fields
- **Field ordering preservation**: Maintains original field order for byte-for-byte round-trip conversion
- **Perfect round-trip integrity**: Control format → JSON → Control format with identical output

## API

### Types

- **`Field`**: A single field with Name and Value ([]string for line-based handling)
- **`Record`**: A slice of Fields representing a single control file record (paragraph), preserving field order
- **`Parser`**: The main parser type

### Methods

- **`NewParser()`**: Create a new parser instance
- **`ParseRecords(r io.Reader)`**: Returns an iterator over records (memory-efficient)
- **`Record.Lookup(field string) ([]string, bool)`**: Get field value lines and existence check (case-insensitive)
- **`Record.Get(field string)`**: Get field value as single string (case-insensitive, returns empty string if not found)
- **`Record.GetLines(field string)`**: Get field value as lines (case-insensitive, returns empty slice if not found)
- **`Record.Has(field string)`**: Check if field exists (case-insensitive)
- **`Record.Fields()`**: List all field names in order
- **`Record.String()`**: Convert record back to RFC822-style format

### Validation

The parser enforces RFC822-style message format specification:

- Field names cannot be empty
- Field names cannot start with '#' or '-'
- Field names must use US-ASCII characters excluding control chars, spaces, and colons
- Duplicate fields within the same record are rejected
- Comments (lines starting with '#') are ignored

## Testing

Run tests with:

```bash
go test ./pkg/rfc822
```

The test suite includes real-world data from multiple APT repositories to ensure robust parsing.
