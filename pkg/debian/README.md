# debian package

A Go package for parsing Debian control format files, including APT repository Release and Packages files.

## Features

- **Spec-compliant parsing**: Follows the official Debian control field format specification
- **Comment handling**: Skips lines starting with '#'
- **Field validation**: Rejects invalid field names per Debian policy
- **Duplicate detection**: Prevents duplicate fields within the same record
- **Case-insensitive access**: Retrieve fields regardless of case
- **Multi-line support**: Handles continuation lines and folded fields

## API

### Types

- **`Record`**: A map representing a single control file record (paragraph)
- **`Parser`**: The main parser type

### Methods

- **`NewParser()`**: Create a new parser instance
- **`ParseRecords(r io.Reader)`**: Returns an iterator over records (memory-efficient)
- **`Record.Lookup(field string) (string, bool)`**: Get field value and existence check (case-insensitive)
- **`Record.Get(field string)`**: Get field value (case-insensitive, returns empty string if not found)
- **`Record.Has(field string)`**: Check if field exists (case-insensitive)
- **`Record.Fields()`**: List all field names

### Validation

The parser enforces Debian control field format specification:

- Field names cannot be empty
- Field names cannot start with '#' or '-'
- Field names must use US-ASCII characters excluding control chars, spaces, and colons
- Duplicate fields within the same record are rejected
- Comments (lines starting with '#') are ignored

## Testing

Run tests with:

```bash
go test ./pkg/debian
```

The test suite includes real-world data from multiple APT repositories to ensure robust parsing.
