// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package labelname

import (
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/KiriKirby/phytozome-go/internal/fastautil"
)

var (
	ecNumberLikePattern = regexp.MustCompile(`^(?:EC[:\-]?)?[A-Za-z]?\d+(?:\.\d+){2,3}$`)
	lemnaGeneIDPattern  = regexp.MustCompile(`(?i)^SP\d{4}D\d{3}G\d{6}(?:_T\d+)?$`)
	locusIDPattern      = regexp.MustCompile(`(?i)^(?:AT[1-5CM]G\d{5}|[A-Z]{2}\d+G\d+|LOC[A-Z_]*\d+|[A-Z][a-z]\d{2}g\d{5,})(?:[._-]?[A-Z]?\d+)?$`)
	primarySymbolRx     = regexp.MustCompile(`^[A-Z0-9]+$`)
)

type AliasRankRequest struct {
	TaskTimestamp     string
	ItemIndex         int
	TaxID             string
	SearchTerm        string
	Symbol            string
	ProteinID         string
	GeneID            string
	TranscriptID      string
	SequenceID        string
	LocusTag          string
	Aliases           []string
	Synonyms          []string
	DBXrefs           []string
	Chromosome        string
	MapLocation       string
	Description       string
	TypeOfGene        string
	SymbolAuthority   string
	FullNameAuthority string
	OtherDesignations []string
	FeatureType       string
}

type AliasRankResult struct {
	TaskTimestamp string
	ItemIndex     int
	RankedAliases []string
}

func RankAliases(request AliasRankRequest) AliasRankResult {
	return AliasRankResult{
		TaskTimestamp: request.TaskTimestamp,
		ItemIndex:     request.ItemIndex,
		RankedAliases: rankAliasRequest(request),
	}
}

func RankAliasBatch(requests []AliasRankRequest) []AliasRankResult {
	if len(requests) == 0 {
		return nil
	}
	results := make([]AliasRankResult, len(requests))
	cacheIndex := make(map[string]int, len(requests))
	uniqueRequests := make([]AliasRankRequest, 0, len(requests))
	requestIndexes := make([]int, len(requests))
	for i, request := range requests {
		key := aliasRankCacheKey(request)
		if index, ok := cacheIndex[key]; ok {
			requestIndexes[i] = index
			continue
		}
		index := len(uniqueRequests)
		cacheIndex[key] = index
		requestIndexes[i] = index
		uniqueRequests = append(uniqueRequests, request)
	}
	uniqueRanked := rankAliasRequestItemsBatch(uniqueRequests)
	rankedByRequest := make([][]rankedAlias, len(requests))
	for i, index := range requestIndexes {
		rankedByRequest[i] = cloneRankedAliases(uniqueRanked[index])
	}
	familyCounts := batchFamilyCounts(rankedByRequest)
	for i, request := range requests {
		ranked := sortRankedAliases(rankedByRequest[i], familyCounts)
		results[i] = AliasRankResult{
			TaskTimestamp: request.TaskTimestamp,
			ItemIndex:     request.ItemIndex,
			RankedAliases: filteredRankedAliasTexts(ranked),
		}
	}
	return results
}

func rankAliasRequest(request AliasRankRequest) []string {
	return filteredRankedAliasTexts(sortRankedAliases(rankAliasRequestItems(request), nil))
}

type rankedAlias struct {
	Text   string
	Score  int
	Family string
}

func rankAliasRequestItems(request AliasRankRequest) []rankedAlias {
	if db, ok := openDefaultGeneDB(); ok {
		if ranked, handled := db.rank(request); handled {
			return ranked
		}
	}
	return nil
}

func rankAliasRequestItemsBatch(requests []AliasRankRequest) [][]rankedAlias {
	out := make([][]rankedAlias, len(requests))
	if len(requests) == 0 {
		return out
	}
	db, ok := openDefaultGeneDB()
	if !ok || db == nil {
		return out
	}
	if len(requests) < 4 {
		for i, request := range requests {
			if ranked, handled := db.rank(request); handled {
				out[i] = ranked
			}
		}
		return out
	}
	workers := runtime.GOMAXPROCS(0) * 4
	if workers < 8 {
		workers = 8
	}
	if workers > len(requests) {
		workers = len(requests)
	}
	if workers > 64 {
		workers = 64
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ranked, handled := db.rank(requests[index]); handled {
					out[index] = ranked
				}
			}
		}()
	}
	for i := range requests {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

func aliasRankCacheKey(request AliasRankRequest) string {
	values := make([]string, 0, len(request.Aliases)+len(request.Synonyms)+len(request.DBXrefs)+len(request.OtherDesignations)+16)
	for _, value := range []string{
		request.TaxID,
		request.SearchTerm,
		request.Symbol,
		request.ProteinID,
		request.GeneID,
		request.TranscriptID,
		request.SequenceID,
		request.LocusTag,
		request.Chromosome,
		request.MapLocation,
		request.Description,
		request.TypeOfGene,
		request.SymbolAuthority,
		request.FullNameAuthority,
		request.FeatureType,
	} {
		if normalized := normalizeAliasKey(value); normalized != "" {
			values = append(values, normalized)
		}
	}
	for _, value := range request.Aliases {
		if normalized := normalizeAliasKey(value); normalized != "" {
			values = append(values, normalized)
		}
	}
	for _, group := range [][]string{request.Synonyms, request.DBXrefs, request.OtherDesignations} {
		for _, value := range group {
			if normalized := normalizeAliasKey(value); normalized != "" {
				values = append(values, normalized)
			}
		}
	}
	return strings.Join(values, "\x00")
}

func RankedAliases(aliases []string) []string {
	return rankAliasRequest(AliasRankRequest{Aliases: aliases})
}

func aliasScores(aliases []string) map[string]int {
	scores := make(map[string]int, len(aliases))
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		key := normalizeAliasKey(trimmed)
		if _, ok := scores[key]; ok {
			continue
		}
		scores[key] = AliasPreferenceScore(trimmed) + QueryAliasPrimarySymbolBonus(trimmed) + aliasRedundantLongFormPenalty(trimmed, aliases)
	}
	return scores
}

func sortRankedAliases(items []rankedAlias, familyCounts map[string]int) []rankedAlias {
	items = cloneRankedAliases(items)
	if len(items) == 0 {
		return nil
	}
	primary := make([]rankedAlias, 0, len(items))
	secondary := make([]rankedAlias, 0)
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		item.Text = text
		if isPrimarySymbolNameCandidate(text) {
			primary = append(primary, item)
		} else {
			secondary = append(secondary, item)
		}
	}
	if familyCounts == nil {
		familyCounts = batchFamilyCounts([][]rankedAlias{primary})
	}
	sort.SliceStable(primary, func(i, j int) bool {
		if primary[i].Score != primary[j].Score {
			return primary[i].Score > primary[j].Score
		}
		leftFamilyCount := familyCounts[primary[i].Family]
		rightFamilyCount := familyCounts[primary[j].Family]
		if leftFamilyCount != rightFamilyCount {
			return leftFamilyCount > rightFamilyCount
		}
		if primary[i].Family != primary[j].Family {
			return primary[i].Family < primary[j].Family
		}
		if len(primary[i].Text) != len(primary[j].Text) {
			return len(primary[i].Text) < len(primary[j].Text)
		}
		return strings.ToLower(primary[i].Text) < strings.ToLower(primary[j].Text)
	})
	sort.SliceStable(secondary, func(i, j int) bool {
		left := strings.ToUpper(secondary[i].Text)
		right := strings.ToUpper(secondary[j].Text)
		if left != right {
			return left < right
		}
		return secondary[i].Text < secondary[j].Text
	})
	return append(primary, secondary...)
}

func batchFamilyCounts(groups [][]rankedAlias) map[string]int {
	counts := make(map[string]int)
	seenByFamily := make(map[string]map[string]struct{})
	for _, group := range groups {
		for _, item := range group {
			if strings.TrimSpace(item.Family) == "" {
				continue
			}
			if seenByFamily[item.Family] == nil {
				seenByFamily[item.Family] = map[string]struct{}{}
			}
			key := normalizeAliasKey(item.Text)
			if key == "" {
				continue
			}
			if _, ok := seenByFamily[item.Family][key]; ok {
				continue
			}
			seenByFamily[item.Family][key] = struct{}{}
			counts[item.Family]++
		}
	}
	for family, count := range counts {
		if count < 2 {
			counts[family] = 0
		}
	}
	return counts
}

func cloneRankedAliases(items []rankedAlias) []rankedAlias {
	return append([]rankedAlias(nil), items...)
}

func rankedAliasTexts(items []rankedAlias) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item.Text); text != "" {
			out = append(out, text)
		}
	}
	return uniqueStrings(out)
}

func filteredRankedAliasTexts(items []rankedAlias) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" || !isUsableRankedAliasText(text) {
			continue
		}
		out = append(out, text)
	}
	return uniqueStrings(out)
}

func isUsableRankedAliasText(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return isPrimarySymbolNameCandidate(value) || IsTrustedCandidate(value)
}

func isPrimarySymbolNameCandidate(value string) bool {
	value = strings.TrimSpace(value)
	if !primarySymbolRx.MatchString(value) || LooksLikeDatabaseIdentifier(value) {
		return false
	}
	for _, r := range value {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func symbolFamily(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	end := len(runes)
	for end > 0 {
		r := runes[end-1]
		if r >= '0' && r <= '9' {
			end--
			continue
		}
		break
	}
	for end > 0 {
		r := runes[end-1]
		if r == '-' || r == '_' || r == '.' {
			end--
			continue
		}
		break
	}
	prefix := strings.TrimSpace(string(runes[:end]))
	if prefix == "" || prefix == value {
		return ""
	}
	hasLetter := false
	for _, r := range prefix {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return ""
	}
	return strings.ToUpper(prefix)
}

func FastaHeaderLabelNameFromInput(input string) string {
	return FastaHeaderLabelName(firstFastaHeaderLine(input))
}

func FastaHeaderLabelName(header string) string {
	return ParentheticalHeaderLabel(header)
}

func ParentheticalHeaderLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	open := strings.LastIndex(value, " (")
	if open < 0 {
		return ""
	}
	rest := value[open+2:]
	closeIndex := strings.Index(rest, ")")
	if closeIndex < 0 {
		return ""
	}
	label := strings.TrimSpace(rest[:closeIndex])
	if label == "" {
		return ""
	}
	for _, ch := range label {
		if ch == ' ' || ch == '\t' {
			return ""
		}
	}
	return label
}

func SplitAliases(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ';' || r == ',' || r == '|' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func FirstAlias(value string) string {
	for _, part := range SplitAliases(value) {
		return part
	}
	return ""
}

func BestAlias(value string) string {
	best := ""
	bestScore := -1
	parts := SplitAliases(value)
	scores := aliasScores(parts)
	for _, part := range parts {
		if part == "" || !IsTrustedCandidate(part) {
			continue
		}
		score := scores[normalizeAliasKey(part)]
		if score > bestScore || (score == bestScore && len(part) < len(best)) {
			best = part
			bestScore = score
		}
	}
	return best
}

func TrustedLabel(candidates ...string) string {
	best := ""
	bestScore := -1
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !IsTrustedCandidate(candidate) {
			continue
		}
		score := TrustedLabelScore(candidate)
		if score > bestScore || (score == bestScore && len(candidate) < len(best)) {
			best = candidate
			bestScore = score
		}
	}
	return best
}

func aliasRedundantLongFormPenalty(candidate string, peers []string) int {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return 0
	}
	candidateUpper := strings.ToUpper(candidate)
	for _, peer := range peers {
		peer = strings.TrimSpace(peer)
		if peer == "" || strings.EqualFold(peer, candidate) {
			continue
		}
		peerUpper := strings.ToUpper(peer)
		if len(candidateUpper) > len(peerUpper)+1 &&
			strings.HasSuffix(candidateUpper, peerUpper) &&
			strings.TrimSpace(peer) != "" &&
			looksLikePrimaryAliasSymbol(peerUpper) &&
			looksLikePrimaryAliasSymbol(candidateUpper) {
			return -6
		}
	}
	return 0
}

func IsTrustedCandidate(value string) bool {
	return TrustedLabelScore(value) >= 12
}

func TrustedLabelScore(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	if LooksLikeECNumber(value) {
		return -100
	}
	if LooksLikeDatabaseIdentifier(value) {
		return -80
	}
	score := AliasPreferenceScore(value)
	hasDigit := false
	letterCount := 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			letterCount++
		}
	}
	switch lowerCount := lowercaseCount(value); {
	case lowerCount >= 3:
		score -= 16
	case lowerCount == 2:
		score -= 8
	case lowerCount == 1:
		score -= 2
	}
	if strings.ContainsAny(value, "._:/") {
		score -= 4
	}
	if strings.Contains(value, ".") {
		score -= strings.Count(value, ".") * 8
	}
	if strings.Contains(value, "'") {
		score += 6
	}
	if !hasDigit {
		switch {
		case letterCount > 8:
			score -= 24
		case letterCount > 6:
			score -= 14
		case letterCount > 4:
			score -= 6
		}
	}
	return score
}

func AliasPreferenceScore(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	score := 0
	hasLetter := false
	hasDigit := false
	upperCount := 0
	lowerCount := 0
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
			upperCount++
			score += 2
		case r >= 'a' && r <= 'z':
			hasLetter = true
			lowerCount++
			score += 1
		case r >= '0' && r <= '9':
			hasDigit = true
			score += 1
		case r == '-' || r == '\'':
			score += 1
		case r == '_' || r == '/' || r == '.':
			score -= 2
		case r == ' ' || r == '\t':
			score -= 8
		default:
			score -= 4
		}
	}
	upper := strings.ToUpper(value)
	switch {
	case strings.HasPrefix(upper, "CYP") && hasDigit:
		score -= 6
	case strings.HasPrefix(upper, "REF") && hasDigit:
		score -= 6
	}
	if hasLetter && hasDigit {
		score += 8
	}
	if strings.Contains(value, "'") {
		score += 8
	}
	if aliasHasInternalDigitPattern(value) {
		score += 6
	}
	if upperCount > 0 && lowerCount == 0 {
		score += 4
	}
	if len(value) <= 8 {
		score += 6
	} else if len(value) <= 12 {
		score += 2
	} else {
		score -= len(value) - 12
	}
	return score
}

func QueryAliasPrimarySymbolBonus(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	upper := strings.ToUpper(value)
	score := 0
	if looksLikePrimaryAliasSymbol(upper) {
		score += 30
	}
	return score
}

func LabelFromAutoDefine(value string) string {
	best := ""
	bestScore := -1
	for _, candidate := range AutoDefineCandidates(value) {
		score := AutoDefineLabelScore(candidate)
		if score > bestScore || (score == bestScore && len(candidate) < len(best)) {
			best = candidate
			bestScore = score
		}
	}
	return best
}

func AutoDefineCandidates(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '(' || r == ')' || r == ',' || r == ';' || r == '/' || r == '\t' || r == '\r' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !looksLikeAliasToken(part) {
			continue
		}
		key := strings.ToUpper(part)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, part)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := AliasPreferenceScore(out[i])
		right := AliasPreferenceScore(out[j])
		if left != right {
			return left > right
		}
		return len(out[i]) < len(out[j])
	})
	return out
}

func AutoDefineLabelScore(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	score := AliasPreferenceScore(value)
	if strings.Contains(value, "'") {
		score += 10
	}
	if len(value) <= 4 {
		score += 12
	} else if len(value) <= 6 {
		score += 8
	} else if len(value) <= 8 {
		score += 4
	} else {
		score -= len(value) - 8
	}
	upper := strings.ToUpper(value)
	if strings.HasPrefix(upper, "CYP") && len(value) > 6 {
		score -= 8
	}
	return score
}

func LooksLikeECNumber(value string) bool {
	return ecNumberLikePattern.MatchString(strings.TrimSpace(value))
}

func LooksLikeDatabaseIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return false
	case strings.HasPrefix(strings.ToUpper(value), "PAC:"):
		return true
	case lemnaGeneIDPattern.MatchString(value):
		return true
	case locusIDPattern.MatchString(value):
		return true
	default:
		return false
	}
}

func firstFastaHeaderLine(input string) string {
	value := strings.TrimSpace(input)
	if value == "" || !strings.HasPrefix(value, ">") {
		return ""
	}
	value = strings.ReplaceAll(value, "\r", "")
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			header := strings.TrimSpace(strings.TrimPrefix(line, ">"))
			if fastautil.IsIgnoredPHGONoteHeader(header) {
				continue
			}
			return header
		}
		return ""
	}
	return ""
}

func querySourceLabelPreferenceBonus(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	score := 0
	upper := strings.ToUpper(value)
	if looksLikePrimaryAliasSymbol(upper) {
		score += 30
	}
	return score
}

func looksLikePrimaryAliasSymbol(value string) bool {
	if value == "" {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func aliasHasInternalDigitPattern(value string) bool {
	if value == "" {
		return false
	}
	seenDigit := false
	seenLetterAfterDigit := false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			seenDigit = true
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			if seenDigit {
				seenLetterAfterDigit = true
			}
		}
	}
	if !seenLetterAfterDigit {
		return false
	}
	last := rune(value[len(value)-1])
	return last >= '0' && last <= '9'
}

func lowercaseCount(value string) int {
	count := 0
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			count++
		}
	}
	return count
}

func looksLikeAliasToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 16 {
		return false
	}
	hasLetter := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			hasLetter = true
		case r >= '0' && r <= '9', r == '-', r == '\'', r == '.':
		default:
			return false
		}
	}
	return hasLetter
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeAliasKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeAliasKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
