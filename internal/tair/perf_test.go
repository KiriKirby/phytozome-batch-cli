package tair

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

func BenchmarkTAIRFastaHeaderCache(b *testing.B) {
	path := writeTAIRPerfFASTA(b, 2500, 180)
	client := NewClient(nil)

	b.Run("cold-parse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, err := os.Open(path)
			if err != nil {
				b.Fatal(err)
			}
			_, err = parseFastaHeaderIndex(f)
			_ = f.Close()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("warm-cache", func(b *testing.B) {
		if _, err := client.loadFastaHeaders(context.Background(), path); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := client.loadFastaHeaders(context.Background(), path); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("warm-cache-shared", func(b *testing.B) {
		if _, err := client.loadFastaHeadersShared(context.Background(), path); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := client.loadFastaHeadersShared(context.Background(), path); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTAIRQuerySourceCacheHit(b *testing.B) {
	client := NewClient(nil)
	version := defaultVersionForTest()
	source := &model.QuerySequenceSource{
		Sequence:          "MPEPTIDE",
		SourceDatabase:    "tair",
		SourceProteomeID:  version.ProteomeID,
		SourceJBrowseName: version.JBrowseName,
		GeneID:            "AT1G01010",
		TranscriptID:      "AT1G01010.1",
		ProteinID:         "AT1G01010.1",
	}
	client.storeQuerySource(querySourceCacheKey(version, "gene", "AT1G01010"), source)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, ok, err := client.ResolveQuerySequence(context.Background(), version, "AT1G01010")
		if err != nil || !ok || resolved == nil || resolved.Sequence == "" {
			b.Fatalf("cache resolve failed: ok=%v source=%#v err=%v", ok, resolved, err)
		}
	}
}

func TestTAIRPerformanceSweepLog(t *testing.T) {
	if os.Getenv("PHGO_TAIR_PERF_SWEEP") == "" {
		t.Skip("set PHGO_TAIR_PERF_SWEEP=1 to run local TAIR performance sweep")
	}

	path := writeTAIRPerfFASTA(t, configuredIntForTAIRPerf("PHGO_TAIR_PERF_FASTA_RECORDS", 1200), configuredIntForTAIRPerf("PHGO_TAIR_PERF_FASTA_LENGTH", 240))
	client := NewClient(nil)

	start := time.Now()
	cold, err := parseTAIRPerfFastaHeaderFile(path)
	if err != nil {
		t.Fatal(err)
	}
	coldDuration := time.Since(start)

	start = time.Now()
	warm1, err := client.loadFastaHeaders(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	firstCacheDuration := time.Since(start)

	start = time.Now()
	warm2, err := client.loadFastaHeaders(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	secondCacheDuration := time.Since(start)

	version := defaultVersionForTest()
	source := &model.QuerySequenceSource{Sequence: "MPEPTIDE", SourceDatabase: "tair", SourceProteomeID: version.ProteomeID, SourceJBrowseName: version.JBrowseName, GeneID: "AT1G01010", TranscriptID: "AT1G01010.1", ProteinID: "AT1G01010.1"}
	client.storeQuerySource(querySourceCacheKey(version, "gene", "AT1G01010"), source)
	resolveStart := time.Now()
	for i := 0; i < 1000; i++ {
		resolved, ok, err := client.ResolveQuerySequence(context.Background(), version, "AT1G01010")
		if err != nil || !ok || resolved == nil {
			t.Fatalf("query source cache resolve failed: ok=%v err=%v", ok, err)
		}
	}
	resolveDuration := time.Since(resolveStart)

	t.Logf(
		"tair_perf local_fasta=%s entries=%d cold_parse_ms=%d first_cache_ms=%d warm_cache_ms=%d query_cache_1000_ms=%d cpu=%d local_blast_threads_default=%d env_threads=%s env_workers=%s",
		path,
		len(cold),
		coldDuration.Milliseconds(),
		firstCacheDuration.Milliseconds(),
		secondCacheDuration.Milliseconds(),
		resolveDuration.Milliseconds(),
		runtime.NumCPU(),
		localBlastThreads(context.Background()),
		strings.TrimSpace(os.Getenv("PHYTOZOME_GO_LOCAL_BLAST_THREADS")),
		strings.TrimSpace(os.Getenv("PHYTOZOME_GO_MAX_WORKERS")),
	)
	if len(cold) != len(warm1) || len(warm1) != len(warm2) {
		t.Fatalf("cache changed index sizes: cold=%d warm1=%d warm2=%d", len(cold), len(warm1), len(warm2))
	}
}

func TestTAIRLocalBlastThreadSweep(t *testing.T) {
	if os.Getenv("PHGO_TAIR_BLAST_THREAD_SWEEP") == "" {
		t.Skip("set PHGO_TAIR_BLAST_THREAD_SWEEP=1 to run local TAIR BLAST thread sweep")
	}
	if _, err := exec.LookPath("makeblastdb"); err != nil {
		t.Skip("makeblastdb not available")
	}
	if _, err := exec.LookPath("blastp"); err != nil {
		t.Skip("blastp not available")
	}

	dir := t.TempDir()
	fastaPath := filepath.Join(dir, "tair-perf.fa")
	if err := os.WriteFile(fastaPath, []byte(tairPerfFASTA(800, 240)), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPrefix := filepath.Join(dir, "tair_perf_db")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := ensureBlastDB(ctx, fastaPath, dbPrefix, "prot"); err != nil {
		t.Fatal(err)
	}
	fastaIndex, err := parseTAIRPerfFastaHeaderFile(fastaPath)
	if err != nil {
		t.Fatal(err)
	}
	req := model.BlastRequest{
		Program:          "blastp",
		Sequence:         ">query\n" + strings.Repeat("M", 120) + "\n",
		EValue:           "1e-5",
		ComparisonMatrix: "BLOSUM62",
		AlignmentsToShow: 20,
		AllowGaps:        true,
		FilterQuery:      false,
	}

	for _, threads := range tairPerfThreadSweepValues() {
		runCtx := WithLocalBlastThreads(ctx, threads)
		start := time.Now()
		result, err := runBlastAndParse(runCtx, "blastp", dbPrefix, fastaIndex, req)
		duration := time.Since(start)
		if err != nil {
			t.Fatalf("blastp threads=%d: %v", threads, err)
		}
		t.Logf("tair_blast_thread_sweep threads=%d rows=%d ms=%d", threads, len(result.Rows), duration.Milliseconds())
	}
}

func TestTAIR11ZenodoProbeLog(t *testing.T) {
	if os.Getenv("PHGO_TAIR11_ZENODO_PROBE") == "" {
		t.Skip("set PHGO_TAIR11_ZENODO_PROBE=1 to run TAIR11 Zenodo probe")
	}
	targets := []string{
		"https://zenodo.org/api/records/17371665/files/gene_aliases_20241001.txt.gz/content",
		"https://zenodo.org/api/records/17371665/files/Araport11_functional_descriptions_20241001.txt.gz/content",
		"https://zenodo.org/api/records/17371665/files/Araport11_GFF3_genes_transposons.20241001.gff.gz/content",
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for _, level := range []int{8, 16, 32, 64} {
		start := time.Now()
		type result struct{ status int }
		results := make(chan result, len(targets)*level)
		var wg sync.WaitGroup
		for _, u := range targets {
			for i := 0; i < level; i++ {
				wg.Add(1)
				go func(rawURL string) {
					defer wg.Done()
					req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
					if err != nil {
						results <- result{status: -1}
						return
					}
					req.Header.Set("User-Agent", "phytozome-go TAIR11 Zenodo probe")
					resp, err := client.Do(req)
					if err != nil {
						results <- result{status: -1}
						return
					}
					_, _ = io.CopyN(io.Discard, resp.Body, 32*1024)
					_ = resp.Body.Close()
					results <- result{status: resp.StatusCode}
				}(u)
			}
		}
		wg.Wait()
		close(results)
		ok := 0
		failed := 0
		statuses := map[int]int{}
		for item := range results {
			statuses[item.status]++
			if item.status == http.StatusOK {
				ok++
			} else {
				failed++
			}
		}
		t.Logf("tair11_zenodo_probe concurrency=%d total=%d ok=%d failed=%d statuses=%v elapsed_ms=%d", level, len(targets)*level, ok, failed, statuses, time.Since(start).Milliseconds())
	}
}

func writeTAIRPerfFASTA(tb testing.TB, records int, length int) string {
	tb.Helper()
	dir := tb.TempDir()
	path := filepath.Join(dir, "tair-perf.fa")
	if err := os.WriteFile(path, []byte(tairPerfFASTA(records, length)), 0o600); err != nil {
		tb.Fatal(err)
	}
	return path
}

func tairPerfFASTA(records int, length int) string {
	if records < 1 {
		records = 1
	}
	if length < 1 {
		length = 1
	}
	var b strings.Builder
	for i := 0; i < records; i++ {
		gene := fmt.Sprintf("AT1G%05d", i+1)
		b.WriteString(">")
		b.WriteString(gene)
		b.WriteString(".1 Symbols: PERF")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(" | synthetic TAIR perf protein\n")
		b.WriteString(wrapFASTA(strings.Repeat("M", length)))
	}
	return b.String()
}

func parseTAIRPerfFastaHeaderFile(path string) (map[string]fastaEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseFastaHeaderIndex(f)
}

func tairPerfThreadSweepValues() []int {
	maxThreads := configuredIntForTAIRPerf("PHGO_TAIR_BLAST_MAX_THREADS", minInt(16, runtime.NumCPU()))
	if maxThreads < 1 {
		maxThreads = 1
	}
	values := []int{1}
	for n := 2; n < maxThreads; n *= 2 {
		values = append(values, n)
	}
	if values[len(values)-1] != maxThreads {
		values = append(values, maxThreads)
	}
	return values
}

func configuredIntForTAIRPerf(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
