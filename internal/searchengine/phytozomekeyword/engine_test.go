// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package phytozomekeyword

import (
	"context"
	"strings"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

type fakeFinder struct {
	genesByID           map[string]GeneRecord
	genesByKeyword      map[string][]GeneRecord
	genesByBroadKeyword map[string][]GeneRecord
	keywordRequests     []string
}

func (f *fakeFinder) FetchGeneByGeneID(ctx context.Context, proteomeID int, geneID string) (GeneRecord, error) {
	if gene, ok := f.genesByID[strings.ToUpper(geneID)]; ok {
		return gene, nil
	}
	return GeneRecord{}, errNotFound{}
}

func (f *fakeFinder) FetchGeneByTranscript(ctx context.Context, proteomeID int, transcriptID string) (GeneRecord, error) {
	return f.FetchGeneByGeneID(ctx, proteomeID, transcriptID)
}

func (f *fakeFinder) FetchGeneByProtein(ctx context.Context, proteomeID int, proteinID string) (GeneRecord, error) {
	return f.FetchGeneByGeneID(ctx, proteomeID, proteinID)
}

func (f *fakeFinder) SearchGenesByKeyword(ctx context.Context, proteomeID int, keyword string, limit int) ([]GeneRecord, error) {
	f.keywordRequests = append(f.keywordRequests, keyword)
	return append([]GeneRecord(nil), f.genesByKeyword[strings.ToUpper(keyword)]...), nil
}

func (f *fakeFinder) SearchGenesByKeywordBroad(ctx context.Context, proteomeID int, keyword string, limit int) ([]GeneRecord, error) {
	return append([]GeneRecord(nil), f.genesByBroadKeyword[strings.ToUpper(keyword)]...), nil
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func TestEngineUsesDirectIdentifierLookupsWithoutCuratedRedirects(t *testing.T) {
	finder := &fakeFinder{genesByID: map[string]GeneRecord{
		"LOC_OS05G25640": testRiceGene("LOC_Os05g25640"),
		"XP_015639656":   testRiceGene("XP_015639656"),
	}}
	engine := New(finder)
	species := model.SpeciesCandidate{ProteomeID: 323, JBrowseName: "Osativa_v7_0"}

	tests := []struct {
		term       string
		searchType string
		gene       string
	}{
		{"LOC_Os05g25640", SearchTypePhytozomeID, "LOC_Os05g25640"},
		{"XP_015639656", SearchTypePhytozomeID, "XP_015639656"},
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
		if rows[0].GeneIdentifier != tt.gene {
			t.Fatalf("%s gene = %q, want %q", tt.term, rows[0].GeneIdentifier, tt.gene)
		}
	}
}

func TestEngineDoesNotRedirectHardcodedRiceAliases(t *testing.T) {
	finder := &fakeFinder{genesByID: map[string]GeneRecord{
		"LOC_OS05G25640": testRiceGene("LOC_Os05g25640"),
	}}
	engine := New(finder)
	for _, term := range []string{"OsC4H1", "CYP73A38", "Os4CL1"} {
		rows, err := engine.SearchKeywordRows(context.Background(), model.SpeciesCandidate{ProteomeID: 323}, term)
		if err != nil {
			t.Fatalf("%s returned error: %v", term, err)
		}
		if len(rows) != 0 {
			t.Fatalf("%s should not be hardcoded to another locus, got %#v", term, rows)
		}
	}
}

func TestEngineRecordsWideSearchFallback(t *testing.T) {
	gene := testRiceGene("LOC_Os05g25640")
	finder := &fakeFinder{
		genesByID:      map[string]GeneRecord{},
		genesByKeyword: map[string][]GeneRecord{"OS-C4H-ODD": {gene}},
	}
	engine := New(finder)
	rows, err := engine.SearchKeywordRows(context.Background(), model.SpeciesCandidate{ProteomeID: 323}, "Os-C4H-odd")
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
	gene := testRiceGene("LOC_Os05g25640")
	finder := &fakeFinder{
		genesByKeyword: map[string][]GeneRecord{
			"WIDE ONLY PHRASE 20260509": {gene},
		},
	}
	engine := New(finder)

	rows, err := engine.SearchKeywordRowsWide(context.Background(), model.SpeciesCandidate{ProteomeID: 99323}, "wide only phrase 20260509")
	if err != nil {
		t.Fatalf("SearchKeywordRowsWide returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].SearchType != SearchTypeWide {
		t.Fatalf("forced wide search type = %q, want %q", rows[0].SearchType, SearchTypeWide)
	}
	if rows[0].GeneIdentifier != "LOC_Os05g25640" {
		t.Fatalf("forced wide search should use wide keyword result, got %q", rows[0].GeneIdentifier)
	}
}

func TestEngineWideSearchUsesBroadKeywordFinder(t *testing.T) {
	gene := testRiceGene("LOC_Os01g60450")
	finder := &fakeFinder{
		genesByBroadKeyword: map[string][]GeneRecord{
			"4CL WEB STYLE": {gene},
		},
	}
	engine := New(finder)

	rows, err := engine.SearchKeywordRowsWide(context.Background(), model.SpeciesCandidate{ProteomeID: 323}, "4cl web style")
	if err != nil {
		t.Fatalf("SearchKeywordRowsWide returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].SearchType != SearchTypeWide {
		t.Fatalf("forced wide search type = %q, want %q", rows[0].SearchType, SearchTypeWide)
	}
	if rows[0].GeneIdentifier != "LOC_Os01g60450" {
		t.Fatalf("wide search should use broad keyword result, got %q", rows[0].GeneIdentifier)
	}
}

func testRiceGene(id string) GeneRecord {
	return GeneRecord{
		PrimaryIdentifier: id,
		Organism: GeneOrganismInfo{
			OrganismName:      "Oryza sativa",
			AnnotationVersion: "v7.0",
			Proteome:          323,
		},
		Transcripts: []GeneTranscript{{
			PrimaryIdentifier:   id + ".1",
			SecondaryIdentifier: "PAC:1",
			IsPrimary:           "1",
		}},
		Deflines: []string{"cytochrome P450, putative, expressed"},
	}
}
