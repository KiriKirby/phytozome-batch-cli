package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/phylo"
	"github.com/KiriKirby/phytozome-go/internal/sessionsnapshot"
	"github.com/KiriKirby/phytozome-go/internal/source"
	"github.com/KiriKirby/phytozome-go/internal/tui"
)

func TestSnapshotCanvasTreeArtifactsPacksRuntimeFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"input.fasta":           ">PHGOT000001\nMPEPTIDE\n",
		"input.meta.json":       `{"schema_version":1}`,
		"runtime-request.json":  `{"schema_version":1}`,
		"runtime-response.json": `{"schema_version":1}`,
		"aligned.fasta":         ">PHGOT000001\nMPEPTIDE\n",
		"tree.nwk":              "(PHGOT000001);",
		"viewer.payload.json":   `{"schema_version":1}`,
		"run.manifest.json":     `{"schema_version":1}`,
		"runtime.stdout.txt":    "ok",
		"runtime.stderr.txt":    "",
		"runtime-summary.txt":   "summary",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	manifest, payloads, err := snapshotCanvasTreeArtifacts(phylo.RunPlan{
		SessionID: "canvas session",
		RunID:     "run:1",
		BaseDir:   dir,
	})
	if err != nil {
		t.Fatalf("snapshotCanvasTreeArtifacts returned error: %v", err)
	}
	if manifest == nil || len(manifest.Entries) != len(files) {
		t.Fatalf("manifest entries = %#v, want %d", manifest, len(files))
	}
	for _, entry := range manifest.Entries {
		if !strings.HasPrefix(entry.Path, "artifacts/tree/canvas_session/run_1/") {
			t.Fatalf("unexpected archive path: %s", entry.Path)
		}
		if len(payloads[entry.Path]) == 0 && filepath.Base(entry.Path) != "runtime.stderr.txt" {
			t.Fatalf("missing payload for %s", entry.Path)
		}
		if entry.RestorePath == "" {
			t.Fatalf("missing restore path for %s", entry.Path)
		}
	}
}

func TestSnapshotCanvasTreeStateKeepsPanelWithoutRun(t *testing.T) {
	w := NewBlastWizard(nil)
	w.prompt.RestoreCanvasTreePanelState(canvasStateKey("canvas"), treePanelForSnapshotTest())
	tree, manifest, payloads, err := w.snapshotCanvasTreeState()
	if err != nil {
		t.Fatalf("snapshotCanvasTreeState returned error: %v", err)
	}
	if tree == nil || !tree.PanelState.EnabledEver || tree.PanelState.AlignmentMethod != string(phylo.AlignmentMUSCLE) {
		t.Fatalf("tree panel state not captured: %#v", tree)
	}
	if manifest != nil || payloads != nil {
		t.Fatalf("unexpected artifact bundle without a run: %#v %#v", manifest, payloads)
	}
}

func TestSnapshotCanvasTreeStateSyncsCurrentDisplayNamesWithoutRecompute(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	panel := treePanelForSnapshotTest()
	w.prompt.RestoreCanvasTreePanelState(canvasStateKey("canvas"), panel)
	record := phylo.InputRecord{
		TaxonID:      "PHGOT000001",
		DisplayName:  "Old PAL",
		SourceType:   string(model.CanvasKindFasta),
		OriginalHead: "Old PAL",
		CanvasItem:   "canvas 1",
		CanvasRow:    0,
		TableValues:  map[string]string{"label_name": "Old PAL", "head": "Old PAL"},
	}
	meta := phylo.Metadata{SchemaVersion: 1, GeneratedAt: now, DisplayNameSource: "label_name", Records: []phylo.InputRecord{record}}
	w.canvasTreeLastPayload = phylo.ViewerPayload{SchemaVersion: 1, SessionID: "canvas", UpdatedAt: now, Newick: "(PHGOT000001);", AlignedFASTA: ">PHGOT000001\nMPEPTIDE\n", Metadata: meta}
	w.canvasTreeLastPlan = phylo.RunPlan{
		SessionID:    "canvas",
		RunID:        "run1",
		BaseDir:      t.TempDir(),
		Settings:     treeSettingsFromSnapshotPanel(panel),
		Records:      []phylo.InputRecord{record},
		Metadata:     meta,
		AlignedFASTA: ">PHGOT000001\nMPEPTIDE\n",
		Newick:       "(PHGOT000001);",
		Fingerprints: phylo.Fingerprints{Alignment: "align", Tree: "tree", Preview: "old-preview"},
	}
	items := []model.CanvasItem{{
		Title:    "canvas 1",
		Selected: []bool{true},
		Rows: []model.CanvasRow{{
			Kind:              model.CanvasKindFasta,
			DisplayName:       "New PAL",
			DisplayNameLocked: true,
			FASTA:             &model.QuerySequenceSource{Annotation: "raw head", LabelName: "Old PAL", Sequence: "MPEPTIDE", SequenceKind: model.SequenceProtein},
		}},
	}}
	tree, _, _, err := w.snapshotCanvasTreeState(items)
	if err != nil {
		t.Fatalf("snapshotCanvasTreeState returned error: %v", err)
	}
	if tree.LastPayload.Metadata.Records[0].DisplayName != "New PAL" {
		t.Fatalf("snapshot payload did not sync latest display name: %#v", tree.LastPayload.Metadata.Records)
	}
	if tree.Fingerprints.Alignment != "align" || tree.Fingerprints.Tree != "tree" {
		t.Fatalf("compute fingerprints should not change after display-name sync: %#v", tree.Fingerprints)
	}
	if tree.Fingerprints.Preview == "old-preview" || tree.Fingerprints.Preview == "" {
		t.Fatalf("preview fingerprint should update after display-name sync: %#v", tree.Fingerprints)
	}
}

func TestSnapshotCanvasTreeStateUsesVisibleCanvasSortOrder(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	panel := treePanelForSnapshotTest()
	w.prompt.RestoreCanvasTreePanelState(canvasStateKey("canvas"), panel)
	w.prompt.RestoreCanvasReviewState(canvasStateKey("canvas"), tui.BlastRunSelectionState{
		Valid: true,
		Sort:  tui.TableSort{Column: 1, Direction: tui.SortAscending},
	})
	records := []phylo.InputRecord{
		{
			TaxonID:      "PHGOT000001",
			DisplayName:  "beta",
			SourceType:   string(model.CanvasKindFasta),
			OriginalHead: "beta",
			CanvasItem:   "canvas 1",
			CanvasRow:    0,
			TableValues:  map[string]string{"label_name": "beta", "head": "beta"},
		},
		{
			TaxonID:      "PHGOT000002",
			DisplayName:  "alpha",
			SourceType:   string(model.CanvasKindFasta),
			OriginalHead: "alpha",
			CanvasItem:   "canvas 1",
			CanvasRow:    1,
			TableValues:  map[string]string{"label_name": "alpha", "head": "alpha"},
		},
	}
	meta := phylo.Metadata{SchemaVersion: 1, GeneratedAt: now, DisplayNameSource: "label_name", Records: records}
	w.canvasTreeLastPayload = phylo.ViewerPayload{
		SchemaVersion: 1,
		SessionID:     "canvas",
		UpdatedAt:     now,
		Newick:        "(PHGOT000001,PHGOT000002);",
		AlignedFASTA:  ">PHGOT000001\nBBB\n>PHGOT000002\nAAA\n",
		Metadata:      meta,
	}
	w.canvasTreeLastPlan = phylo.RunPlan{
		SessionID:    "canvas",
		RunID:        "run1",
		BaseDir:      t.TempDir(),
		Settings:     treeSettingsFromSnapshotPanel(panel),
		Records:      records,
		Metadata:     meta,
		AlignedFASTA: ">PHGOT000001\nBBB\n>PHGOT000002\nAAA\n",
		Newick:       "(PHGOT000001,PHGOT000002);",
		Fingerprints: phylo.Fingerprints{Alignment: "align", Tree: "tree", Preview: "old-preview"},
	}
	items := []model.CanvasItem{{
		Title:    "canvas 1",
		Selected: []bool{true, true},
		Rows: []model.CanvasRow{
			{
				RowNumber: 7,
				Kind:      model.CanvasKindFasta,
				FASTA:     &model.QuerySequenceSource{Annotation: "beta", LabelName: "beta", Sequence: "BBB", SequenceKind: model.SequenceProtein},
			},
			{
				RowNumber: 3,
				Kind:      model.CanvasKindFasta,
				FASTA:     &model.QuerySequenceSource{Annotation: "alpha", LabelName: "alpha", Sequence: "AAA", SequenceKind: model.SequenceProtein},
			},
		},
	}}
	tree, _, _, err := w.snapshotCanvasTreeState(items)
	if err != nil {
		t.Fatalf("snapshotCanvasTreeState returned error: %v", err)
	}
	if len(tree.LastPayload.Metadata.Records) != 2 {
		t.Fatalf("snapshot payload records = %#v, want 2", tree.LastPayload.Metadata.Records)
	}
	if tree.LastPayload.Metadata.Records[0].DisplayName != "alpha" || tree.LastPayload.Metadata.Records[1].DisplayName != "beta" {
		t.Fatalf("snapshot payload order should follow visible canvas sort: %#v", tree.LastPayload.Metadata.Records)
	}
}

func TestCloseCanvasTreeViewerStopsCanvasScopedServer(t *testing.T) {
	w := NewBlastWizard(nil)
	w.instanceID = "canvas-viewer-close-test"

	server, url, err := w.ensureCanvasTreeViewer(context.Background())
	if err != nil {
		t.Fatalf("ensureCanvasTreeViewer returned error: %v", err)
	}
	if server == nil || strings.TrimSpace(url) == "" {
		t.Fatalf("viewer server not initialized: %#v %q", server, url)
	}
	if _, err := http.Get(server.URL() + "/health"); err != nil {
		t.Fatalf("viewer health before close failed: %v", err)
	}

	w.closeCanvasTreeViewer()

	if w.canvasTreeViewer != nil || w.canvasTreeViewerCancel != nil {
		t.Fatalf("viewer fields should be cleared after close")
	}
	if !w.canvasTreeLastPayload.UpdatedAt.IsZero() || strings.TrimSpace(w.canvasTreeLastPayload.Newick) != "" {
		t.Fatalf("viewer payload should be cleared after close: %#v", w.canvasTreeLastPayload)
	}
	if strings.TrimSpace(w.canvasTreeLastPlan.BaseDir) != "" || strings.TrimSpace(w.canvasTreeLastPlan.SessionID) != "" {
		t.Fatalf("viewer plan should be cleared after close: %#v", w.canvasTreeLastPlan)
	}

	var lastErr error
	for range 20 {
		_, lastErr = http.Get(server.URL() + "/health")
		if lastErr != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("viewer health still responded after close; lastErr=%v", lastErr)
}

func TestReuseLastCanvasTreePlanDoesNotBiologicallyValidateAlignment(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	settings := phylo.DefaultTreeSettings()
	records := []phylo.InputRecord{
		{TaxonID: "PHGOT000001", DisplayName: "DNA row", Sequence: "ATGCGTATGCGT", SequenceKind: phylo.SequenceNucleotide, RowFingerprint: "dna"},
		{TaxonID: "PHGOT000002", DisplayName: "Protein row", Sequence: "MPEPTIDE", SequenceKind: phylo.SequenceProtein, RowFingerprint: "protein"},
	}
	meta := phylo.Metadata{SchemaVersion: 1, GeneratedAt: now, DisplayNameSource: "label_name", Records: records}
	staleAligned := ">PHGOT000001\nATGCGTATGCGT\n>PHGOT000002\nMPEPTIDE\n"
	last, err := phylo.BuildRunPlan("canvas", "old", t.TempDir(), settings, phylo.SequenceProtein, records, meta, staleAligned, "(PHGOT000001,PHGOT000002);", now)
	if err != nil {
		t.Fatalf("BuildRunPlan last: %v", err)
	}
	w.canvasTreeLastPlan = last
	plan, err := phylo.BuildRunPlan("canvas", "new", t.TempDir(), settings, phylo.SequenceProtein, records, meta, "", "", now)
	if err != nil {
		t.Fatalf("BuildRunPlan candidate: %v", err)
	}
	reused, ok, err := w.reuseLastCanvasTreePlan(plan, records, meta, now)
	if err != nil {
		t.Fatalf("reuseLastCanvasTreePlan returned error: %v", err)
	}
	if !ok {
		t.Fatalf("matching runtime artifacts should be reused without Go-side biological validation")
	}
	if !strings.Contains(reused.Plan.AlignedFASTA, "ATGCGTATGCGT") {
		t.Fatalf("reused alignment was unexpectedly changed: %#v", reused.Plan.AlignedFASTA)
	}
}

func TestSnapshotCanvasItemsRestoresDisplayNameLock(t *testing.T) {
	items := canvasItemsFromSnapshot([]sessionsnapshot.CanvasItemV1{{
		Title: "canvas 1",
		Rows: []sessionsnapshot.CanvasRowV1{{
			Kind:              model.CanvasKindFasta,
			DisplayName:       "Locked PAL",
			DisplayNameLocked: true,
			FASTA: &model.QuerySequenceSource{
				LabelName: "PAL1",
				Sequence:  "MPEPTIDE",
			},
		}},
	}})
	if len(items) != 1 || len(items[0].Rows) != 1 {
		t.Fatalf("snapshot canvas items mismatch: %#v", items)
	}
	if !items[0].Rows[0].DisplayNameLocked {
		t.Fatalf("snapshot restore did not preserve display-name lock: %#v", items[0].Rows[0])
	}
}

func TestCanvasItemsFromSnapshotRecomputesSubtitleFromSelectedState(t *testing.T) {
	items := canvasItemsFromSnapshot([]sessionsnapshot.CanvasItemV1{{
		Title:    "canvas 1",
		Subtitle: "0/2 lines",
		Selected: []bool{true, false},
		Rows: []sessionsnapshot.CanvasRowV1{
			{
				Kind:  model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{Sequence: "MPEPTIDE", SequenceKind: model.SequenceProtein},
			},
			{
				Kind:  model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{Sequence: "MSECOND", SequenceKind: model.SequenceProtein},
			},
		},
	}})
	if len(items) != 1 {
		t.Fatalf("snapshot item count = %d, want 1", len(items))
	}
	if got := items[0].Subtitle; got != "1/2 lines" {
		t.Fatalf("snapshot subtitle = %q, want recomputed selection summary", got)
	}
}

func TestSnapshotCanvasSequenceCacheCollectsKeywordAndBlastRows(t *testing.T) {
	w := NewBlastWizard(nil)
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 370201, GenomeLabel: "TAIR12"}
	w.proteinSequenceCache[w.proteinSequenceCacheKey(370201, "AT2G37040.1")] = model.ProteinSequenceData{
		Sequence:       "MKEYWORD",
		OriginalHeader: ">AT2G37040.1",
	}
	w.proteinSequenceCache[w.proteinSequenceCacheKey(456, "BLASTSEQ1")] = model.ProteinSequenceData{
		Sequence:       "MBLAST",
		OriginalHeader: ">BLASTSEQ1",
	}

	cache, err := w.snapshotCanvasSequenceCache(context.Background(), []model.CanvasItem{{
		Title: "canvas 1",
		Rows: []model.CanvasRow{
			{
				Kind: model.CanvasKindKeyword,
				KeywordRow: &model.KeywordResultRow{
					LabelName:  "PAL1",
					ProteinID:  "AT2G37040.1",
					SequenceID: "AT2G37040.1",
				},
			},
			{
				Kind: model.CanvasKindBlast,
				BlastRow: &model.BlastResultRow{
					TargetID:   456,
					Protein:    "BLASTSEQ1",
					SequenceID: "BLASTSEQ1",
				},
			},
			{
				Kind: model.CanvasKindKeyword,
				KeywordRow: &model.KeywordResultRow{
					LabelName:  "INLINE1",
					ProteinID:  "INLINE1",
					SequenceID: "INLINE1",
					ExtraColumns: map[string]string{
						"protein_sequence": "MINLINE",
					},
				},
			},
		},
	}})
	if err != nil {
		t.Fatalf("snapshotCanvasSequenceCache returned error: %v", err)
	}
	if cache == nil {
		t.Fatal("expected canvas sequence cache")
	}
	gotIDs := make([]string, 0, len(cache.Entries))
	for _, entry := range cache.Entries {
		gotIDs = append(gotIDs, entry.SequenceID)
	}
	for _, want := range []string{"AT2G37040.1", "BLASTSEQ1", "INLINE1"} {
		if !slices.Contains(gotIDs, want) {
			t.Fatalf("canvas sequence cache missing %q: %#v", want, cache.Entries)
		}
	}
}

func TestCanvasTreeTableValuesUseGeneLocusAndPHgoLabelForNCBIKeywordRows(t *testing.T) {
	row := model.CanvasRow{
		Kind: model.CanvasKindKeyword,
		KeywordRow: &model.KeywordResultRow{
			SourceDatabase: "ncbi",
			Genome:         "Oryza sativa Japonica Group",
			GeneLocus:      "Os08g14760",
			GeneIdentifier: "GeneID:4335555",
			LabelName:      "Os4CL1",
			SequenceID:     "XP_015650724.1",
		},
	}
	values := canvasTreeTableValues(row, "1")
	if values["geneid"] != "Os08g14760" {
		t.Fatalf("tree table geneid = %q, want Gene locus", values["geneid"])
	}
	if values["blast_labelname"] != "" || values["blast_geneid"] != "" {
		t.Fatalf("keyword rows should not synthesize BLAST source fields: %#v", values)
	}
	if values[phylo.PHgoDisplayNameSource] != "Os-Os08g14760 (Os4CL1)" {
		t.Fatalf("PHgo table value = %q", values[phylo.PHgoDisplayNameSource])
	}
	records, meta, err := phylo.BuildInput([]phylo.RowSource{{
		ItemTitle:    "1",
		RowIndex:     0,
		CanvasRow:    row,
		Sequence:     "MPEPTIDE",
		SequenceKind: phylo.SequenceProtein,
		OriginalHead: "XP_015650724.1",
		TableValues:  values,
	}}, phylo.PHgoDisplayNameSource, "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	if meta.DisplayNameSource != phylo.PHgoDisplayNameSource || records[0].DisplayName != "Os-Os08g14760 (Os4CL1)" {
		t.Fatalf("PHgo display source did not carry into tree input: records=%#v meta=%#v", records, meta)
	}
}

func TestWriteCanvasSessionSnapshotIncludesSequenceCache(t *testing.T) {
	w := NewBlastWizard(nil)
	w.suppressTaskModals = true
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 370201, GenomeLabel: "TAIR12"}
	outputDir := mustOutputDir()
	path := filepath.Join(outputDir, "canvas-sequence-cache-test"+sessionsnapshot.FileExtension)
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })

	w.proteinSequenceCache[w.proteinSequenceCacheKey(370201, "AT2G37040.1")] = model.ProteinSequenceData{
		Sequence:       "MKEYWORD",
		OriginalHeader: ">AT2G37040.1",
	}
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title: "canvas 1",
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindKeyword,
				KeywordRow: &model.KeywordResultRow{
					LabelName:  "PAL1",
					ProteinID:  "AT2G37040.1",
					SequenceID: "AT2G37040.1",
				},
			}},
			Selected: []bool{true},
		}},
	}
	err := w.writeCanvasSessionSnapshot(state, exportSettings{
		BaseName:     "canvas-sequence-cache-test",
		OutputDir:    outputDir,
		WriteSession: true,
	})
	if err != nil {
		t.Fatalf("writeCanvasSessionSnapshot returned error: %v", err)
	}

	snapshot, err := sessionsnapshot.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if snapshot.SequenceCache == nil || len(snapshot.SequenceCache.Entries) != 1 {
		t.Fatalf("expected canvas snapshot sequence cache, got %#v", snapshot.SequenceCache)
	}
	if snapshot.SequenceCache.Entries[0].Sequence != "MKEYWORD" {
		t.Fatalf("canvas snapshot sequence cache mismatch: %#v", snapshot.SequenceCache.Entries[0])
	}
}

func TestCanvasRowHasSequenceForExportUsesAllFastaSequenceFields(t *testing.T) {
	if !canvasRowHasSequenceForExport(model.CanvasRow{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{ProteinSequence: "MPEPTIDE"}}) {
		t.Fatal("FASTA rows with ProteinSequence should be export-ready")
	}
	if !canvasRowHasSequenceForExport(model.CanvasRow{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{NucleotideSequence: "ATGC"}}) {
		t.Fatal("FASTA rows with NucleotideSequence should be export-ready")
	}
}

func TestRestoreCanvasTreeSnapshotRestoresPayloadAndPlan(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	dir := t.TempDir()
	snapshot := treeSnapshotForTest(dir, now)
	writeTreeSnapshotRestoreArtifacts(t, dir, snapshot)
	w.restoreCanvasTreeSnapshot(snapshot, false)
	if w.canvasTreeLastPayload.Newick != "(PHGOT000001);" {
		t.Fatalf("payload not restored: %#v", w.canvasTreeLastPayload)
	}
	if w.canvasTreeLastPlan.BaseDir == "" || w.canvasTreeLastPlan.RunID != "run1" {
		t.Fatalf("plan not restored: %#v", w.canvasTreeLastPlan)
	}
	if len(w.canvasTreeLastPlan.Records) != 1 || w.canvasTreeLastPlan.Records[0].DisplayName != "PAL display" {
		t.Fatalf("plan records/metadata not restored: %#v", w.canvasTreeLastPlan.Records)
	}
	if !strings.Contains(w.canvasTreeLastPlan.RuntimeRequest, `"session_id"`) || !strings.Contains(w.canvasTreeLastPlan.RuntimeResponse, `"mega-phgo-runtime"`) {
		t.Fatalf("runtime audit files not restored into plan: request=%q response=%q", w.canvasTreeLastPlan.RuntimeRequest, w.canvasTreeLastPlan.RuntimeResponse)
	}
	if w.canvasTreeLastPlan.Settings.AlignmentMethod != phylo.AlignmentMUSCLE || w.canvasTreeLastPlan.Fingerprints.Tree != "manifest-tree" {
		t.Fatalf("manifest settings/fingerprints not restored: %#v", w.canvasTreeLastPlan)
	}
	if !w.canvasTreeForceCompute {
		t.Fatalf("restored snapshot tree state must force the next refresh to recompute")
	}
	panel := w.prompt.SnapshotCanvasTreePanelState(canvasStateKey("canvas"))
	if panel.AlignmentMethod != string(phylo.AlignmentMUSCLE) || !panel.EnabledEver {
		t.Fatalf("panel not restored: %#v", panel)
	}
}

func TestRestoreCanvasTreeSnapshotBlocksFirstArtifactReuse(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	dir := t.TempDir()
	settings := phylo.DefaultTreeSettings()
	settings.AlignmentMethod = phylo.AlignmentMUSCLE
	records := []phylo.InputRecord{{
		TaxonID:        "PHGOT000001",
		DisplayName:    "PAL display",
		Sequence:       "MPEPTIDE",
		SequenceKind:   phylo.SequenceProtein,
		RowFingerprint: "row-fingerprint",
	}}
	meta := phylo.Metadata{SchemaVersion: 1, GeneratedAt: now, DisplayNameSource: "label_name", Records: records}
	aligned := ">PHGOT000001\nMPEPTIDE\n"
	newick := "(PHGOT000001);"
	last, err := phylo.BuildRunPlan("canvas", "run1", dir, settings, phylo.SequenceProtein, records, meta, aligned, newick, now)
	if err != nil {
		t.Fatalf("BuildRunPlan last: %v", err)
	}
	snapshot := sessionsnapshot.CanvasTreeV2{
		PanelState:       treePanelForSnapshotTest(),
		LastPayload:      phylo.BuildPayload("canvas", records, meta, aligned, newick, now),
		LastManifest:     last.ToArtifactSet().Manifest,
		LastArtifactDir:  dir,
		LastRunID:        "run1",
		LastAlignedFASTA: aligned,
		LastNewick:       newick,
		Fingerprints:     last.Fingerprints,
	}
	writeTreeSnapshotRestoreArtifacts(t, dir, snapshot)
	w.restoreCanvasTreeSnapshot(snapshot, false)
	if !w.canvasTreeForceCompute {
		t.Fatalf("snapshot restore did not mark the next refresh as full-compute")
	}
	candidate, err := phylo.BuildRunPlan("canvas", "run2", t.TempDir(), settings, phylo.SequenceProtein, records, meta, "", "", now)
	if err != nil {
		t.Fatalf("BuildRunPlan candidate: %v", err)
	}
	if reused, ok, err := w.reuseLastCanvasTreePlan(candidate, records, meta, now); err != nil {
		t.Fatalf("reuse with force flag returned error: %v", err)
	} else if ok {
		t.Fatalf("first refresh after snapshot restore reused artifacts instead of recomputing: %#v", reused)
	}
	w.canvasTreeForceCompute = false
	if _, ok, err := w.reuseLastCanvasTreePlan(candidate, records, meta, now); err != nil {
		t.Fatalf("reuse after clearing force flag returned error: %v", err)
	} else if !ok {
		t.Fatalf("matching non-snapshot artifacts should remain reusable after the forced refresh has completed")
	}
}

func TestRestoreCanvasTreeSnapshotDoesNotBiologicallyValidatePayload(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	dir := t.TempDir()
	snapshot := treeSnapshotForTest(dir, now)
	snapshot.LastPayload.AlignedFASTA = ">PHGOT000001\nATGCGTATGCGT\n"
	snapshot.LastAlignedFASTA = snapshot.LastPayload.AlignedFASTA
	writeTreeSnapshotRestoreArtifacts(t, dir, snapshot)
	w.restoreCanvasTreeSnapshot(snapshot, false)
	if strings.TrimSpace(w.canvasTreeLastPayload.Newick) != "(PHGOT000001);" {
		t.Fatalf("snapshot payload should be restored without Go-side biological validation: %#v", w.canvasTreeLastPayload)
	}
	if strings.TrimSpace(w.canvasTreeLastPlan.AlignedFASTA) != ">PHGOT000001\nATGCGTATGCGT" {
		t.Fatalf("snapshot plan should keep restored runtime alignment unchanged: %#v", w.canvasTreeLastPlan)
	}
}

func TestRestoreCanvasTreeSnapshotCanRecoverPayloadFromArtifact(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	dir := t.TempDir()
	snapshot := treeSnapshotForTest(dir, now)
	writeTreeSnapshotRestoreArtifacts(t, dir, snapshot)
	payloadData, err := json.Marshal(snapshot.LastPayload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "viewer.payload.json"), payloadData, 0o644); err != nil {
		t.Fatalf("write viewer payload: %v", err)
	}
	snapshot.LastPayload = phylo.ViewerPayload{}
	w.restoreCanvasTreeSnapshot(snapshot, false)
	if w.canvasTreeLastPayload.Newick != "(PHGOT000001);" || len(w.canvasTreeLastPlan.Records) != 1 {
		t.Fatalf("payload artifact fallback did not restore tree state: payload=%#v plan=%#v", w.canvasTreeLastPayload, w.canvasTreeLastPlan)
	}
}

func TestRestoreCanvasTreeSnapshotRemapsLegacyOutputTreeDirToCache(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	outputDir, err := appfs.OutputDir()
	if err != nil {
		t.Fatalf("OutputDir returned error: %v", err)
	}
	legacyDir := filepath.Join(outputDir, "tree", "legacy-session", "run1")
	cacheDir := mustCanvasTreeArtifactDir("legacy-session", "run1")
	snapshot := treeSnapshotForTest(legacyDir, now)
	writeTreeSnapshotRestoreArtifacts(t, cacheDir, snapshot)

	w.restoreCanvasTreeSnapshot(snapshot, false)

	if !samePath(w.canvasTreeLastPlan.BaseDir, cacheDir) {
		t.Fatalf("restored BaseDir = %q, want cache dir %q", w.canvasTreeLastPlan.BaseDir, cacheDir)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy output tree dir should not be required during restore, stat err=%v", err)
	}
}

func TestRestoreLegacyCanvasTreeSnapshotResetsTreeParamsToCurrentDefaults(t *testing.T) {
	w := NewBlastWizard(nil)
	now := time.Now()
	dir := t.TempDir()
	snapshot := treeSnapshotForTest(dir, now)
	snapshot.PanelState.AlignmentParams = map[string]string{"max_iterations": "8"}
	snapshot.PanelState.TreeParams = map[string]string{"distance_model": "p-distance"}
	snapshot.LastManifest.Settings.AlignmentParams = map[string]string{"max_iterations": "8"}
	snapshot.LastManifest.Settings.TreeParams = map[string]string{"distance_model": "p-distance"}
	writeTreeSnapshotRestoreArtifacts(t, dir, snapshot)

	w.restoreCanvasTreeSnapshot(snapshot, true)

	if got := w.canvasTreeLastPlan.Settings.AlignmentParams["multiple_gap_opening_penalty"]; got != "10" {
		t.Fatalf("legacy alignment params should reset to current protein defaults, got multiple_gap_opening_penalty=%q", got)
	}
	if got := w.canvasTreeLastPlan.Settings.TreeParams["model_method"]; got != "Poisson model" {
		t.Fatalf("tree defaults should still be restored from current definition, got model_method=%q", got)
	}
	panel := w.prompt.SnapshotCanvasTreePanelState(canvasStateKey("canvas"))
	if got := panel.AlignmentParams["multiple_gap_opening_penalty"]; got != "10" {
		t.Fatalf("legacy panel alignment params should reset to current protein defaults, got multiple_gap_opening_penalty=%q", got)
	}
}

func TestTreeSettingsFromSnapshotPanelNormalizesEmptyDraftState(t *testing.T) {
	settings := treeSettingsFromSnapshotPanel(tui.CanvasTreePanelState{})
	if settings.DisplayNameSource != phylo.DefaultDisplayNameSource {
		t.Fatalf("display source = %q", settings.DisplayNameSource)
	}
	if settings.AlignmentMethod != phylo.DefaultAlignmentMethod {
		t.Fatalf("alignment method = %q", settings.AlignmentMethod)
	}
	if settings.ConversionTarget != phylo.DefaultConversionTarget || !settings.ConversionSkipUnselect {
		t.Fatalf("mode/recovery defaults = target %q unselect %v", settings.ConversionTarget, settings.ConversionSkipUnselect)
	}
	if settings.TreeMethod != phylo.DefaultTreeMethod {
		t.Fatalf("tree method = %q", settings.TreeMethod)
	}
	if len(settings.AlignmentParams) == 0 || len(settings.TreeParams) == 0 {
		t.Fatalf("expected default params to be populated: %#v", settings)
	}
}

func TestEnsureCanvasTreeRuntimeInteractiveUsesInjectedChecker(t *testing.T) {
	w := NewBlastWizard(nil)
	want := errors.New("runtime missing")
	called := false
	w.ensureCanvasTreeRuntime = func(context.Context) error {
		called = true
		return want
	}
	err := w.ensureCanvasTreeRuntimeInteractive(context.Background())
	if !called {
		t.Fatal("runtime checker was not called")
	}
	if !errors.Is(err, want) {
		t.Fatalf("ensureCanvasTreeRuntimeInteractive error = %v, want %v", err, want)
	}
}

func TestRefreshCanvasTreeReturnsClearErrorWithoutSequenceSource(t *testing.T) {
	w := NewBlastWizard(nil)
	w.canvasTreeRefreshRun = func(ctx context.Context, runState canvasLaunchState, settings phylo.TreeSettings) error {
		selected := selectedCanvasRowsInOrder(runState.Items)
		if len(selected) != 2 {
			t.Fatalf("selected rows = %d, want 2", len(selected))
		}
		return fmt.Errorf("mega-phgo-runtime: protein sequence source is unavailable")
	}
	state := canvasLaunchState{
		Items: []model.CanvasItem{
			{
				Title: "canvas 1",
				Rows: []model.CanvasRow{{
					Kind: model.CanvasKindKeyword,
					KeywordRow: &model.KeywordResultRow{
						LabelName:  "PAL1",
						ProteinID:  "AT2G37040.1",
						SequenceID: "AT2G37040.1",
					},
				}},
				Selected: []bool{true},
			},
			{
				Title: "canvas 2",
				Rows: []model.CanvasRow{{
					Kind: model.CanvasKindFasta,
					FASTA: &model.QuerySequenceSource{
						Annotation: "query1",
						Sequence:   "MPEPTIDE",
					},
				}},
				Selected: []bool{true},
			},
		},
	}
	err := w.refreshCanvasTreeInteractive(context.Background(), &state, phylo.DefaultTreeSettings())
	if err == nil {
		t.Fatal("expected refreshCanvasTree to fail when source is unavailable")
	}
	if strings.Contains(strings.ToLower(err.Error()), "nil pointer") {
		t.Fatalf("refreshCanvasTree should not panic-wrap nil pointer errors: %v", err)
	}
	if !strings.Contains(err.Error(), "protein sequence source is unavailable") {
		t.Fatalf("unexpected refresh error: %v", err)
	}
}

func TestHydrateCanvasRowSequenceDataUsesSnapshotCacheForLegacyRows(t *testing.T) {
	w := NewBlastWizard(nil)
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 370201, GenomeLabel: "TAIR12"}
	w.hydrateSnapshotSequenceCache(&sessionsnapshot.SequenceCacheV1{
		Entries: []sessionsnapshot.SequenceCacheEntryV1{
			{TargetID: 370201, SequenceID: "AT2G37040.1", Sequence: "MKEYWORD", OriginalHeader: ">AT2G37040.1"},
			{TargetID: 456, SequenceID: "BLASTSEQ1", Sequence: "MBLAST", OriginalHeader: ">BLASTSEQ1"},
		},
	})
	items := w.hydrateCanvasRowSequenceData([]model.CanvasItem{{
		Title: "canvas 1",
		Rows: []model.CanvasRow{
			{
				Kind: model.CanvasKindKeyword,
				KeywordRow: &model.KeywordResultRow{
					LabelName:  "PAL1",
					SequenceID: "AT2G37040.1",
				},
			},
			{
				Kind: model.CanvasKindBlast,
				BlastRow: &model.BlastResultRow{
					TargetID:   456,
					Protein:    "BLASTSEQ1",
					SequenceID: "BLASTSEQ1",
				},
			},
		},
	}})
	if got := items[0].Rows[0].SequenceData; got == nil || got.Sequence != "MKEYWORD" {
		t.Fatalf("keyword row did not hydrate sequence cache: %#v", got)
	}
	if got := items[0].Rows[1].SequenceData; got == nil || got.Sequence != "MBLAST" {
		t.Fatalf("blast row did not hydrate sequence cache: %#v", got)
	}
	if items[0].Rows[0].SequenceReady == nil || !*items[0].Rows[0].SequenceReady {
		t.Fatalf("keyword row should be marked sequence-ready: %#v", items[0].Rows[0].SequenceReady)
	}
	if items[0].Rows[1].SequenceReady == nil || !*items[0].Rows[1].SequenceReady {
		t.Fatalf("blast row should be marked sequence-ready: %#v", items[0].Rows[1].SequenceReady)
	}
}

func TestCanvasTreeRowSourcesUsesStoredSequenceDataForMultiCanvasSelection(t *testing.T) {
	w := NewBlastWizard(nil)
	state := canvasLaunchState{
		Items: []model.CanvasItem{
			{
				Title: "canvas 1",
				Rows: []model.CanvasRow{
					{
						Kind: model.CanvasKindKeyword,
						KeywordRow: &model.KeywordResultRow{
							LabelName:  "PAL1",
							SequenceID: "AT2G37040.1",
						},
						SequenceData: &model.ProteinSequenceData{Sequence: "MKEYWORD", OriginalHeader: ">AT2G37040.1"},
					},
					{
						Kind: model.CanvasKindKeyword,
						KeywordRow: &model.KeywordResultRow{
							LabelName:  "SKIP",
							SequenceID: "ATSKIP.1",
						},
						SequenceData: &model.ProteinSequenceData{Sequence: "MSKIP", OriginalHeader: ">ATSKIP.1"},
					},
				},
				Selected: []bool{true, false},
			},
			{
				Title: "canvas 2",
				Rows: []model.CanvasRow{
					{
						Kind: model.CanvasKindFasta,
						FASTA: &model.QuerySequenceSource{
							Annotation: "plain fasta",
							Sequence:   "MFASTA",
						},
					},
					{
						Kind: model.CanvasKindBlast,
						BlastRow: &model.BlastResultRow{
							TargetID:   456,
							Protein:    "BLASTSEQ1",
							SequenceID: "BLASTSEQ1",
							LabelName:  "C4H",
						},
						SequenceData: &model.ProteinSequenceData{Sequence: "MBLAST", OriginalHeader: ">BLASTSEQ1"},
					},
				},
				Selected: []bool{false, true},
			},
		},
	}
	selected := selectedCanvasRowsInOrder(state.Items)
	if len(selected) != 2 {
		t.Fatalf("selected rows = %d, want 2", len(selected))
	}
	sources, err := w.canvasTreeRowSources(context.Background(), state, selected)
	if err != nil {
		t.Fatalf("canvasTreeRowSources returned error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("row source count = %d, want 2", len(sources))
	}
	if sources[0].Sequence != "MKEYWORD" || sources[1].Sequence != "MBLAST" {
		t.Fatalf("row sources should use stored sequences in selected order: %#v", sources)
	}
	if sources[0].ItemTitle != "canvas 1" || sources[1].ItemTitle != "canvas 2" {
		t.Fatalf("row source canvas order mismatch: %#v", sources)
	}
}

func TestCanvasTreeRowSourcesDoesNotInferMixedKindFasta(t *testing.T) {
	w := NewBlastWizard(nil)
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "mixed",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{
					Sequence:     "ACGTMPEPTIDE",
					SequenceKind: "",
				},
			}},
		}},
	}
	selected := selectedCanvasRowsInOrder(state.Items)
	if len(selected) != 1 {
		t.Fatalf("selected rows = %d, want 1", len(selected))
	}
	sources, err := w.canvasTreeRowSources(context.Background(), state, selected)
	if err != nil {
		t.Fatalf("canvasTreeRowSources returned error: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("row source count = %d, want 1", len(sources))
	}
	if sources[0].SequenceKind != phylo.SequenceUnknown {
		t.Fatalf("sequence kind = %s, want unknown", sources[0].SequenceKind)
	}
}

func TestCanvasTreeRowSourcesChoosesFastaSequenceForConversionTarget(t *testing.T) {
	w := NewBlastWizard(nil)
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "dual-sequence",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{
					Sequence:           "MGENERIC",
					ProteinSequence:    "MPROTEIN",
					NucleotideSequence: "ATGCCCGGG",
					SequenceKind:       model.SequenceProtein,
				},
			}},
		}},
	}
	selected := selectedCanvasRowsInOrder(state.Items)

	proteinSettings := phylo.DefaultTreeSettings()
	proteinSources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, proteinSettings)
	if err != nil {
		t.Fatalf("protein canvasTreeRowSourcesWithSkippedForSettings returned error: %v", err)
	}
	if len(proteinSources) != 1 || proteinSources[0].Sequence != "MPROTEIN" || proteinSources[0].SequenceKind != phylo.SequenceProtein {
		t.Fatalf("protein target row source = %#v, want protein sequence", proteinSources)
	}

	dnaSettings := phylo.DefaultTreeSettings()
	dnaSettings.ConversionTarget = phylo.ConversionTargetDNA
	dnaSources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, dnaSettings)
	if err != nil {
		t.Fatalf("DNA canvasTreeRowSourcesWithSkippedForSettings returned error: %v", err)
	}
	if len(dnaSources) != 1 || dnaSources[0].Sequence != "ATGCCCGGG" || dnaSources[0].SequenceKind != phylo.SequenceNucleotide {
		t.Fatalf("DNA target row source = %#v, want nucleotide sequence", dnaSources)
	}
}

func TestCanvasTreeRowSourcesResolvesNucleotideSequenceForDNAConversion(t *testing.T) {
	w := NewBlastWizard(nil)
	w.source = fakeSource{
		name: "lemna",
		nucleotideSeqs: map[string]string{
			"blastn|SEQ1": "ATGCCCGGG",
		},
	}
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "blast-row",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindBlast,
				BlastRow: &model.BlastResultRow{
					TargetID:     18,
					SequenceID:   "SEQ1",
					TranscriptID: "SEQ1",
					Protein:      "SEQ1",
				},
				SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
			}},
		}},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA
	selected := selectedCanvasRowsInOrder(state.Items)
	sources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, settings)
	if err != nil {
		t.Fatalf("canvasTreeRowSourcesWithSkippedForSettings returned error: %v", err)
	}
	if len(sources) != 1 || sources[0].Sequence != "ATGCCCGGG" || sources[0].SequenceKind != phylo.SequenceNucleotide {
		t.Fatalf("DNA resolver row source = %#v, want resolved nucleotide sequence", sources)
	}
}

func TestCanvasTreeRowSourcesResolvesNucleotideUsingRowSourceDatabase(t *testing.T) {
	w := NewBlastWizard(nil)
	w.sourceFactory = func(database string) source.DataSource {
		if database == "lemna" {
			return fakeSource{
				name: "lemna",
				nucleotideSeqs: map[string]string{
					"blastn|SEQ1": "ATGCCCGGG",
				},
			}
		}
		return nil
	}
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "snapshot-blast-row",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindBlast,
				BlastRow: &model.BlastResultRow{
					SourceDatabase: "lemna",
					TargetID:       18,
					SequenceID:     "SEQ1",
					TranscriptID:   "SEQ1",
					Protein:        "SEQ1",
				},
				SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
			}},
		}},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA
	selected := selectedCanvasRowsInOrder(state.Items)
	sources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, settings)
	if err != nil {
		t.Fatalf("canvasTreeRowSourcesWithSkippedForSettings returned error: %v", err)
	}
	if len(sources) != 1 || sources[0].Sequence != "ATGCCCGGG" || sources[0].SequenceKind != phylo.SequenceNucleotide {
		t.Fatalf("row-source resolver source = %#v, want resolved nucleotide sequence", sources)
	}
}

func TestCanvasTreeRowSourcesResolvesFastaProteinToDNAUsingPhytozomeHeader(t *testing.T) {
	w := NewBlastWizard(nil)
	w.sourceFactory = func(database string) source.DataSource {
		if database == "phytozome" {
			return fakeSource{
				name:    "phytozome",
				species: []model.SpeciesCandidate{{ProteomeID: 167, SearchAlias: "Arabidopsis thaliana", GenomeLabel: "Arabidopsis thaliana TAIR10"}},
				nucleotideSeqs: map[string]string{
					"blastn|AT1G51680.1": "ATGCCCGGG",
				},
			}
		}
		return nil
	}
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "snapshot-fasta-row",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{
					Sequence:            "MPEPTIDE",
					ProteinSequence:     "MPEPTIDE",
					SequenceKind:        model.SequenceProtein,
					PreferredSequenceID: "AT1G51680.1",
					GeneID:              "AT1G51680.1",
					SourceDatabase:      "fasta",
					OrganismShort:       "Arabidopsis thaliana TAIR10",
					Annotation:          "phgo://Arabidopsis thaliana TAIR10/4CL1/AT1G51680.1\\6",
				},
			}},
		}},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA
	selected := selectedCanvasRowsInOrder(state.Items)
	sources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, settings)
	if err != nil {
		t.Fatalf("canvasTreeRowSourcesWithSkippedForSettings returned error: %v", err)
	}
	if len(sources) != 1 || sources[0].Sequence != "" || sources[0].SequenceKind != phylo.SequenceNucleotide {
		t.Fatalf("FASTA row source = %#v, want selected empty nucleotide record when no real DNA sequence is embedded or resolved", sources)
	}
}

func TestCanvasItemsFromSnapshotInputRestoresCanvasSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "canvas-source.pgo")
	if err := sessionsnapshot.WriteFile(path, sessionsnapshot.Snapshot{
		Context: sessionsnapshot.ContextV1{CreatedAt: time.Now(), Mode: "canvas"},
		Canvas: &sessionsnapshot.CanvasResultV1{
			Items: []sessionsnapshot.CanvasItemV1{{
				Rows: []sessionsnapshot.CanvasRowV1{{
					Kind: model.CanvasKindBlast,
					BlastRow: &model.BlastResultRow{
						SourceDatabase: "lemna",
						SequenceID:     "SEQ1",
					},
				}},
				Selected: []bool{true},
			}},
		},
	}); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	w := NewBlastWizard(nil)
	w.sourceFactory = func(database string) source.DataSource {
		if database == "lemna" {
			return fakeSource{name: "lemna"}
		}
		return nil
	}
	items, err := w.canvasItemsFromSnapshotInput(path, path)
	if err != nil {
		t.Fatalf("canvasItemsFromSnapshotInput returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if w.source == nil || w.source.Name() != "lemna" {
		t.Fatalf("snapshot source = %#v, want lemna", w.source)
	}
}

func TestInferSnapshotDatabaseUsesCanvasRows(t *testing.T) {
	snapshot := sessionsnapshot.Snapshot{
		Canvas: &sessionsnapshot.CanvasResultV1{
			Items: []sessionsnapshot.CanvasItemV1{{
				Rows: []sessionsnapshot.CanvasRowV1{
					{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{SourceDatabase: "fasta"}},
					{Kind: model.CanvasKindBlast, BlastRow: &model.BlastResultRow{SourceDatabase: "lemna"}},
				},
			}},
		},
	}
	if got := inferSnapshotDatabase(snapshot); got != "lemna" {
		t.Fatalf("inferSnapshotDatabase = %q, want lemna", got)
	}
}

func TestCanvasTreeRowSourcesPassesAmbiguousFastaToMEGARuntime(t *testing.T) {
	w := NewBlastWizard(nil)
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "mixed",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindFasta,
				FASTA: &model.QuerySequenceSource{
					Sequence:     "???",
					SequenceKind: "",
				},
			}},
		}},
	}
	selected := selectedCanvasRowsInOrder(state.Items)
	sources, err := w.canvasTreeRowSourcesWithSkipped(context.Background(), state, selected)
	if err != nil {
		t.Fatalf("canvasTreeRowSourcesWithSkipped returned error: %v", err)
	}
	if len(sources) != 1 || sources[0].Sequence != "???" {
		t.Fatalf("row source = %#v, want ambiguous FASTA passed through", sources)
	}
}

func TestSelectedCanvasRowsInOrderKeepsSelectedRowsWithoutExportReadySequence(t *testing.T) {
	rows := selectedCanvasRowsInOrder([]model.CanvasItem{{
		Title:    "tree-input",
		Selected: []bool{true, true, false},
		Rows: []model.CanvasRow{
			{Kind: model.CanvasKindKeyword, KeywordRow: &model.KeywordResultRow{LabelName: "missing-sequence"}},
			{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{Annotation: "empty-fasta"}},
			{Kind: model.CanvasKindFasta, FASTA: &model.QuerySequenceSource{Sequence: "MPEPTIDE"}},
		},
	}})
	if len(rows) != 2 {
		t.Fatalf("selected rows = %#v, want both checked rows even when sequence payloads are empty", rows)
	}
}

func TestNormalizeCanvasTreeRowSourcesPassesMixedDNAProteinToMEGARuntime(t *testing.T) {
	sources := []phylo.RowSource{
		{ItemTitle: "protein", RowIndex: 0, Sequence: "MPEPTIDE", SequenceKind: phylo.SequenceProtein},
		{ItemTitle: "dna", RowIndex: 0, Sequence: "ATGCGTATGCGT", SequenceKind: phylo.SequenceNucleotide},
	}
	normalized, skipped, kind, err := normalizeCanvasTreeRowSourcesWithSkipped(sources, nil, phylo.DefaultTreeSettings())
	if err != nil {
		t.Fatalf("normalizeCanvasTreeRowSourcesWithSkipped returned error: %v", err)
	}
	if kind != phylo.SequenceProtein {
		t.Fatalf("normalized kind = %s, want protein", kind)
	}
	if len(normalized) != 2 {
		t.Fatalf("normalized rows = %#v, want both rows for MEGA runtime conversion", normalized)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped rows = %#v, want none", skipped)
	}
}

func TestNormalizeCanvasTreeRowSourcesDoesNotSkipMismatchedRowsWhenConfigured(t *testing.T) {
	sources := []phylo.RowSource{
		{ItemTitle: "protein", RowIndex: 0, Sequence: "MPEPTIDE", SequenceKind: phylo.SequenceProtein},
		{ItemTitle: "dna", RowIndex: 1, Sequence: "ATGCNNNNATGC", SequenceKind: phylo.SequenceNucleotide},
	}
	settings := phylo.DefaultTreeSettings()
	normalized, skipped, kind, err := normalizeCanvasTreeRowSourcesWithSkipped(sources, nil, settings)
	if err != nil {
		t.Fatalf("normalizeCanvasTreeRowSourcesWithSkipped returned error: %v", err)
	}
	if kind != phylo.SequenceProtein {
		t.Fatalf("normalized kind = %s, want protein", kind)
	}
	if len(normalized) != 2 {
		t.Fatalf("normalized rows = %#v, want all rows passed to MEGA runtime", normalized)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped rows = %#v, want none", skipped)
	}
}

func TestNormalizeCanvasTreeRowSourcesPassesEmptyRowsToMEGARuntime(t *testing.T) {
	sources := []phylo.RowSource{
		{ItemTitle: "protein", RowIndex: 0, Sequence: "MPEPTIDE", SequenceKind: phylo.SequenceProtein},
		{ItemTitle: "dna", RowIndex: 1, Sequence: "", SequenceKind: phylo.SequenceNucleotide},
	}
	normalized, skipped, kind, err := normalizeCanvasTreeRowSourcesWithSkipped(sources, nil, phylo.DefaultTreeSettings())
	if err != nil {
		t.Fatalf("normalizeCanvasTreeRowSourcesWithSkipped returned error: %v", err)
	}
	if kind != phylo.SequenceProtein {
		t.Fatalf("normalized kind = %s, want protein", kind)
	}
	if len(normalized) != 2 {
		t.Fatalf("normalized rows = %#v, want all rows passed to MEGA runtime", normalized)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped rows = %#v, want none", skipped)
	}
}

func TestNormalizeCanvasTreeRowSourcesPreservesUnknownRowKind(t *testing.T) {
	sources := []phylo.RowSource{
		{ItemTitle: "ambiguous", RowIndex: 0, Sequence: "ACGTMPEPTIDE", SequenceKind: phylo.SequenceUnknown},
	}
	normalized, _, kind, err := normalizeCanvasTreeRowSourcesWithSkipped(sources, nil, phylo.DefaultTreeSettings())
	if err != nil {
		t.Fatalf("normalizeCanvasTreeRowSourcesWithSkipped returned error: %v", err)
	}
	if kind != phylo.SequenceProtein {
		t.Fatalf("global target kind = %s, want protein", kind)
	}
	if normalized[0].SequenceKind != phylo.SequenceUnknown {
		t.Fatalf("row sequence kind = %s, want unknown metadata preserved", normalized[0].SequenceKind)
	}
}

func TestCanvasTreeDNAModeResolvesKeywordNucleotideBeforeStoredProtein(t *testing.T) {
	w := NewBlastWizard(nil)
	w.source = fakeSource{
		name:           "phytozome",
		nucleotideSeqs: map[string]string{"blastn|tx1": "ATGAAATGA"},
	}
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 42}
	selected := canvasSelectedRow{
		ItemTitle: "keyword",
		RowIndex:  0,
		Row: model.CanvasRow{
			Kind: model.CanvasKindKeyword,
			KeywordRow: &model.KeywordResultRow{
				SourceDatabase: "phytozome",
				SequenceID:     "tx1",
			},
			SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
		},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA
	settings.AlignmentMethod = phylo.AlignmentClustalW

	choice, err := w.canvasTreeSequenceForSettings(context.Background(), selected, settings)
	if err != nil {
		t.Fatalf("canvasTreeSequenceForSettings returned error: %v", err)
	}
	if choice.Kind != phylo.SequenceNucleotide || choice.Sequence != "ATGAAATGA" {
		t.Fatalf("DNA mode should resolve real nucleotide sequence, got %#v", choice)
	}
}

func TestCanvasTreeDNAModeDoesNotInventNucleotideWhenResolverMisses(t *testing.T) {
	w := NewBlastWizard(nil)
	w.source = fakeSource{name: "phytozome"}
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 42}
	selected := canvasSelectedRow{
		ItemTitle: "keyword",
		RowIndex:  0,
		Row: model.CanvasRow{
			Kind: model.CanvasKindKeyword,
			KeywordRow: &model.KeywordResultRow{
				SourceDatabase: "phytozome",
				SequenceID:     "tx1",
			},
			SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
		},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA

	choice, err := w.canvasTreeSequenceForSettings(context.Background(), selected, settings)
	if err == nil {
		t.Fatalf("canvasTreeSequenceForSettings returned %#v, want resolver error", choice)
	}
	if !strings.Contains(err.Error(), "no nucleotide sequence") {
		t.Fatalf("resolver error = %v, want original nucleotide fetch error", err)
	}
}

func TestCanvasTreeRowSourcesReturnsNucleotideResolverErrors(t *testing.T) {
	w := NewBlastWizard(nil)
	w.source = fakeSource{name: "phytozome"}
	w.lastKeywordSpecies = model.SpeciesCandidate{ProteomeID: 42}
	state := canvasLaunchState{
		Items: []model.CanvasItem{{
			Title:    "keyword",
			Selected: []bool{true},
			Rows: []model.CanvasRow{{
				Kind: model.CanvasKindKeyword,
				KeywordRow: &model.KeywordResultRow{
					SourceDatabase: "phytozome",
					SequenceID:     "tx1",
				},
				SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
			}},
		}},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA

	selected := selectedCanvasRowsInOrder(state.Items)
	sources, err := w.canvasTreeRowSourcesWithSkippedForSettings(context.Background(), state, selected, settings)
	if err == nil {
		t.Fatalf("canvasTreeRowSourcesWithSkippedForSettings returned sources %#v, want resolver error", sources)
	}
	if !strings.Contains(err.Error(), "no nucleotide sequence") {
		t.Fatalf("row-source error = %v, want original nucleotide fetch error", err)
	}
}

func TestCanvasTreeDNAModeReturnsSourceConstructionErrors(t *testing.T) {
	w := NewBlastWizard(nil)
	selected := canvasSelectedRow{
		ItemTitle: "blast",
		RowIndex:  0,
		Row: model.CanvasRow{
			Kind: model.CanvasKindBlast,
			BlastRow: &model.BlastResultRow{
				SourceDatabase: "missingdb",
				TargetID:       42,
				SequenceID:     "tx1",
			},
			SequenceData: &model.ProteinSequenceData{Sequence: "MPEPTIDE"},
		},
	}
	settings := phylo.DefaultTreeSettings()
	settings.ConversionTarget = phylo.ConversionTargetDNA

	choice, err := w.canvasTreeSequenceForSettings(context.Background(), selected, settings)
	if err == nil {
		t.Fatalf("canvasTreeSequenceForSettings returned %#v, want source-construction error", choice)
	}
	if !strings.Contains(err.Error(), "unsupported BLAST target database") {
		t.Fatalf("source-construction error = %v", err)
	}
}

func TestRefreshCanvasTreeInteractiveSkipUnchecksRowsAndRetries(t *testing.T) {
	w := NewBlastWizard(nil)
	state := canvasLaunchState{
		Items: []model.CanvasItem{
			{
				Title:    "protein",
				Selected: []bool{true},
				Rows: []model.CanvasRow{{
					Kind:  model.CanvasKindFasta,
					FASTA: &model.QuerySequenceSource{Sequence: "MPEPTIDE", SequenceKind: model.SequenceProtein},
				}},
			},
			{
				Title:    "dna",
				Selected: []bool{true},
				Rows: []model.CanvasRow{{
					Kind:  model.CanvasKindFasta,
					FASTA: &model.QuerySequenceSource{Sequence: "NN", SequenceKind: model.SequenceDNA},
				}},
			},
		},
	}
	callCount := 0
	recoveryCount := 0
	w.canvasTreeRefreshRun = func(ctx context.Context, runState canvasLaunchState, settings phylo.TreeSettings) error {
		callCount++
		selected := selectedCanvasRowsInOrder(runState.Items)
		if callCount == 1 {
			if len(selected) != 2 {
				t.Fatalf("first refresh selected rows = %d, want 2", len(selected))
			}
			return &canvasTreeSkippedRowsError{SkippedRows: []canvasTreeSkippedRow{{
				ItemTitle: "dna",
				RowIndex:  0,
				Reason:    "skipped by mega-phgo-runtime",
			}}}
		}
		if len(selected) != 1 || selected[0].ItemTitle != "protein" {
			t.Fatalf("retry selected rows = %#v, want only protein row", selected)
		}
		return nil
	}
	w.canvasTreeRecover = func(description string, backTarget error, allowSkip bool) (string, error) {
		recoveryCount++
		if !allowSkip {
			t.Fatalf("allowSkip = false, want true")
		}
		if !strings.Contains(description, "cannot be used for the current tree refresh") {
			t.Fatalf("unexpected recovery description: %q", description)
		}
		if !strings.Contains(description, "dna row 1") {
			t.Fatalf("recovery description missing skipped row detail: %q", description)
		}
		return "skip", nil
	}
	settings := phylo.DefaultTreeSettings()
	err := w.refreshCanvasTreeInteractive(context.Background(), &state, settings)
	if err != nil {
		t.Fatalf("refreshCanvasTreeInteractive returned error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("refresh call count = %d, want 2", callCount)
	}
	if recoveryCount != 1 {
		t.Fatalf("recovery call count = %d, want 1", recoveryCount)
	}
	if !state.Items[0].Selected[0] {
		t.Fatalf("protein row should remain selected: %#v", state.Items[0].Selected)
	}
	if state.Items[1].Selected[0] {
		t.Fatalf("skipped dna row should be unselected: %#v", state.Items[1].Selected)
	}
	if got := state.Items[1].Subtitle; got != "0/1 lines" {
		t.Fatalf("dna canvas subtitle = %q, want selection summary updated", got)
	}
}

func TestMegaPHGORuntimeCanvasSnapshot123Probe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	path := strings.TrimSpace(os.Getenv("PHYTOZOME_MEGAPHGO_PGO"))
	explicitPath := path != ""
	if path == "" {
		path = `C:\Users\wangsychn\Desktop\output\123.pgo`
	}
	if _, err := os.Stat(path); err != nil {
		if !explicitPath {
			t.Skipf("real Canvas snapshot %s is not available: %v", path, err)
		}
		t.Fatalf("real Canvas snapshot %s is not available: %v", path, err)
	}
	appRoot := repoRootForWorkflowRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	snapshot, err := sessionsnapshot.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	if snapshot.Canvas == nil || len(snapshot.Canvas.Items) == 0 {
		t.Fatalf("snapshot %s does not contain Canvas items", path)
	}
	w := NewBlastWizard(nil)
	w.instanceID = "123-pgo-runtime-probe"
	w.hydrateSnapshotSequenceCache(snapshot.SequenceCache)
	items := w.hydrateCanvasRowSequenceData(canvasItemsFromSnapshot(snapshot.Canvas.Items))
	state := canvasLaunchState{Items: items}
	selected := selectedCanvasRowsInOrder(state.Items)
	if len(selected) < 2 {
		t.Fatalf("snapshot %s selected rows = %d, want at least two sequence-ready rows", path, len(selected))
	}

	proteinSettings := phylo.DefaultTreeSettings()
	proteinSettings.ConversionTarget = phylo.ConversionTargetProtein
	proteinSettings.AlignmentMethod = phylo.AlignmentClustalW
	result, err := w.buildCanvasTreeArtifacts(context.Background(), state, selected, proteinSettings)
	if err != nil {
		t.Fatalf("Protein runtime probe failed: %v\nartifacts: %s", err, result.ArtifactDir)
	}
	logText := readRuntimeLogForWorkflowProbe(t, result.ArtifactDir)
	if strings.Contains(logText, "conversion.applied") || strings.Contains(logText, "converted_dna_to_protein=") {
		t.Fatalf("Protein runtime probe should not perform PHgo DNA-to-protein conversion, got:\n%s", logText)
	}

	lemnaBlastState := canvasSnapshotSelectedRowsByDatabase(items, model.CanvasKindBlast, "lemna")
	lemnaSelected := selectedCanvasRowsInOrder(lemnaBlastState.Items)
	if len(lemnaSelected) < 2 {
		t.Fatalf("snapshot %s selected Lemna BLAST rows = %d, want at least two", path, len(lemnaSelected))
	}
	dnaResolveSettings := phylo.DefaultTreeSettings()
	dnaResolveSettings.ConversionTarget = phylo.ConversionTargetDNA
	dnaResolveSettings.AlignmentMethod = phylo.AlignmentClustalW
	result, err = w.buildCanvasTreeArtifacts(context.Background(), lemnaBlastState, lemnaSelected, dnaResolveSettings)
	if err != nil {
		t.Fatalf("DNA mode should resolve selected Lemna BLAST rows from snapshot source metadata: %v\nartifacts: %s", err, result.ArtifactDir)
	}
	for _, seq := range alignedSequencesForWorkflowTest(t, result.Plan.AlignedFASTA) {
		if !looksNucleotideOnlyForWorkflowTest(strings.ReplaceAll(seq, "-", "")) {
			t.Fatalf("resolved Lemna BLAST DNA alignment should stay nucleotide:\n%s", result.Plan.AlignedFASTA)
		}
	}

	dnaState := canvasSnapshotDNAOnlyState(items)
	dnaSelected := selectedCanvasRowsInOrder(dnaState.Items)
	if len(dnaSelected) < 2 {
		t.Fatalf("snapshot %s DNA-capable selected rows = %d, want at least two", path, len(dnaSelected))
	}
	for _, method := range []phylo.AlignmentMethod{phylo.AlignmentClustalW, phylo.AlignmentMUSCLE, phylo.AlignmentClustalWCodons, phylo.AlignmentMUSCLECodons} {
		t.Run("DNA_"+string(method), func(t *testing.T) {
			settings := phylo.DefaultTreeSettings()
			settings.ConversionTarget = phylo.ConversionTargetDNA
			settings.AlignmentMethod = method
			result, err := w.buildCanvasTreeArtifacts(context.Background(), dnaState, dnaSelected, settings)
			if err != nil {
				t.Fatalf("DNA runtime probe for %s failed: %v\nartifacts: %s", method, err, result.ArtifactDir)
			}
			for _, seq := range alignedSequencesForWorkflowTest(t, result.Plan.AlignedFASTA) {
				if !looksNucleotideOnlyForWorkflowTest(strings.ReplaceAll(seq, "-", "")) {
					t.Fatalf("DNA aligned FASTA for %s should stay nucleotide:\n%s", method, result.Plan.AlignedFASTA)
				}
			}
			summary, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime-summary.txt"))
			if err != nil {
				t.Fatalf("read runtime summary: %v", err)
			}
			if !strings.Contains(string(summary), "alignment_method="+string(method)) {
				t.Fatalf("runtime summary did not record %s:\n%s", method, summary)
			}
		})
	}
}

func canvasSnapshotDNAOnlyState(items []model.CanvasItem) canvasLaunchState {
	out := cloneCanvasItems(items)
	for itemIndex := range out {
		selected := make([]bool, len(out[itemIndex].Rows))
		for rowIndex, row := range out[itemIndex].Rows {
			if row.Kind != model.CanvasKindFasta || row.FASTA == nil {
				continue
			}
			choice := canvasFastaTreeSequenceChoice(*row.FASTA, phylo.SequenceNucleotide)
			if strings.TrimSpace(choice.Sequence) != "" && choice.Kind == phylo.SequenceNucleotide {
				selected[rowIndex] = true
			}
		}
		out[itemIndex].Selected = selected
		updateCanvasItemSubtitle(&out[itemIndex])
	}
	return canvasLaunchState{Items: out}
}

func canvasSnapshotSelectedRowsByDatabase(items []model.CanvasItem, kind model.CanvasKind, database string) canvasLaunchState {
	out := cloneCanvasItems(items)
	database = strings.ToLower(strings.TrimSpace(database))
	for itemIndex := range out {
		selected := make([]bool, len(out[itemIndex].Rows))
		for rowIndex, row := range out[itemIndex].Rows {
			if row.Kind != kind {
				continue
			}
			if strings.EqualFold(normalizeSnapshotDatabase(canvasRowSourceDatabase(row)), database) {
				selected[rowIndex] = true
			}
		}
		out[itemIndex].Selected = selected
		updateCanvasItemSubtitle(&out[itemIndex])
	}
	return canvasLaunchState{Items: out}
}

func repoRootForWorkflowRuntimeProbeTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func readRuntimeLogForWorkflowProbe(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "runtime.log"))
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	return string(data)
}

func alignedSequencesForWorkflowTest(t *testing.T, fasta string) []string {
	t.Helper()
	var seqs []string
	var seq strings.Builder
	flush := func() {
		if seq.Len() == 0 {
			return
		}
		seqs = append(seqs, seq.String())
		seq.Reset()
	}
	for _, line := range strings.Split(strings.ReplaceAll(fasta, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			continue
		}
		seq.WriteString(line)
	}
	flush()
	if len(seqs) == 0 {
		t.Fatalf("expected aligned FASTA sequences")
	}
	return seqs
}

func looksNucleotideOnlyForWorkflowTest(sequence string) bool {
	sequence = strings.TrimSpace(strings.ToUpper(sequence))
	if sequence == "" {
		return false
	}
	for _, ch := range sequence {
		if !strings.ContainsRune("ACGTUNRYKMSWBDHV", ch) {
			return false
		}
	}
	return true
}

func treePanelForSnapshotTest() tui.CanvasTreePanelState {
	return tui.CanvasTreePanelState{
		EnabledEver:            true,
		CurrentControl:         0,
		DisplayNameSource:      "label_name",
		ConversionTarget:       string(phylo.ConversionTargetProtein),
		ConversionSkipUnselect: true,
		AlignmentMethod:        string(phylo.AlignmentMUSCLE),
		TreeMethod:             string(phylo.TreeNeighborJoining),
		AlignmentParams:        map[string]string{"max_iterations": "8"},
		TreeParams:             map[string]string{"distance_model": "p-distance"},
	}
}

func treeSnapshotForTest(dir string, now time.Time) sessionsnapshot.CanvasTreeV2 {
	panel := treePanelForSnapshotTest()
	record := phylo.InputRecord{TaxonID: "PHGOT000001", DisplayName: "PAL display", SequenceKind: phylo.SequenceProtein}
	meta := phylo.Metadata{SchemaVersion: 1, GeneratedAt: now, DisplayNameSource: "label_name", Records: []phylo.InputRecord{record}}
	return sessionsnapshot.CanvasTreeV2{
		PanelState:       panel,
		LastPayload:      phylo.ViewerPayload{SchemaVersion: 1, SessionID: "canvas", UpdatedAt: now, Newick: "(PHGOT000001);", AlignedFASTA: ">PHGOT000001\nMPEPTIDE\n", Metadata: meta},
		LastManifest:     treeRunManifestForSnapshotTest(now),
		LastArtifactDir:  dir,
		LastRunID:        "run1",
		LastAlignedFASTA: ">PHGOT000001\nMPEPTIDE\n",
		LastNewick:       "(PHGOT000001);",
		Fingerprints:     phylo.Fingerprints{Alignment: "a", Tree: "t"},
	}
}

func treeRunManifestForSnapshotTest(now time.Time) phylo.RunManifest {
	return phylo.RunManifest{
		SchemaVersion:   1,
		CreatedAt:       now,
		Settings:        phylo.TreeSettings{DisplayNameSource: "label_name", AlignmentMethod: phylo.AlignmentMUSCLE, AlignmentParams: map[string]string{"max_iterations": "8"}, TreeMethod: phylo.TreeNeighborJoining, TreeParams: map[string]string{"distance_model": "p-distance"}},
		Fingerprints:    phylo.Fingerprints{Alignment: "manifest-align", Tree: "manifest-tree", Preview: "manifest-preview"},
		InputFASTA:      "input.fasta",
		MetadataJSON:    "input.meta.json",
		RuntimeRequest:  "runtime-request.json",
		RuntimeResponse: "runtime-response.json",
		AlignedFASTA:    "aligned.fasta",
		NewickPath:      "tree.nwk",
	}
}

func writeTreeSnapshotRestoreArtifacts(t *testing.T, dir string, snapshot sessionsnapshot.CanvasTreeV2) {
	t.Helper()
	files := map[string]string{
		"input.fasta":           ">PHGOT000001\nMPEPTIDE\n",
		"runtime-request.json":  `{"schema_version":1,"session_id":"canvas"}`,
		"runtime-response.json": `{"schema_version":1,"runtime":"mega-phgo-runtime"}`,
		"aligned.fasta":         ">PHGOT000001\nMPEPTIDE\n",
		"tree.nwk":              "(PHGOT000001);",
	}
	meta, err := json.Marshal(snapshot.LastPayload.Metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	files["input.meta.json"] = string(meta)
	manifest, err := json.Marshal(snapshot.LastManifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	files["run.manifest.json"] = string(manifest)
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
