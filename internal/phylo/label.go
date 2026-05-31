package phylo

import (
	"strings"
	"unicode"
)

func FormatPHgoLabel(species string, geneID string, labelName string) string {
	return phgoLabelPart(SpeciesInitials(species)) + "-" + phgoLabelPart(geneID) + " (" + phgoLabelPart(labelName) + ")"
}

func SpeciesInitials(species string) string {
	words := strings.Fields(strings.TrimSpace(species))
	if len(words) < 2 {
		return ""
	}
	first := firstWordInitial(words[0])
	second := firstWordInitial(words[1])
	if first == "" || second == "" {
		return ""
	}
	return first + second
}

func phgoLabelPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "~"
	}
	return value
}

func firstWordInitial(word string) string {
	for _, r := range strings.TrimSpace(word) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(r)
		}
	}
	return ""
}
