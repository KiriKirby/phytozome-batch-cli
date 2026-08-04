package phylo

import (
	"strings"

	"github.com/KiriKirby/phytozome-go/internal/fastautil"
)

func InputFASTA(records []InputRecord) string {
	return inputFASTA(records, nil)
}

func RuntimeInputFASTA(records []InputRecord, settings TreeSettings) string {
	return runtimeInputFASTA(records, settings)
}

func runtimeInputFASTA(records []InputRecord, settings TreeSettings) string {
	normalize := runtimeSequenceNormalizer(settings)
	return inputFASTA(records, normalize)
}

func inputFASTA(records []InputRecord, normalize func(string) string) string {
	var b strings.Builder
	for _, record := range records {
		sequence := sanitizeFASTASequence(record.Sequence)
		if normalize != nil {
			sequence = normalize(sequence)
		}
		b.WriteString(fastautil.NormalizeGeneratedHeader(record.TaxonID))
		b.WriteByte('\n')
		writeWrappedSequence(&b, sequence, 80)
	}
	return b.String()
}

func runtimeSequenceNormalizer(settings TreeSettings) func(string) string {
	return trimTerminalStars
}

func trimTerminalStars(sequence string) string {
	return strings.TrimRight(sequence, "*")
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
