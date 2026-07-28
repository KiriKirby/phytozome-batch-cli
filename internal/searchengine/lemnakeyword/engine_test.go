// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package lemnakeyword

import (
	"context"
	"strings"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

type fakeFinder struct {
	reportRows  map[string][]model.KeywordResultRow
	idRows      map[string][]model.KeywordResultRow
	labelRows   map[string][]model.KeywordResultRow
	keywordRows map[string][]model.KeywordResultRow
	wideRows    map[string][]model.KeywordResultRow
	broadRows   map[string][]model.KeywordResultRow
}

func (f *fakeFinder) SearchKeywordRowsByReportURL(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.reportRows[strings.ToUpper(term)]), nil
}

func (f *fakeFinder) SearchKeywordRowsByIdentifier(ctx context.Context, species model.SpeciesCandidate, term string, kind string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.idRows[strings.ToUpper(kind+"|"+term)]), nil
}

func (f *fakeFinder) SearchKeywordRowsByLabel(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.labelRows[strings.ToUpper(term)]), nil
}

func (f *fakeFinder) SearchKeywordRowsByKeywordText(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.keywordRows[strings.ToUpper(term)]), nil
}

func (f *fakeFinder) SearchKeywordRowsByWideText(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.wideRows[strings.ToUpper(term)]), nil
}

func (f *fakeFinder) SearchKeywordRowsByBroadText(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return cloneRows(f.broadRows[strings.ToUpper(term)]), nil
}

func TestEngineMapsLemnaProgramsWithoutCuratedRedirects(t *testing.T) {
	finder := &fakeFinder{
		reportRows: map[string][]model.KeywordResultRow{
			"HTTPS://WWW.LEMNA.ORG/JBROWSE2/?ASSEMBLY=SP9509D&CONFIG=HTTPS%3A%2F%2FWWW.LEMNA.ORG%2FJBROWSE2%2FCONFIG.JSON&FILTER=SP9509D020G000340&HIGHLIGHT=SP9509D020G000340&PHGO_GENE=SP9509D020G000340&PHGO_ROOT=SP_POLYRHIZA_9509": {{TranscriptID: "Sp9509d020g000340_T001", LabelName: "C4H"}},
		},
		idRows: map[string][]model.KeywordResultRow{
			"TRANSCRIPT|SP9509D020G000340_T001": {{TranscriptID: "Sp9509d020g000340_T001", LabelName: "C4H"}},
			"GENE|SP9509D020G000340":            {{GeneIdentifier: "Sp9509d020g000340", LabelName: "C4H"}},
		},
		labelRows: map[string][]model.KeywordResultRow{
			"C4H": {{TranscriptID: "Sp9509d020g000340_T001", LabelName: "C4H"}},
		},
		keywordRows: map[string][]model.KeywordResultRow{
			"PHENYLALANINE AMMONIA LYASE": {{TranscriptID: "Sp9509d011g008180_T004", LabelName: "PAL1"}},
		},
	}
	engine := New(finder)
	species := model.SpeciesCandidate{JBrowseName: "Sp_polyrhiza_9509"}

	tests := []struct {
		term       string
		searchType string
		label      string
	}{
		{"https://www.lemna.org/jbrowse2/?assembly=Sp9509d&config=https%3A%2F%2Fwww.lemna.org%2Fjbrowse2%2Fconfig.json&filter=Sp9509d020g000340&highlight=Sp9509d020g000340&phgo_gene=Sp9509d020g000340&phgo_root=Sp_polyrhiza_9509", SearchTypeReportURL, "C4H"},
		{"Sp9509d020g000340_T001", SearchTypeTranscriptID, "C4H"},
		{"Sp9509d020g000340", SearchTypeGeneID, "C4H"},
		{"C4H", SearchTypeLabelSymbol, "C4H"},
		{"phenylalanine ammonia lyase", SearchTypeKeyword, "PAL1"},
	}
	for _, tt := range tests {
		rows, err := engine.SearchKeywordRows(context.Background(), species, tt.term)
		if err != nil {
			t.Fatalf("%s returned error: %v", tt.term, err)
		}
		if len(rows) != 1 {
			t.Fatalf("%s rows = %d, want 1", tt.term, len(rows))
		}
		if rows[0].SearchType != tt.searchType {
			t.Fatalf("%s search type = %q, want %q", tt.term, rows[0].SearchType, tt.searchType)
		}
		if rows[0].LabelName != tt.label {
			t.Fatalf("%s label = %q, want %q", tt.term, rows[0].LabelName, tt.label)
		}
	}
}

func TestEngineDoesNotRedirectHardcodedRiceTerms(t *testing.T) {
	finder := &fakeFinder{
		idRows: map[string][]model.KeywordResultRow{
			"ANY|SP9509D020G000340_T001": {{TranscriptID: "Sp9509d020g000340_T001", LabelName: "C4H"}},
		},
		keywordRows: map[string][]model.KeywordResultRow{
			"TRANS-CINNAMATE 4-MONOOXYGENASE": {{TranscriptID: "keyword-only", LabelName: "raw"}},
		},
	}
	engine := New(finder)
	for _, term := range []string{"LOC_Os05g25640", "XP_015639656", "OsC4H1", "CYP73A38"} {
		rows, err := engine.SearchKeywordRows(context.Background(), model.SpeciesCandidate{ProteomeID: 2026051201}, term)
		if err != nil {
			t.Fatalf("%s returned error: %v", term, err)
		}
		if len(rows) != 0 {
			t.Fatalf("%s should not be hardcoded to another Lemna row, got %#v", term, rows)
		}
	}
}

func TestEngineRecordsWideSearchFallback(t *testing.T) {
	finder := &fakeFinder{
		wideRows: map[string][]model.KeywordResultRow{
			"PHENYLALANINE BROAD": {{TranscriptID: "Sp9509d011g008180_T004", LabelName: "PAL1"}},
		},
	}
	engine := New(finder)

	rows, err := engine.SearchKeywordRows(context.Background(), model.SpeciesCandidate{ProteomeID: 2026051201, JBrowseName: "test-wide"}, "phenylalanine broad")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if !strings.Contains(rows[0].SearchType, "fallback to wide search") {
		t.Fatalf("search type should record wide fallback, got %q", rows[0].SearchType)
	}
}

func TestEngineCanForceWideSearch(t *testing.T) {
	finder := &fakeFinder{
		wideRows: map[string][]model.KeywordResultRow{
			"PHENYLALANINE BROAD": {{TranscriptID: "Sp9509d011g008180_T004", LabelName: "PAL1"}},
		},
	}
	engine := New(finder)

	rows, err := engine.SearchKeywordRowsWide(context.Background(), model.SpeciesCandidate{ProteomeID: 2026051201, JBrowseName: "test-wide"}, "phenylalanine broad")
	if err != nil {
		t.Fatalf("SearchKeywordRowsWide returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].SearchType != SearchTypeWide {
		t.Fatalf("forced wide search type = %q, want %q", rows[0].SearchType, SearchTypeWide)
	}
	if rows[0].LabelName != "PAL1" {
		t.Fatalf("forced wide search should preserve label name, got %q", rows[0].LabelName)
	}
}

func TestEngineWideSearchCanUseBroadRows(t *testing.T) {
	finder := &fakeFinder{
		broadRows: map[string][]model.KeywordResultRow{
			"4CL WEB STYLE": {{TranscriptID: "Sp9509d011g008180_T004", LabelName: "4CL"}},
		},
	}
	engine := New(finder)

	rows, err := engine.SearchKeywordRowsWide(context.Background(), model.SpeciesCandidate{ProteomeID: 2026051201, JBrowseName: "test-wide"}, "4cl web style")
	if err != nil {
		t.Fatalf("SearchKeywordRowsWide returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].SearchType != SearchTypeWide {
		t.Fatalf("forced wide search type = %q, want %q", rows[0].SearchType, SearchTypeWide)
	}
	if rows[0].LabelName != "4CL" {
		t.Fatalf("forced wide search should use broad rows, got %q", rows[0].LabelName)
	}
}
