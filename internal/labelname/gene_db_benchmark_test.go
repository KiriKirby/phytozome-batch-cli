package labelname

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func BenchmarkBuildGeneInfoDatabase(b *testing.B) {
	content := syntheticGeneInfoRows(benchmarkGeneInfoRowCount())
	gzPath := writeBenchmarkGeneInfoGZ(b, content)
	lastModified := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	remote := GeneInfoMetadata{
		URL:             GeneInfoURL,
		LastModified:    lastModified,
		LastModifiedRaw: lastModified.Format(http.TimeFormat),
		ContentLength:   int64(len(content)),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), fmt.Sprintf("symbolname-%d.pgd", i))
		if err := buildGeneInfoDatabaseFromGZ(gzPath, dbPath, remote, DownloadOptions{Workers: DefaultBuildWorkers()}); err != nil {
			b.Fatalf("build gene db: %v", err)
		}
	}
}

func BenchmarkRankAliasBatch(b *testing.B) {
	rowCount := benchmarkGeneInfoRowCount()
	path := buildTestGeneInfoDB(b, syntheticGeneInfoRows(rowCount))
	SetDefaultGeneInfoDatabasePath(path)
	b.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })
	requests := make([]AliasRankRequest, 5000)
	for i := range requests {
		n := i%rowCount + 1
		requests[i] = AliasRankRequest{
			TaskTimestamp: "bench",
			ItemIndex:     i,
			DBXrefs:       []string{fmt.Sprintf("TAIR:AT%05dG%05d", n%5+1, n)},
			Synonyms:      []string{fmt.Sprintf("BENCHALIAS%d", n)},
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := RankAliasBatch(requests)
		if len(results) != len(requests) {
			b.Fatalf("RankAliasBatch returned %d results", len(results))
		}
	}
}

func benchmarkGeneInfoRowCount() int {
	if value := strings.TrimSpace(os.Getenv("PHGO_SYMBOL_NAME_BENCH_ROWS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 25000
}

func writeBenchmarkGeneInfoGZ(tb testing.TB, content string) string {
	tb.Helper()
	dir := tb.TempDir()
	gzPath := filepath.Join(dir, "gene_info.gz")
	file, err := os.Create(gzPath)
	if err != nil {
		tb.Fatalf("create gzip: %v", err)
	}
	gz := gzip.NewWriter(file)
	if _, err := gz.Write([]byte(content)); err != nil {
		tb.Fatalf("write gzip: %v", err)
	}
	if err := gz.Close(); err != nil {
		tb.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		tb.Fatalf("close file: %v", err)
	}
	return gzPath
}

func syntheticGeneInfoRows(count int) string {
	var builder strings.Builder
	builder.Grow(count * 220)
	builder.WriteString("#tax_id\tGeneID\tSymbol\tLocusTag\tSynonyms\tdbXrefs\tchromosome\tmap_location\tdescription\ttype_of_gene\tSymbol_from_nomenclature_authority\tFull_name_from_nomenclature_authority\tNomenclature_status\tOther_designations\tModification_date\tFeature_type\n")
	for i := 1; i <= count; i++ {
		family := i % 100
		fmt.Fprintf(
			&builder,
			"3702\t%d\tBENCH%d_%03d\tLOC%05d\tBENCHALIAS%d|AT%dG%05d.1\tTAIR:AT%dG%05d|GeneID:%d\t%d\t-\tbenchmark protein family %d\tprotein-coding\tBENCH%d_%03d\tbenchmark full name %d\tO\tbenchmark designation %d\t20260610\t-\n",
			900000+i,
			family,
			i,
			i,
			i,
			i%5+1,
			i,
			i%5+1,
			i,
			900000+i,
			i%5+1,
			family,
			family,
			i,
			i,
			i,
		)
	}
	return builder.String()
}
