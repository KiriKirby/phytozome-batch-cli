package fastautil

import (
	"strings"
	"unicode"
)

// NormalizeGeneratedHeader returns one physical FASTA header line with every
// whitespace character encoded as an underscore. Input parsers intentionally
// do not use this function so legacy FASTA headers with spaces remain valid.
func NormalizeGeneratedHeader(header string) string {
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, ">")
	if header == "" {
		return ">"
	}
	var b strings.Builder
	b.Grow(len(header) + 1)
	b.WriteByte('>')
	for _, r := range header {
		if unicode.IsSpace(r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// IsIgnoredPHGONoteHeader reports whether a FASTA header belongs to a PHgo note
// entry that should be skipped by parsers across the application.
func IsIgnoredPHGONoteHeader(header string) bool {
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, ">")
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(header), "phgo://note")
}
