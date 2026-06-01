package phylo

import (
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

func TestBuildInputUsesDisplayNameThenSourceThenHead(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	records, meta, err := BuildInput([]RowSource{
		{
			ItemTitle:    "group 1",
			ItemIndex:    0,
			RowIndex:     0,
			CanvasRow:    model.CanvasRow{DisplayName: "Tree PAL"},
			Sequence:     "MPEPTIDE",
			SourceType:   "keyword",
			OriginalHead: "AT1G01010.1",
			TableValues:  map[string]string{"label_name": "PAL1", "head": "ignored"},
		},
		{
			ItemTitle:    "group 1",
			ItemIndex:    0,
			RowIndex:     1,
			CanvasRow:    model.CanvasRow{},
			Sequence:     "ATGC",
			SourceType:   "fasta",
			OriginalHead: "",
			TableValues:  map[string]string{"label_name": "", "head": "fallback head"},
		},
		{
			ItemTitle:    "group 1",
			ItemIndex:    0,
			RowIndex:     2,
			CanvasRow:    model.CanvasRow{},
			Sequence:     "ATGC",
			SourceType:   "fasta",
			OriginalHead: "raw head",
			TableValues:  map[string]string{"label_name": ""},
		},
	}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3", len(records))
	}
	if records[0].TaxonID != "PHGOT000001" || records[1].TaxonID != "PHGOT000002" {
		t.Fatalf("unexpected stable IDs: %#v", records)
	}
	if records[0].DisplayName != "Tree PAL" {
		t.Fatalf("display name = %q, want Tree PAL", records[0].DisplayName)
	}
	if records[1].DisplayName != "fallback head" {
		t.Fatalf("source-column fallback = %q, want fallback head", records[1].DisplayName)
	}
	if records[2].DisplayName != "raw head" {
		t.Fatalf("head fallback = %q, want raw head", records[2].DisplayName)
	}
	if meta.DisplayNameSource != "label_name" || len(meta.Records) != 3 {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
}

func TestBuildInputDoesNotInferBlankSequenceKind(t *testing.T) {
	now := time.Date(2026, 5, 30, 1, 30, 0, 0, time.UTC)
	records, _, err := BuildInput([]RowSource{
		{
			ItemTitle:    "snapshot",
			RowIndex:     0,
			Sequence:     "ATGCGTATGCGT",
			SequenceKind: "",
			OriginalHead: "dna",
			TableValues:  map[string]string{"label_name": "dna"},
		},
		{
			ItemTitle:    "snapshot",
			RowIndex:     1,
			Sequence:     "MPEPTIDE",
			SequenceKind: "",
			OriginalHead: "protein",
			TableValues:  map[string]string{"label_name": "protein"},
		},
	}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	if records[0].SequenceKind != "" {
		t.Fatalf("blank DNA sequence kind = %q, want unchanged blank", records[0].SequenceKind)
	}
	if records[1].SequenceKind != "" {
		t.Fatalf("blank protein sequence kind = %q, want unchanged blank", records[1].SequenceKind)
	}
}

func TestBuildFingerprintsAreStableAndSensitiveToSettings(t *testing.T) {
	now := time.Now()
	records, _, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		CanvasRow:    model.CanvasRow{DisplayName: "PAL1"},
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	first := BuildFingerprints(records, settings, "aligned", "(PHGOT000001)")
	second := BuildFingerprints(records, settings, "aligned", "(PHGOT000001)")
	if first != second {
		t.Fatalf("fingerprints must be stable: %#v vs %#v", first, second)
	}
	settings.AlignmentParams = map[string]string{"pairwise_gap_opening_penalty": "1"}
	third := BuildFingerprints(records, settings, "aligned", "(PHGOT000001)")
	if first.Alignment == third.Alignment {
		t.Fatalf("alignment fingerprint should change when params change")
	}

	settings = DefaultTreeSettings()
	settings.ConversionSkipUnselect = !settings.ConversionSkipUnselect
	skipChanged := BuildFingerprints(records, settings, "aligned", "(PHGOT000001)")
	if first.Input != skipChanged.Input {
		t.Fatalf("conversion skip/unselect setting must not change input fingerprint")
	}
	if first.Alignment != skipChanged.Alignment || first.Tree != skipChanged.Tree {
		t.Fatalf("runtime skipped-row cleanup preference must not trigger recompute: %#v vs %#v", first, skipChanged)
	}
}

func TestBuildFingerprintsDoNotRecomputeForDisplayNameOnlyChanges(t *testing.T) {
	now := time.Now()
	row := RowSource{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		CanvasRow:    model.CanvasRow{DisplayName: "PAL1"},
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "raw header",
		TableValues:  map[string]string{"label_name": "PAL1", "species": "Athaliana", "head": "raw header"},
	}
	records, _, err := BuildInput([]RowSource{row}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	first := BuildFingerprints(records, settings, "aligned", "(PHGOT000001)")

	row.CanvasRow.DisplayName = "PAL display"
	renamed, _, err := BuildInput([]RowSource{row}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput renamed returned error: %v", err)
	}
	renamedFingerprints := BuildFingerprints(renamed, settings, "aligned", "(PHGOT000001)")
	if first.Alignment != renamedFingerprints.Alignment || first.Tree != renamedFingerprints.Tree {
		t.Fatalf("display-name edits must not trigger runtime recompute: %#v vs %#v", first, renamedFingerprints)
	}
	if first.Preview == renamedFingerprints.Preview {
		t.Fatalf("display-name edits should trigger preview refresh")
	}

	settings.DisplayNameSource = "species"
	sourceChanged := BuildFingerprints(renamed, settings, "aligned", "(PHGOT000001)")
	if first.Alignment != sourceChanged.Alignment || first.Tree != sourceChanged.Tree {
		t.Fatalf("display-name source changes must not trigger runtime recompute: %#v vs %#v", first, sourceChanged)
	}
	if renamedFingerprints.Preview == sourceChanged.Preview {
		t.Fatalf("display-name source changes should trigger preview refresh")
	}
}

func TestMarshalMetadataIncludesDisplayNames(t *testing.T) {
	meta := Metadata{
		SchemaVersion:     1,
		GeneratedAt:       time.Now().UTC(),
		DisplayNameSource: "label_name",
		Records: []InputRecord{{
			TaxonID:     "PHGOT000001",
			DisplayName: "PAL1",
		}},
	}
	data, err := MarshalMetadata(meta)
	if err != nil {
		t.Fatalf("MarshalMetadata returned error: %v", err)
	}
	if !strings.Contains(string(data), "\"display_name\": \"PAL1\"") {
		t.Fatalf("metadata json missing display_name: %s", data)
	}
}

func TestInputFASTAUsesStableTaxonIDs(t *testing.T) {
	fasta := InputFASTA([]InputRecord{
		{TaxonID: "PHGOT000001", DisplayName: "PAL1", Sequence: "MPEP TIDE\n"},
		{TaxonID: "PHGOT000002", DisplayName: "PAL2", Sequence: "ATGC"},
	})
	if strings.Contains(fasta, "PAL1") || !strings.Contains(fasta, ">PHGOT000001\nMPEPTIDE\n") || !strings.Contains(fasta, ">PHGOT000002\nATGC\n") {
		t.Fatalf("unexpected FASTA:\n%s", fasta)
	}
}

func TestInputFASTADoesNotDropEmptySequences(t *testing.T) {
	fasta := InputFASTA([]InputRecord{
		{TaxonID: "PHGOT000001", DisplayName: "empty", Sequence: ""},
		{TaxonID: "PHGOT000002", DisplayName: "filled", Sequence: "ATGC"},
	})
	if !strings.Contains(fasta, ">PHGOT000001\n") {
		t.Fatalf("empty record header was dropped:\n%s", fasta)
	}
	if !strings.Contains(fasta, ">PHGOT000002\nATGC\n") {
		t.Fatalf("filled record missing:\n%s", fasta)
	}
}

func TestBuildRunPlanProducesArtifacts(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		CanvasRow:    model.CanvasRow{DisplayName: "PAL1"},
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", t.TempDir(), DefaultTreeSettings(), SequenceProtein, records, meta, "aligned", "(PHGOT000001);", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	artifact := plan.ToArtifactSet()
	if artifact.Manifest.RuntimeRequest != "runtime-request.json" || artifact.Manifest.RuntimeResponse != "runtime-response.json" {
		t.Fatalf("expected runtime-oriented artifact contract: %#v", artifact.Manifest)
	}
}
