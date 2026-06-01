package phylo

import (
	"strings"
)

func InputFASTA(records []InputRecord) string {
	var b strings.Builder
	for _, record := range records {
		sequence := sanitizeFASTASequence(record.Sequence)
		b.WriteString(">")
		b.WriteString(record.TaxonID)
		b.WriteByte('\n')
		writeWrappedSequence(&b, sequence, 80)
	}
	return b.String()
}

func sanitizeFASTASequence(sequence string) string {
	var b strings.Builder
	b.Grow(len(sequence))
	for _, ch := range sequence {
		switch {
		case ch == '\r' || ch == '\n' || ch == '\t' || ch == ' ':
			continue
		default:
			b.WriteRune(ch)
		}
	}
	return strings.TrimSpace(b.String())
}

func writeWrappedSequence(b *strings.Builder, sequence string, width int) {
	if width <= 0 {
		width = 80
	}
	runes := []rune(sequence)
	for len(runes) > 0 {
		n := width
		if len(runes) < n {
			n = len(runes)
		}
		b.WriteString(string(runes[:n]))
		b.WriteByte('\n')
		runes = runes[n:]
	}
}
