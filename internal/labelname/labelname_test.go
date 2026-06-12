package labelname

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
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
		"3702\t836359\tNAC101\tAT5G62380\tANAC101|MMI9.6|MMI9_6|NAC-domain protein 101|VASCULAR-RELATED NAC-DOMAIN 6|VND6\tAraport:AT5G62380|TAIR:AT5G62380\t5\t-\tNAC-domain protein 101\tprotein-coding\tNAC101\tNAC-domain protein 101\tO\tNAC-domain protein 101\t20260513\t-",
	))
	SetDefaultGeneInfoDatabasePath(path)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })

	got := RankAliases(AliasRankRequest{Aliases: []string{"VND6"}, DBXrefs: []string{"TAIR:AT5G62380"}})
	if len(got.RankedAliases) == 0 || got.RankedAliases[0] != "VND6" {
		t.Fatalf("RankAliases() = %#v, want VND6 from gene_info Synonyms", got.RankedAliases)
	}
	assertBeforeAlias(t, got.RankedAliases, "VND6", "NAC-domain protein 101")
	assertBeforeAlias(t, got.RankedAliases, "ANAC101", "MMI9.6")
}

func TestGeneInfoDatabaseRanksLocusToAuthoritySymbol(t *testing.T) {
	path := buildTestGeneInfoDB(t, stringsJoinLines(
		"#tax_id\tGeneID\tSymbol\tLocusTag\tSynonyms\tdbXrefs\tchromosome\tmap_location\tdescription\ttype_of_gene\tSymbol_from_nomenclature_authority\tFull_name_from_nomenclature_authority\tNomenclature_status\tOther_designations\tModification_date\tFeature_type",
		"3702\t818280\tPAL1\tAT2G37040\tATPAL1|CI0004|PHE ammonia lyase 1|T1J8.22|T1J8_22\tAraport:AT2G37040|TAIR:AT2G37040\t2\t-\tPHE ammonia lyase 1\tprotein-coding\tPAL1\tPHE ammonia lyase 1\tO\tPHE ammonia lyase 1\t20260610\t-",
	))
	SetDefaultGeneInfoDatabasePath(path)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })

	got := RankAliases(AliasRankRequest{LocusTag: "AT2G37040", DBXrefs: []string{"TAIR:AT2G37040"}})
	if len(got.RankedAliases) == 0 || got.RankedAliases[0] != "PAL1" {
		t.Fatalf("RankAliases() = %#v, want PAL1 first", got.RankedAliases)
	}
	assertBeforeAlias(t, got.RankedAliases, "PAL1", "PHE ammonia lyase 1")
	assertBeforeAlias(t, got.RankedAliases, "ATPAL1", "T1J8.22")
}

func TestSortRankedAliasesAppendsNonPrimarySymbolNamesAlphabetically(t *testing.T) {
	items := []rankedAlias{
		{Text: "beta protein", Score: 999},
		{Text: "ANAC101", Score: 20, Family: "ANAC"},
		{Text: "MMI9.6", Score: 999, Family: "MMI"},
		{Text: "VND6", Score: 40, Family: "VND"},
		{Text: "alpha protein", Score: 1},
	}
	got := rankedAliasTexts(sortRankedAliases(items, nil))
	want := []string{"VND6", "ANAC101", "alpha protein", "beta protein", "MMI9.6"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("sorted aliases = %#v, want %#v", got, want)
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
	dbPath := filepath.Join(dir, "symbolname.pgd")
	lastModified := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	writeTestGeneInfoDB(t, dbPath, content, GeneInfoMetadata{
		URL:             "https://example.test/GENE_INFO/",
		LastModified:    lastModified,
		LastModifiedRaw: lastModified.Format(http.TimeFormat),
		ContentLength:   int64(len(content)),
	})
	return dbPath
}

func writeTestGeneInfoDB(t testing.TB, dbPath string, content string, remote GeneInfoMetadata) {
	t.Helper()
	db, err := bolt.Open(dbPath, 0o644, &bolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("create test symbol name db: %v", err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{geneDBBucketMeta, geneDBBucketRecords, geneDBBucketIndex} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("create test db buckets: %v", err)
	}
	var count uint64
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		record, ok := parseGeneInfoLine(line)
		if !ok {
			continue
		}
		count++
		record.ID = count
		if err := db.Update(func(tx *bolt.Tx) error {
			records := tx.Bucket([]byte(geneDBBucketRecords))
			index := tx.Bucket([]byte(geneDBBucketIndex))
			if err := records.Put(u64key(record.ID), encodeGeneRecord(record)); err != nil {
				return err
			}
			for _, term := range record.indexTerms() {
				if err := putIndexTerm(index, term.Key, record.ID, term.Weight); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("write test record: %v", err)
		}
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(geneDBBucketMeta))
		values := map[string]string{
			"schema_version": geneDBSchemaVersion,
			"url":            remote.URL,
			"last_modified":  remote.LastModifiedRaw,
			"downloaded_at":  time.Now().UTC().Format(time.RFC3339Nano),
			"content_length": strconv.FormatInt(remote.ContentLength, 10),
			"record_count":   strconv.FormatUint(count, 10),
		}
		for key, value := range values {
			if err := meta.Put([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("write test metadata: %v", err)
	}
}

func stringsJoinLines(lines ...string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}

func assertBeforeAlias(t testing.TB, aliases []string, left string, right string) {
	t.Helper()
	leftIndex, rightIndex := -1, -1
	for i, alias := range aliases {
		if alias == left && leftIndex < 0 {
			leftIndex = i
		}
		if alias == right && rightIndex < 0 {
			rightIndex = i
		}
	}
	if leftIndex < 0 {
		t.Fatalf("alias %q not found in %#v", left, aliases)
	}
	if rightIndex < 0 {
		t.Fatalf("alias %q not found in %#v", right, aliases)
	}
	if leftIndex >= rightIndex {
		t.Fatalf("alias order in %#v: want %q before %q", aliases, left, right)
	}
}
