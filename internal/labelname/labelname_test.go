package labelname

import (
	"compress/gzip"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBestAliasPrefersCanonicalFamilyStyleOverInternalPrefix(t *testing.T) {
	if got := BestAlias("ATPAL1; PAL1"); got != "PAL1" {
		t.Fatalf("BestAlias()=%q want PAL1", got)
	}
	if got := BestAlias("CYP84A1; FAH1; F5H1"); got != "F5H1" {
		t.Fatalf("BestAlias()=%q want F5H1", got)
	}
	if got := BestAlias("CYP98A3; REF8"); got != "CYP98A3" {
		t.Fatalf("BestAlias()=%q want CYP98A3", got)
	}
}

func TestLabelFromAutoDefineFindsCompactFunctionalAlias(t *testing.T) {
	if got := LabelFromAutoDefine("(1 of 2) K09755 - ferulate-5-hydroxylase (CYP84A, F5H)"); got != "F5H" {
		t.Fatalf("LabelFromAutoDefine()=%q want F5H", got)
	}
	if got := LabelFromAutoDefine("(1 of 1) K09754 - coumaroylquinate(coumaroylshikimate) 3'-monooxygenase (CYP98A3, C3'H)"); got != "C3'H" {
		t.Fatalf("LabelFromAutoDefine()=%q want C3'H", got)
	}
}

func TestFastaHeaderLabelNamePreservesParentheticalLabel(t *testing.T) {
	tests := map[string]string{
		"Arabidopsis thaliana TAIR10|AT5G62380.1 (AtVND6)": "AtVND6",
		"Arabidopsis thaliana TAIR10|AT5G62380.1 (VND6)":   "VND6",
		"Arabidopsis thaliana TAIR10|AT5G62380.1 (ATVND6)": "ATVND6",
	}
	for input, want := range tests {
		if got := FastaHeaderLabelName(input); got != want {
			t.Fatalf("FastaHeaderLabelName(%q)=%q want %q", input, got, want)
		}
	}
}

func TestTrustedLabelPrefersCanonicalCompactSymbol(t *testing.T) {
	if got := TrustedLabel("CYP84A1", "F5H1", "LysoPL2"); got != "F5H1" {
		t.Fatalf("TrustedLabel()=%q want F5H1", got)
	}
}

func TestTrustedLabelRejectsUntrustedCandidates(t *testing.T) {
	if got := TrustedLabel("E2.3.1.133", "LysoPL2"); got != "" {
		t.Fatalf("TrustedLabel()=%q want empty", got)
	}
}

func TestAliasPreferenceScoreDoesNotPenalizeATSpeciesIdentifiers(t *testing.T) {
	if gotAt, gotOs := AliasPreferenceScore("AT1G51680"), AliasPreferenceScore("OS1G51680"); gotAt != gotOs {
		t.Fatalf("AliasPreferenceScore should not special-case AT species ids: AT=%d OS=%d", gotAt, gotOs)
	}
	if gotAt, gotZm := QueryAliasPrimarySymbolBonus("AT4CL1"), QueryAliasPrimarySymbolBonus("ZM4CL1"); gotAt != gotZm {
		t.Fatalf("QueryAliasPrimarySymbolBonus should not special-case AT species labels: AT=%d ZM=%d", gotAt, gotZm)
	}
}

func TestRankAliasBatchMatchesSingleRankingWithTrimmedDuplicateInputs(t *testing.T) {
	request := AliasRankRequest{
		TaskTimestamp: "t1",
		ItemIndex:     7,
		ProteinID:     " AT5G13930.1 ",
		GeneID:        "AT5G13930",
		Aliases:       []string{" PAL1 ", "ATPAL1", "pal1", " PAL1"},
	}
	got := RankAliasBatch([]AliasRankRequest{request, request})
	if len(got) != 2 {
		t.Fatalf("RankAliasBatch returned %d results, want 2", len(got))
	}
	want := RankAliases(request)
	for i := range got {
		if len(got[i].RankedAliases) != len(want.RankedAliases) {
			t.Fatalf("result %d aliases = %#v, want %#v", i, got[i].RankedAliases, want.RankedAliases)
		}
		for j := range want.RankedAliases {
			if got[i].RankedAliases[j] != want.RankedAliases[j] {
				t.Fatalf("result %d alias %d = %q, want %q", i, j, got[i].RankedAliases[j], want.RankedAliases[j])
			}
		}
	}
}

func TestGeneInfoDatabaseRanksToSymbolName(t *testing.T) {
	path := buildTestGeneInfoDB(t, stringsJoinLines(
		"#tax_id\tGeneID\tSymbol\tLocusTag\tSynonyms\tdbXrefs\tchromosome\tmap_location\tdescription\ttype_of_gene\tSymbol_from_nomenclature_authority\tFull_name_from_nomenclature_authority\tNomenclature_status\tOther_designations\tModification_date\tFeature_type",
		"3702\t838863\tVND6\t-\tANAC101|AtVND6\tTAIR:AT5G62380\t5\t-\tvascular NAC domain protein\tprotein-coding\tVND6\tvascular-related NAC-domain 6\tO\tNAC domain protein 101\t20260610\t-",
	))
	SetDefaultGeneInfoDatabasePath(path)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })

	got := RankAliases(AliasRankRequest{DBXrefs: []string{"TAIR:AT5G62380"}})
	if len(got.RankedAliases) == 0 || got.RankedAliases[0] != "VND6" {
		t.Fatalf("RankAliases() = %#v, want VND6 from gene_info Symbol", got.RankedAliases)
	}
}

func TestGeneInfoDatabaseMissReturnsEmpty(t *testing.T) {
	path := buildTestGeneInfoDB(t, stringsJoinLines(
		"#tax_id\tGeneID\tSymbol\tLocusTag\tSynonyms\tdbXrefs\tchromosome\tmap_location\tdescription\ttype_of_gene\tSymbol_from_nomenclature_authority\tFull_name_from_nomenclature_authority\tNomenclature_status\tOther_designations\tModification_date\tFeature_type",
		"3702\t838863\tVND6\t-\tANAC101\tTAIR:AT5G62380\t5\t-\tvascular NAC domain protein\tprotein-coding\tVND6\tvascular-related NAC-domain 6\tO\tNAC domain protein 101\t20260610\t-",
	))
	SetDefaultGeneInfoDatabasePath(path)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })

	got := RankAliases(AliasRankRequest{Aliases: []string{"NO_SUCH_SYMBOL"}})
	if len(got.RankedAliases) != 0 {
		t.Fatalf("RankAliases() = %#v, want empty when local gene_info has no match", got.RankedAliases)
	}
}

func buildTestGeneInfoDB(t testing.TB, content string) string {
	t.Helper()
	dir := t.TempDir()
	gzPath := filepath.Join(dir, "gene_info.gz")
	file, err := os.Create(gzPath)
	if err != nil {
		t.Fatalf("create gzip: %v", err)
	}
	gz := gzip.NewWriter(file)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	dbPath := filepath.Join(dir, "symbolname.pgd")
	lastModified := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if err := buildGeneInfoDatabaseFromGZ(gzPath, dbPath, GeneInfoMetadata{
		URL:             GeneInfoURL,
		LastModified:    lastModified,
		LastModifiedRaw: lastModified.Format(http.TimeFormat),
		ContentLength:   int64(len(content)),
	}, DownloadOptions{}); err != nil {
		t.Fatalf("build gene db: %v", err)
	}
	return dbPath
}

func stringsJoinLines(lines ...string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
