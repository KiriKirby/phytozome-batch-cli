package labelname

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TestParseGeneInfoDirectorySize(t *testing.T) {
	cases := map[string]int64{
		"197K": int64(197 * 1024),
		"1.4M": 1468006,
		"1.4G": 1503238553,
		"-":    0,
	}
	for input, want := range cases {
		if got := parseGeneInfoDirectorySize(input); got != want {
			t.Fatalf("parseGeneInfoDirectorySize(%q)=%d, want %d", input, got, want)
		}
	}
}

func TestGeneInfoDirectoryMetadataUsesLatestPartAndTotalSize(t *testing.T) {
	older := time.Date(2026, 6, 10, 5, 27, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 10, 5, 29, 0, 0, time.UTC)
	got := geneInfoDirectoryMetadata([]GeneInfoSourceFile{
		{Name: "All_Archaea_Bacteria.gene_info.gz", URL: "https://example.test/a.gz", LastModified: older, ContentLength: 10},
		{Name: "All_Plants.gene_info.gz", URL: "https://example.test/p.gz", LastModified: newer, ContentLength: 20},
	})
	if got.URL != GeneInfoDirectoryURL {
		t.Fatalf("URL=%q, want %q", got.URL, GeneInfoDirectoryURL)
	}
	if got.ContentLength != 30 {
		t.Fatalf("ContentLength=%d, want 30", got.ContentLength)
	}
	if !got.LastModified.Equal(newer) {
		t.Fatalf("LastModified=%s, want %s", got.LastModified, newer)
	}
	if len(got.Parts) != 2 {
		t.Fatalf("Parts=%d, want 2", len(got.Parts))
	}
}

func TestFetchGeneInfoDirectoryPartsSelectsCategorySplits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/GENE_INFO/":
			_, _ = w.Write([]byte(`<pre>Name Last modified Size <hr>
<a href="/gene/DATA/">Parent Directory</a> -
<a href="Mammalia/">Mammalia/</a> 2026-06-10 05:28 -
<a href="Plants/">Plants/</a> 2026-06-10 05:29 -
<a href="All_Data.gene_info.gz">All_Data.gene_info.gz</a> 2026-06-10 05:27 1.4G
<a href="Organelles.gene_info.gz">Organelles.gene_info.gz</a> 2026-06-10 05:29 32M
<a href="Plasmids.gene_info.gz">Plasmids.gene_info.gz</a> 2026-06-10 05:29 5.7M
</pre>`))
		case "/GENE_INFO/Mammalia/":
			_, _ = w.Write([]byte(`<pre>Name Last modified Size <hr>
<a href="/GENE_INFO/">Parent Directory</a> -
<a href="All_Mammalia.gene_info.gz">All_Mammalia.gene_info.gz</a> 2026-06-10 05:28 220M
<a href="Homo_sapiens.gene_info.gz">Homo_sapiens.gene_info.gz</a> 2026-06-10 05:28 4.9M
</pre>`))
		case "/GENE_INFO/Plants/":
			_, _ = w.Write([]byte(`<pre>Name Last modified Size <hr>
<a href="/GENE_INFO/">Parent Directory</a> -
<a href="All_Plants.gene_info.gz">All_Plants.gene_info.gz</a> 2026-06-10 05:29 189M
<a href="Arabidopsis_thaliana.gene_info.gz">Arabidopsis_thaliana.gene_info.gz</a> 2026-06-10 05:29 1.4M
</pre>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got, err := FetchGeneInfoDirectoryParts(t.Context(), server.URL+"/GENE_INFO/")
	if err != nil {
		t.Fatalf("FetchGeneInfoDirectoryParts() error = %v", err)
	}
	names := make(map[string]bool, len(got))
	for _, part := range got {
		names[part.Name] = true
	}
	for _, want := range []string{"All_Mammalia.gene_info.gz", "All_Plants.gene_info.gz", "Organelles.gene_info.gz", "Plasmids.gene_info.gz"} {
		if !names[want] {
			t.Fatalf("missing selected split %q from %#v", want, got)
		}
	}
	for _, excluded := range []string{"All_Data.gene_info.gz", "Homo_sapiens.gene_info.gz", "Arabidopsis_thaliana.gene_info.gz"} {
		if names[excluded] {
			t.Fatalf("unexpected selected split %q from %#v", excluded, got)
		}
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseFromSplitArchive(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	if _, err := gz.Write(dbData); err != nil {
		t.Fatalf("gzip db: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	archiveData := archive.Bytes()
	cut := len(archiveData) / 2
	partData := [][]byte{archiveData[:cut], archiveData[cut:]}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/symbolname.pgd.gz.part001":
			_, _ = w.Write(partData[0])
		case "/symbolname.pgd.gz.part002":
			_, _ = w.Write(partData[1])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sum := fmt.Sprintf("%x", sha256.Sum256(dbData))
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, PrebuiltGeneInfoManifest{
		SchemaVersion:      geneDBSchemaVersion,
		SHA256:             sum,
		RecordCount:        1,
		SourceURL:          GeneInfoDirectoryURL,
		SourceLastModified: "Wed, 10 Jun 2026 00:00:00 GMT",
		Parts: []PrebuiltGeneInfoPart{
			{URL: server.URL + "/symbolname.pgd.gz.part001", ContentLength: int64(len(partData[0]))},
			{URL: server.URL + "/symbolname.pgd.gz.part002", ContentLength: int64(len(partData[1]))},
		},
	}, DownloadOptions{})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	info, err := InspectGeneInfoDatabase(dest)
	if err != nil {
		t.Fatalf("InspectGeneInfoDatabase() error = %v", err)
	}
	if info.RecordCount != 1 {
		t.Fatalf("RecordCount=%d, want 1", info.RecordCount)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseFromSplitZstdArchive(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	var archive bytes.Buffer
	zw, err := zstd.NewWriter(&archive, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	if _, err := zw.Write(dbData); err != nil {
		t.Fatalf("zstd db: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zstd: %v", err)
	}
	archiveData := archive.Bytes()
	cut := len(archiveData) / 2
	partData := [][]byte{archiveData[:cut], archiveData[cut:]}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/symbolname.pgd.zst.part001":
			_, _ = w.Write(partData[0])
		case "/symbolname.pgd.zst.part002":
			_, _ = w.Write(partData[1])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sum := fmt.Sprintf("%x", sha256.Sum256(dbData))
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, PrebuiltGeneInfoManifest{
		SchemaVersion:      geneDBSchemaVersion,
		SHA256:             sum,
		RecordCount:        1,
		SourceURL:          GeneInfoDirectoryURL,
		SourceLastModified: "Wed, 10 Jun 2026 00:00:00 GMT",
		Parts: []PrebuiltGeneInfoPart{
			{URL: server.URL + "/symbolname.pgd.zst.part001", ContentLength: int64(len(partData[0]))},
			{URL: server.URL + "/symbolname.pgd.zst.part002", ContentLength: int64(len(partData[1]))},
		},
	}, DownloadOptions{})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	info, err := InspectGeneInfoDatabase(dest)
	if err != nil {
		t.Fatalf("InspectGeneInfoDatabase() error = %v", err)
	}
	if info.RecordCount != 1 {
		t.Fatalf("RecordCount=%d, want 1", info.RecordCount)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseFromManySplitZstdParts(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"3702\t2\tPAL1\tAT2G37040\tPAL1A\tGeneID:2\t-\t-\tphenylalanine ammonia-lyase 1\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"4577\t3\tC4H\tZm00001eb000010\tC4H1\tGeneID:3\t-\t-\tcinnamate 4-hydroxylase\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 37)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	manifest := splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 3)
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	var sawWorkers bool
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{
		Progress: func(event GeneInfoProgress) {
			if event.Stage == "download" && event.Workers > 1 {
				sawWorkers = true
			}
		},
	})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	if !sawWorkers {
		t.Fatal("download progress never reported multipart workers")
	}
	if got, err := os.ReadFile(dest); err != nil {
		t.Fatalf("read installed db: %v", err)
	} else if !bytes.Equal(got, dbData) {
		t.Fatal("installed database bytes do not match original database bytes")
	}
	SetDefaultGeneInfoDatabasePath(dest)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })
	got := RankAliases(AliasRankRequest{DBXrefs: []string{"GeneID:2"}})
	if len(got.RankedAliases) == 0 || got.RankedAliases[0] != "PAL1" {
		t.Fatalf("rank from reassembled db=%v, want PAL1 first", got.RankedAliases)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseRetriesTransientSplitPartFailure(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 31)
	var firstPartAttempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		if idx == 0 && firstPartAttempts.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1), DownloadOptions{})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	if firstPartAttempts.Load() < 2 {
		t.Fatalf("first part attempts=%d, want retry", firstPartAttempts.Load())
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseRejectsShortSplitPart(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 23)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		data := partData[idx]
		if idx == 1 {
			data = data[:len(data)-1]
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1), DownloadOptions{})
	if err == nil {
		t.Fatal("DownloadPrebuiltGeneInfoDatabase() error = nil, want short part error")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("error=%v, want size mismatch", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("dest should not be installed after short part, stat err=%v", statErr)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseRejectsWrongSplitOrder(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 19)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	manifest := splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1)
	manifest.Parts[0], manifest.Parts[1] = manifest.Parts[1], manifest.Parts[0]
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{})
	if err == nil {
		t.Fatal("DownloadPrebuiltGeneInfoDatabase() error = nil, want wrong order error")
	}
	if !strings.Contains(err.Error(), "zstd") && !strings.Contains(err.Error(), "sha256") && !strings.Contains(err.Error(), "magic number mismatch") {
		t.Fatalf("error=%v, want zstd, magic, or sha256 failure", err)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseFromGitHubSample(t *testing.T) {
	if os.Getenv("PHGO_TEST_GITHUB_SAMPLE") != "1" {
		t.Skip("set PHGO_TEST_GITHUB_SAMPLE=1 to verify real GitHub sample split download")
	}
	t.Setenv("PHGO_SYMBOL_NAME_PGD_MANIFEST_URL", "https://raw.githubusercontent.com/KiriKirby/phytozome-go-symbolname-db/symbolname-db-sample/symbolname/manifest.json")
	manifest, err := FetchPrebuiltGeneInfoManifest(t.Context())
	if err != nil {
		t.Fatalf("FetchPrebuiltGeneInfoManifest() error = %v", err)
	}
	if len(manifest.Parts) < 2 {
		t.Fatalf("sample manifest parts=%d, want multipart sample", len(manifest.Parts))
	}
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	info, err := InspectGeneInfoDatabase(dest)
	if err != nil {
		t.Fatalf("InspectGeneInfoDatabase() error = %v", err)
	}
	if info.RecordCount != manifest.RecordCount {
		t.Fatalf("RecordCount=%d, want manifest %d", info.RecordCount, manifest.RecordCount)
	}
	SetDefaultGeneInfoDatabasePath(dest)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })
	got := RankAliases(AliasRankRequest{DBXrefs: []string{"GeneID:1"}})
	if len(got.RankedAliases) == 0 || got.RankedAliases[0] != "VND6" {
		t.Fatalf("rank from GitHub sample db=%v, want VND6 first", got.RankedAliases)
	}
}

func TestPrebuiltPartDownloadWorkersBounds(t *testing.T) {
	if got := prebuiltPartDownloadWorkers(0); got != 1 {
		t.Fatalf("workers(0)=%d, want 1", got)
	}
	if got := prebuiltPartDownloadWorkers(3); got != 3 {
		t.Fatalf("workers(3)=%d, want 3", got)
	}
	if got := prebuiltPartDownloadWorkers(717); got != 8 {
		t.Fatalf("workers(717)=%d, want 8", got)
	}
	t.Setenv("PHGO_SYMBOL_NAME_PREBUILT_PART_WORKERS", "12")
	if got := prebuiltPartDownloadWorkers(717); got != 12 {
		t.Fatalf("configured workers=%d, want 12", got)
	}
	t.Setenv("PHGO_SYMBOL_NAME_PREBUILT_PART_WORKERS", "128")
	if got := prebuiltPartDownloadWorkers(717); got != 32 {
		t.Fatalf("configured capped workers=%d, want 32", got)
	}
}

func zstdCompressForTest(t testing.TB, data []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw, err := zstd.NewWriter(&archive, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("zstd db: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zstd: %v", err)
	}
	return archive.Bytes()
}

func splitBytesForTest(data []byte, partSize int) [][]byte {
	var parts [][]byte
	for len(data) > 0 {
		n := partSize
		if len(data) < n {
			n = len(data)
		}
		parts = append(parts, append([]byte(nil), data[:n]...))
		data = data[n:]
	}
	return parts
}

func splitManifestForTest(serverURL string, archiveName string, partData [][]byte, dbData []byte, recordCount int64) PrebuiltGeneInfoManifest {
	parts := make([]PrebuiltGeneInfoPart, len(partData))
	for idx, data := range partData {
		parts[idx] = PrebuiltGeneInfoPart{
			URL:           fmt.Sprintf("%s/%s.part%03d", serverURL, archiveName, idx+1),
			ContentLength: int64(len(data)),
		}
	}
	return PrebuiltGeneInfoManifest{
		SchemaVersion:      geneDBSchemaVersion,
		SHA256:             fmt.Sprintf("%x", sha256.Sum256(dbData)),
		RecordCount:        recordCount,
		SourceURL:          GeneInfoDirectoryURL,
		SourceLastModified: "Wed, 10 Jun 2026 00:00:00 GMT",
		Parts:              parts,
	}
}

func partIndexFromPath(t testing.TB, path string) int {
	t.Helper()
	idx := strings.LastIndex(path, ".part")
	if idx < 0 {
		t.Fatalf("missing part suffix in path %q", path)
	}
	var partNumber int
	if _, err := fmt.Sscanf(path[idx:], ".part%03d", &partNumber); err != nil {
		t.Fatalf("parse part path %q: %v", path, err)
	}
	if partNumber <= 0 {
		t.Fatalf("invalid part number in path %q", path)
	}
	return partNumber - 1
}
