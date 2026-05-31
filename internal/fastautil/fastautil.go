package fastautil

import "strings"

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
