package labelname

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

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
