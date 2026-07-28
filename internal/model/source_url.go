// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package model

import (
	"net/url"
	"strings"
)

const (
	SourcePageURLProvenanceKey      = "source_page_url_provenance"
	SourcePageURLProvenanceInput    = "input_report_url"
	SourcePageURLProvenanceResponse = "source_response_url"
)

func NormalizeProvidedSourcePageURL(value string) string {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + strings.TrimPrefix(value, "//")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return ""
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(strings.TrimSpace(parsed.Host))
	return parsed.String()
}

func MarkSourcePageURLProvenance(extra map[string]string, provenance string) map[string]string {
	provenance = strings.TrimSpace(provenance)
	if provenance == "" {
		return extra
	}
	if extra == nil {
		extra = map[string]string{}
	}
	extra[SourcePageURLProvenanceKey] = provenance
	return extra
}

func SourcePageURLHasProvenance(extra map[string]string) bool {
	switch strings.TrimSpace(extra[SourcePageURLProvenanceKey]) {
	case SourcePageURLProvenanceInput, SourcePageURLProvenanceResponse:
		return true
	default:
		return false
	}
}

func SanitizeKeywordResultRowSourceWebURL(row *KeywordResultRow) {
	if row == nil {
		return
	}
	if strings.TrimSpace(row.GeneReportURL) == "" {
		return
	}
	if !SourcePageURLHasProvenance(row.ExtraColumns) {
		row.GeneReportURL = ""
	}
}

func SanitizeKeywordResultRowsSourceWebURLs(rows []KeywordResultRow) {
	for i := range rows {
		SanitizeKeywordResultRowSourceWebURL(&rows[i])
	}
}

func SanitizeBlastResultRowsSourceWebURLs(rows []BlastResultRow) {
	for i := range rows {
		rows[i].GeneReportURL = ""
	}
}

func SetInputSourcePageURL(row *KeywordResultRow, rawURL string) {
	if row == nil {
		return
	}
	normalized := NormalizeProvidedSourcePageURL(rawURL)
	if normalized == "" {
		return
	}
	row.GeneReportURL = normalized
	row.ExtraColumns = MarkSourcePageURLProvenance(row.ExtraColumns, SourcePageURLProvenanceInput)
}
