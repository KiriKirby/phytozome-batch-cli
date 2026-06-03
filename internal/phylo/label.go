package phylo

import (
	"strings"
	"unicode"
)

func FormatPHgoLabel(species string, geneID string, labelName string) string {
	return phgoLabelPart(SpeciesInitials(species)) + "-" + phgoLabelPart(geneID) + " (" + phgoLabelPart(labelName) + ")"
}

func FormatYTLabel(species string, geneLocus string, labelName string) string {
	labelName = strings.TrimSpace(labelName)
	if labelName == "" {
		return strings.TrimSpace(geneLocus)
	}
	return formatUnderscoreLabel(geneLocus, ensureLabelPrefix(SpeciesInitials(species), labelName))
}

func FormatYTV2Label(species string, geneLocus string, labelName string) string {
	labelName = strings.TrimSpace(labelName)
	if labelName == "" {
		return strings.TrimSpace(geneLocus)
	}
	return formatUnderscoreLabel(geneLocus, trimLabelPrefix(SpeciesInitials(species), labelName))
}

func formatUnderscoreLabel(geneLocus string, suffix string) string {
	geneLocus = strings.TrimSpace(geneLocus)
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return geneLocus
	}
	if geneLocus == "" {
		return suffix
	}
	return geneLocus + "_" + suffix
}

func ensureLabelPrefix(prefix string, labelName string) string {
	prefix = strings.TrimSpace(prefix)
	labelName = strings.TrimSpace(labelName)
	if prefix == "" || strings.HasPrefix(labelName, prefix) {
		return labelName
	}
	return prefix + labelName
}

func trimLabelPrefix(prefix string, labelName string) string {
	prefix = strings.TrimSpace(prefix)
	labelName = strings.TrimSpace(labelName)
	if prefix == "" {
		return labelName
	}
	return strings.TrimPrefix(labelName, prefix)
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
