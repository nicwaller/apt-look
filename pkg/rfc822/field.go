package rfc822

import (
	"fmt"
	"strings"
)

// Header represents a single header section in an RFC822-style message
// Fields are stored in a slice to preserve the original ordering for round-trip conversion
type Header []Field

// Field represents a single field in an RFC822-style message
type Field struct {
	Name  string
	Value FieldValues
}

// String() is used to display values in the Go debugger
func (f Field) String() string {
	return fmt.Sprintf("%s: %s", f.Name, f.Value.String())
}

// FieldValues are stored as a slice of strings in order to permit flexible handling of multi-line fields.
// Although RFC822 section 3.1.1 prescribes a specific handling for long header fields, Apt uses a different style for MD5Sum and other hash fields.
// So, values are stored in a slice to allow for flexible handling based on the field name.
type FieldValues []string

// Unfold returns the field value as a single logical line according to RFC822 unfolding rules
// CRLF immediately followed by LWSP-char is replaced with the LWSP-char (space).
func (f FieldValues) Unfold() string {
	// Join all lines with a single space, effectively "unfolding" the field
	return strings.Join(f, " ")
}

// String() is used to display values in the Go debugger
func (f FieldValues) String() string {
	return f.Unfold()
}
