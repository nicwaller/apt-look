package deb822

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"strings"

	"github.com/nicwaller/apt-look/pkg/rfc822"
)

// ParseRecords returns an iterator over multiple records from a deb822-style document
// Each record is separated by blank lines, which is a deb822 extension to RFC 822
func ParseRecords(r io.Reader) iter.Seq2[rfc822.Record, error] {
	return func(yield func(rfc822.Record, error) bool) {
		scanner := bufio.NewScanner(r)
		var lines []string

		flushRecord := func() bool {
			if len(lines) > 0 {
				// Join lines and parse as a single header
				content := strings.Join(lines, "\n")
				parser := rfc822.NewParser()
				record, err := parser.ParseHeader(strings.NewReader(content))
				if err != nil {
					yield(nil, fmt.Errorf("parsing record: %w", err))
					return false
				}
				if len(record) > 0 {
					if !yield(record, nil) {
						return false
					}
				}
				lines = lines[:0] // Reset slice
			}
			return true
		}

		for scanner.Scan() {
			line := scanner.Text()

			// Empty line indicates end of record
			if strings.TrimSpace(line) == "" {
				if !flushRecord() {
					return
				}
				continue
			}

			lines = append(lines, line)
		}

		// Flush any remaining record
		flushRecord()

		if err := scanner.Err(); err != nil {
			yield(nil, fmt.Errorf("scanner error: %w", err))
		}
	}
}