// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package sessionsnapshot

import (
	"archive/zip"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/phylo"
	"github.com/KiriKirby/phytozome-go/internal/tui"
)

func TestWriteReadKeywordSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyword")
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	in := Snapshot{
		Context: ContextV1{
			CreatedAt:          now,
			ApplicationName:    "phytozome GO",
			ApplicationVersion: "dev",
			Database:           "tair",
			Mode:               "family",
			ResultKind:         "keyword-result",
		},
		Keyword: &KeywordResultV1{
			SelectedSpecies: model.SpeciesCandidate{JBrowseName: "TAIR10", GenomeLabel: "TAIR10"},
			Groups: []model.KeywordSearchGroup{{
				SearchTerm: "PAL",
				Rows: []model.KeywordResultRow{{
					SourceDatabase: "tair",
					LabelName:      "PAL1",
					ProteinID:      "AT2G37040.1",
				}},
			}},
			Selected: []bool{true},
		},
		KeywordSource: &KeywordSourceStateV4{
			Database:     "ncbi",
			SourceKind:   "keyword",
			Engine:       "ncbi-eutilities-keyword",
			ResultDomain: "variant-clinical",
			SearchTypes:  []string{"NCBI protein accession"},
			Terms:        []string{"XP_015650724.1"},
			NCBI: &NCBIKeywordSourceV4{
				EntrezDatabase:       "clinvar",
				RecordType:           "clinvar",
				EUtilitiesBaseURL:    "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
				EngineSchema:         "ncbi-eutilities-keyword-v4",
				Accessions:           []string{"XP_015650724.1"},
				UIDs:                 []string{"123"},
				RequestedIDs:         []string{"RCV000000001"},
				ReplacementTargets:   []string{"RCV000000999"},
				ReplacementDecisions: []string{"new"},
				LinkResolution:       "elink",
				LinkedFromDB:         "gene",
				LinkedToDB:           "clinvar",
				LinkedFromTypes:      []string{"gene"},
				LinkedToTypes:        []string{"clinvar"},
				LinkNames:            []string{"gene_clinvar"},
				LinkSourceIDs:        []string{"4345054"},
				LinkTargetIDs:        []string{"123"},
				SearchTypeIDs:        []string{"clinvar"},
				ResultDomains:        []string{"variant-clinical"},
				GroupSearchTypes:     []string{"NCBI clinvar accession"},
			},
		},
		KeywordReview: &KeywordReviewStateV1{
			SelectionState: tui.RowSelectionState{Valid: true, SelectedRow: 3, SelectedColumn: 2},
		},
		SequenceCache: &SequenceCacheV1{
			Entries: []SequenceCacheEntryV1{{
				TargetID:       370201,
				SequenceID:     "AT2G37040.1",
				Sequence:       "MSTNPKPQR",
				OriginalHeader: ">AT2G37040.1",
			}},
		},
		ExportSettings: &ExportSettingsV2{
			BaseName:  "keyword",
			OutputDir: dir,
			Prompt: PromptExportSettingsV2{
				BaseName:        "keyword",
				WriteExcel:      true,
				WriteSession:    true,
				FastaHeaderMode: model.FastaHeaderModePhgo,
				UsePhgoHeader:   true,
			},
		},
	}
	if err := WriteFile(path, in); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if out.Context.FormatName != FormatName || out.Context.FormatVersion != FormatVersion {
		t.Fatalf("unexpected format context: %#v", out.Context)
	}
	if out.Keyword == nil || len(out.Keyword.Groups) != 1 || out.Keyword.Groups[0].Rows[0].LabelName != "PAL1" {
		t.Fatalf("keyword module did not round-trip: %#v", out.Keyword)
	}
	if out.KeywordSource == nil || out.KeywordSource.NCBI == nil || out.KeywordSource.NCBI.RecordType != "clinvar" {
		t.Fatalf("keyword source module did not round-trip: %#v", out.KeywordSource)
	}
	if out.KeywordSource.NCBI.LinkResolution != "elink" || len(out.KeywordSource.NCBI.LinkNames) != 1 || out.KeywordSource.NCBI.LinkNames[0] != "gene_clinvar" {
		t.Fatalf("keyword source link provenance did not round-trip: %#v", out.KeywordSource.NCBI)
	}
	if len(out.KeywordSource.NCBI.ReplacementTargets) != 1 || out.KeywordSource.NCBI.ReplacementTargets[0] != "RCV000000999" {
		t.Fatalf("keyword source replacement state did not round-trip: %#v", out.KeywordSource.NCBI)
	}
	if out.KeywordReview == nil || !out.KeywordReview.SelectionState.Valid || out.KeywordReview.SelectionState.SelectedRow != 3 {
		t.Fatalf("keyword review state did not round-trip: %#v", out.KeywordReview)
	}
	if out.SequenceCache == nil || len(out.SequenceCache.Entries) != 1 || out.SequenceCache.Entries[0].Sequence != "MSTNPKPQR" {
		t.Fatalf("sequence cache did not round-trip: %#v", out.SequenceCache)
	}
	if out.ExportSettings == nil || out.ExportSettings.Prompt.BaseName != "keyword" || !out.ExportSettings.Prompt.WriteExcel {
		t.Fatalf("export settings did not round-trip: %#v", out.ExportSettings)
	}

	reader, err := zip.OpenReader(path + FileExtension)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	seenManifest := false
	seenModule := false
	seenSourceModule := false
	for _, file := range reader.File {
		if file.Name == "manifest.xml" {
			seenManifest = true
		}
		if file.Name == "modules/keyword-result-v2.xml" {
			seenModule = true
		}
		if file.Name == "modules/keyword-source-state-v4.xml" {
			seenSourceModule = true
		}
	}
	if !seenManifest || !seenModule || !seenSourceModule {
		t.Fatalf("missing XML archive entries: manifest=%t keyword=%t keywordSource=%t", seenManifest, seenModule, seenSourceModule)
	}
}

func TestSnapshotSanitizesUnauthenticatedSourceWebURLs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source-url")
	authentic := model.KeywordResultRow{
		SourceDatabase: "phytozome",
		LabelName:      "AT1G01010",
		GeneReportURL:  "https://phytozome-next.jgi.doe.gov/report/gene/Athaliana_TAIR10/AT1G01010",
	}
	model.SetInputSourcePageURL(&authentic, authentic.GeneReportURL)
	synthetic := model.KeywordResultRow{
		SourceDatabase: "lemna",
		LabelName:      "Sp9509d020g000340",
		GeneReportURL:  "https://www.lemna.org/jbrowse2/?phgo_gene=Sp9509d020g000340&phgo_root=Sp_polyrhiza_9509",
	}
	blastSynthetic := model.BlastResultRow{
		SourceDatabase: "phytozome",
		Protein:        "Glyma.17G064400.1.p",
		GeneReportURL:  "https://phytozome-next.jgi.doe.gov/report/protein/G_max_Wm82_a6_v1/Glyma.17G064400.1.p",
	}
	in := Snapshot{
		Context: ContextV1{
			CreatedAt:          time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
			Database:           "phytozome",
			Mode:               "blast",
			ResultKind:         "blast-result",
			ApplicationName:    "phytozome GO",
			ApplicationVersion: "dev",
		},
		Keyword: &KeywordResultV1{
			Groups: []model.KeywordSearchGroup{{
				Rows: []model.KeywordResultRow{authentic, synthetic},
			}},
		},
		Blast: &BlastResultV1{
			Runs: []BlastRunV1{{
				Results: model.BlastResult{Rows: []model.BlastResultRow{blastSynthetic}},
			}},
		},
	}
	if err := WriteFile(path, in); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	rows := out.Keyword.Groups[0].Rows
	if rows[0].GeneReportURL == "" {
		t.Fatalf("authentic input URL was not preserved")
	}
	if rows[1].GeneReportURL != "" {
		t.Fatalf("synthetic keyword URL survived sanitization: %q", rows[1].GeneReportURL)
	}
	if got := out.Blast.Runs[0].Results.Rows[0].GeneReportURL; got != "" {
		t.Fatalf("synthetic BLAST URL survived sanitization: %q", got)
	}
}

func TestWriteReadBlastSnapshotPreservesOriginalRunCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blast")
	if err := WriteFile(path, Snapshot{
		Context: ContextV1{CreatedAt: time.Now()},
		Blast: &BlastResultV1{
			SelectedSpecies:  model.SpeciesCandidate{JBrowseName: "legacy"},
			OriginalRunCount: 4,
			Runs: []BlastRunV1{{
				Index:   1,
				Results: model.BlastResult{Rows: []model.BlastResultRow{{Protein: "legacy.1"}}},
			}},
		},
		ExternalReferences: &ExternalReferenceSettingsV2{
			AutoLabelBlastHits: true,
			UseUniProt:         true,
			UseInterPro:        true,
			InterProSettings:   model.DefaultInterProConservedRegionSettings(),
		},
	}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if out.Blast == nil || out.Blast.OriginalRunCount != 4 {
		t.Fatalf("blast snapshot original run count not preserved: %#v", out.Blast)
	}
	if out.ExternalReferences == nil || !out.ExternalReferences.UseInterPro {
		t.Fatalf("external references not preserved: %#v", out.ExternalReferences)
	}
}

func TestReadFileKeepsOldSnapshotsWithoutNewModules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy")
	if err := WriteFile(path, Snapshot{
		Context: ContextV1{CreatedAt: time.Now()},
		Blast: &BlastResultV1{
			SelectedSpecies: model.SpeciesCandidate{JBrowseName: "legacy"},
			Runs: []BlastRunV1{{
				Index: 1,
				Results: model.BlastResult{Rows: []model.BlastResultRow{{
					SourceDatabase: "phytozome",
					Protein:        "legacy.1",
				}}},
			}},
		},
	}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if out.Blast == nil || out.BlastReview != nil || out.SequenceCache != nil {
		t.Fatalf("legacy snapshot should load without optional modules: %#v", out)
	}
}

func TestResolveOpenPathUsesOutputDirAndOptionalExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.pgo")
	if err := WriteFile(path, Snapshot{Context: ContextV1{CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	resolved, err := ResolveOpenPath("saved", dir)
	if err != nil {
		t.Fatalf("ResolveOpenPath returned error: %v", err)
	}
	if resolved != path {
		t.Fatalf("resolved path = %q, want %q", resolved, path)
	}
}

func TestWriteReadSnapshotArtifactsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact")
	snapshot := Snapshot{
		Context: ContextV1{CreatedAt: time.Now()},
		Artifacts: &ArtifactManifestV2{
			Entries: []ArtifactEntryV2{{
				ID:          "artifacts/output/test.bin",
				Path:        "artifacts/output/test.bin",
				Kind:        "generated-output",
				MediaType:   "application/octet-stream",
				Description: "test payload",
			}},
		},
		ArtifactPayloads: map[string][]byte{
			"artifacts/output/test.bin": {0x00, 0x01, 0x02, 0x03},
		},
	}
	if err := WriteFile(path, snapshot); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if out.Artifacts == nil || len(out.Artifacts.Entries) != 1 {
		t.Fatalf("artifact manifest did not round-trip: %#v", out.Artifacts)
	}
	got := out.ArtifactPayloads["artifacts/output/test.bin"]
	if len(got) != 4 || got[0] != 0x00 || got[3] != 0x03 {
		t.Fatalf("artifact payload did not round-trip: %#v", got)
	}
}

func TestWriteReadCanvasSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "canvas")
	snapshot := Snapshot{
		Context: ContextV1{
			CreatedAt: time.Now(),
			Database:  "tair",
			Mode:      "canvas",
		},
		Canvas: &CanvasResultV1{
			Items: []CanvasItemV1{{
				Title:        "group 1",
				Subtitle:     "2/2 lines",
				Kind:         model.CanvasKindKeyword,
				SourceLabel:  "PAL family",
				ImportedFrom: "snapshot",
				ActiveColumns: []model.CanvasColumn{
					{ID: "search_term", Header: "search_tern"},
					{ID: "description", Header: "discripition"},
				},
				Rows: []CanvasRowV1{
					{Kind: model.CanvasKindKeyword, DisplayName: "PAL1", DisplayNameLocked: true, KeywordRow: &model.KeywordResultRow{LabelName: "PAL1", ProteinID: "AT2G37040.1"}},
					{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{LabelName: "query1", Annotation: ">query1\nMSTNPKPQR"}},
				},
				Selected: []bool{true, false},
				MSAFlags: []bool{false, true},
			}},
			CurrentItem:   0,
			NextNumericID: 3,
			ImportedFrom:  "demo.pgo",
			Tree: &CanvasTreeV2{
				PanelState: tui.CanvasTreePanelState{
					EnabledEver:            true,
					Expanded:               true,
					CurrentControl:         0,
					DisplayNameSource:      "label_name",
					ConversionTarget:       string(phylo.ConversionTargetProtein),
					ConversionSkipUnselect: true,
					AlignmentMethod:        "muscle",
					TreeMethod:             "neighbor_joining",
				},
				LastPayload: phylo.ViewerPayload{
					SchemaVersion: 1,
					SessionID:     "canvas",
					UpdatedAt:     time.Now(),
					Newick:        "(PHGOT000001,PHGOT000002);",
				},
				ViewerState: json.RawMessage(`{"schema_version":2,"reactree":{"layout":"circular","fontFamily":"Georgia"}}`),
				LastManifest: phylo.RunManifest{
					SchemaVersion: 1,
					Settings:      phylo.DefaultTreeSettings(),
				},
				LastArtifactDir:  filepath.Join(dir, "tree", "canvas", "run"),
				LastRunID:        "run",
				LastAlignedFASTA: ">PHGOT000001\nMPEP\n",
				LastNewick:       "(PHGOT000001,PHGOT000002);",
				Fingerprints:     phylo.Fingerprints{Alignment: "a", Tree: "t", Preview: "p"},
				ArtifactPaths:    []string{"artifacts/tree/canvas/run/tree.nwk"},
			},
		},
		CanvasMSA: &CanvasMSAV1{
			State: phylo.MSAState{
				SchemaVersion: 1,
				UpdatedAt:     time.Now(),
				Rows: []phylo.MSASelectionRow{
					{TaxonID: "PHGOT000001", DisplayName: "PAL1", Index: 0, State: "green"},
					{TaxonID: "PHGOT000002", DisplayName: "query1", Index: 1, State: "yellow"},
				},
				Settings:    map[string]any{"wrap": false},
				Colours:     map[string]any{"text": "#111111"},
				Groups:      []map[string]any{{"name": "manual group", "start": float64(1), "end": float64(3)}},
				Annotations: []map[string]any{{"label": "score", "visible": true}},
				Features:    []map[string]any{{"taxon_id": "PHGOT000001", "type": "domain", "begin": float64(1), "end": float64(3)}},
				Markers:     []map[string]any{{"name": "bookmark", "column": float64(2)}},
			},
			LastPayload: phylo.ViewerPayload{
				SchemaVersion: 1,
				SessionID:     "canvas",
				UpdatedAt:     time.Now(),
				AlignedFASTA:  ">PHGOT000001\nMPEP\n>PHGOT000002\nMSEQ\n",
			},
			LastAlignedFASTA: ">PHGOT000001\nMPEP\n>PHGOT000002\nMSEQ\n",
		},
		SequenceCache: &SequenceCacheV1{
			Entries: []SequenceCacheEntryV1{{
				TargetID:       370201,
				SequenceID:     "AT2G37040.1",
				Sequence:       "MSTNPKPQR",
				OriginalHeader: ">AT2G37040.1",
			}},
		},
		CanvasReview: &CanvasReviewStateV1{
			SelectionState: tui.BlastRunSelectionState{
				Valid:       true,
				CurrentRun:  0,
				ControlMode: 2,
			},
		},
	}
	if err := WriteFile(path, snapshot); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if out.Canvas == nil || len(out.Canvas.Items) != 1 {
		t.Fatalf("canvas module did not round-trip: %#v", out.Canvas)
	}
	if out.Canvas.Items[0].Title != "group 1" || len(out.Canvas.Items[0].Rows) != 2 {
		t.Fatalf("canvas item payload mismatch: %#v", out.Canvas.Items[0])
	}
	if out.Canvas.Items[0].Rows[0].DisplayName != "PAL1" {
		t.Fatalf("canvas display name did not round-trip: %#v", out.Canvas.Items[0].Rows[0])
	}
	if !out.Canvas.Items[0].Rows[0].DisplayNameLocked {
		t.Fatalf("canvas display-name lock did not round-trip: %#v", out.Canvas.Items[0].Rows[0])
	}
	if out.Canvas.Items[0].SourceLabel != "PAL family" || out.Canvas.Items[0].ImportedFrom != "snapshot" {
		t.Fatalf("canvas item metadata mismatch: %#v", out.Canvas.Items[0])
	}
	if len(out.Canvas.Items[0].ActiveColumns) == 0 {
		t.Fatalf("canvas active columns should round-trip: %#v", out.Canvas.Items[0])
	}
	if len(out.Canvas.Items[0].MSAFlags) != 2 || !out.Canvas.Items[0].MSAFlags[1] {
		t.Fatalf("canvas MSA flags should round-trip: %#v", out.Canvas.Items[0].MSAFlags)
	}
	if out.Canvas.Items[0].Rows[0].RowNumber != 0 || out.Canvas.Items[0].Rows[1].RowNumber != 0 {
		t.Fatalf("canvas row number payload mismatch: %#v", out.Canvas.Items[0].Rows)
	}
	if out.Canvas.Tree == nil || out.Canvas.Tree.LastPayload.Newick == "" || out.Canvas.Tree.PanelState.AlignmentMethod != "muscle" {
		t.Fatalf("canvas tree state did not round-trip: %#v", out.Canvas.Tree)
	}
	if string(out.Canvas.Tree.ViewerState) != `{"schema_version":2,"reactree":{"layout":"circular","fontFamily":"Georgia"}}` {
		t.Fatalf("canvas tree viewer state mismatch: %s", out.Canvas.Tree.ViewerState)
	}
	if out.Canvas.Tree.LastManifest.SchemaVersion != 1 || out.Canvas.Tree.Fingerprints.Tree != "t" {
		t.Fatalf("canvas tree manifest/fingerprints mismatch: %#v", out.Canvas.Tree)
	}
	if out.CanvasMSA == nil || len(out.CanvasMSA.State.Rows) != 2 || out.CanvasMSA.State.Rows[1].State != "yellow" {
		t.Fatalf("canvas MSA state did not round-trip: %#v", out.CanvasMSA)
	}
	if len(out.CanvasMSA.State.Groups) != 1 || out.CanvasMSA.LastPayload.AlignedFASTA == "" {
		t.Fatalf("canvas MSA groups/payload did not round-trip: %#v", out.CanvasMSA)
	}
	if out.CanvasMSA.State.Colours["text"] != "#111111" || len(out.CanvasMSA.State.Annotations) != 1 || len(out.CanvasMSA.State.Features) != 1 || len(out.CanvasMSA.State.Markers) != 1 {
		t.Fatalf("canvas MSA durable state did not round-trip completely: %#v", out.CanvasMSA.State)
	}
	if out.CanvasReview == nil || !out.CanvasReview.SelectionState.Valid || out.CanvasReview.SelectionState.ControlMode != 2 {
		t.Fatalf("canvas review state did not round-trip: %#v", out.CanvasReview)
	}
	if out.SequenceCache == nil || len(out.SequenceCache.Entries) != 1 || out.SequenceCache.Entries[0].SequenceID != "AT2G37040.1" {
		t.Fatalf("canvas sequence cache did not round-trip: %#v", out.SequenceCache)
	}
}

func TestOlderSnapshotVersionKeepsLegacyTreeFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old-tree")
	snapshot := Snapshot{
		Context: ContextV2{
			CreatedAt:     time.Now(),
			FormatName:    FormatName,
			FormatVersion: "2.6",
			Database:      "tair",
			Mode:          "canvas",
			ResultKind:    "canvas-result",
		},
		Canvas: &CanvasResultV2{
			Tree: &CanvasTreeV2{
				PanelState: tui.CanvasTreePanelState{
					EnabledEver:      true,
					ConversionTarget: string(phylo.ConversionTargetProtein),
					AlignmentMethod:  string(phylo.AlignmentMUSCLE),
					AlignmentParams:  map[string]string{"legacy_alignment_param": "stale"},
					TreeMethod:       string(phylo.TreeMaximumLikelihood),
					TreeParams:       map[string]string{"legacy_tree_param": "stale"},
				},
			},
		},
	}
	if err := WriteFile(path, snapshot); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	out, err := ReadFile(path + FileExtension)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !IsLegacyTreeSnapshot(out) {
		t.Fatalf("snapshots below %s must stay legacy so tree params are reset to current MEGA defaults", FormatVersion)
	}
}
