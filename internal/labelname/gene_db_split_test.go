package labelname

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
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

const fullGitHubDBEnv = "PHGO_TEST_GITHUB_FULL_SYMBOLNAME"
const realOutputEnv = "PHGO_TEST_REAL_OUTPUT_DIR"

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
		SourceURL:          "https://example.test/GENE_INFO/",
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
		SourceURL:          "https://example.test/GENE_INFO/",
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

func TestDownloadPrebuiltGeneInfoDatabaseHandlesArbitrarySplitPartCount(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"3702\t2\tPAL1\tAT2G37040\tPAL1A\tGeneID:2\t-\t-\tphenylalanine ammonia-lyase 1\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"4577\t3\tC4H\tZm00001eb000010\tC4H1\tGeneID:3\t-\t-\tcinnamate 4-hydroxylase\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"4577\t4\tF5H1\tZm00001eb000020\tF5H\tGeneID:4\t-\t-\tferulate 5-hydroxylase\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 11)
	if len(partData) <= 3 {
		t.Fatalf("test archive split into %d parts, want more than 3", len(partData))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		if idx%2 == 0 {
			time.Sleep(15 * time.Millisecond)
		}
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	manifest := splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 4)
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{})
	if err != nil {
		t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
	}
	if got, err := os.ReadFile(dest); err != nil {
		t.Fatalf("read installed db: %v", err)
	} else if !bytes.Equal(got, dbData) {
		t.Fatal("installed database bytes do not match original database bytes")
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

func TestDownloadPrebuiltGeneInfoDatabaseSplitFailureDoesNotLoseStagedPartPaths(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"3702\t2\tPAL1\tAT2G37040\tPAL1A\tGeneID:2\t-\t-\tphenylalanine ammonia-lyase 1\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := partIndexFromPath(t, r.URL.Path)
		if idx == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1), DownloadOptions{})
	if err == nil {
		t.Fatal("DownloadPrebuiltGeneInfoDatabase returned nil error, want failure")
	}
	if strings.Contains(err.Error(), "The system cannot find the path specified") || strings.Contains(strings.ToLower(err.Error()), "cannot find the path") {
		t.Fatalf("download failure exposed deleted staged part path race: %v", err)
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

func TestDownloadPrebuiltGeneInfoDatabaseDoesNotRejectLargeDeclaredSplitPartBeforeRead(t *testing.T) {
	dbPath := buildTestGeneInfoDB(t, stringsJoinLines(
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
	))
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source db: %v", err)
	}
	archiveData := zstdCompressForTest(t, dbData)
	partData := splitBytesForTest(archiveData, 23)
	var sawRequest atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest.Store(true)
		idx := partIndexFromPath(t, r.URL.Path)
		_, _ = w.Write(partData[idx])
	}))
	defer server.Close()

	manifest := splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1)
	manifest.Parts[0].ContentLength = 1024 * 1024 * 1024
	dest := filepath.Join(t.TempDir(), "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{})
	if err == nil {
		t.Fatal("DownloadPrebuiltGeneInfoDatabase() error = nil, want size mismatch")
	}
	if !sawRequest.Load() {
		t.Fatal("large declared part was rejected before the server was contacted")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("error=%v, want size mismatch", err)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseCleansStagedPartsAfterFailure(t *testing.T) {
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
		data := partData[idx]
		if idx == 0 {
			data = data[:len(data)-1]
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "symbolname.pgd")
	err = DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, splitManifestForTest(server.URL, "symbolname.pgd.zst", partData, dbData, 1), DownloadOptions{})
	if err == nil {
		t.Fatal("DownloadPrebuiltGeneInfoDatabase() error = nil, want failure")
	}
	matches, globErr := filepath.Glob(filepath.Join(destDir, "symbolname-parts-*"))
	if globErr != nil {
		t.Fatalf("glob staged part dirs: %v", globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("staged part dirs were not cleaned: %v", matches)
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

func TestGitHubRetryAfterParsesOfficialRateLimitHeaders(t *testing.T) {
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	header := http.Header{}
	header.Set("Retry-After", "3")
	if got := githubRetryAfter(header, now); got != 3*time.Second {
		t.Fatalf("Retry-After seconds=%s, want 3s", got)
	}
	header = http.Header{}
	header.Set("X-RateLimit-Remaining", "0")
	header.Set("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(7*time.Second).Unix()))
	if got := githubRetryAfter(header, now); got != 7*time.Second {
		t.Fatalf("X-RateLimit-Reset delay=%s, want 7s", got)
	}
	header.Set("X-RateLimit-Remaining", "1")
	if got := githubRetryAfter(header, now); got != 0 {
		t.Fatalf("non-exhausted X-RateLimit-Reset delay=%s, want 0", got)
	}
}

func TestDownloadPrebuiltGeneInfoDatabaseRetriesForbiddenWithRetryAfter(t *testing.T) {
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
			w.Header().Set("Retry-After", "1")
			http.Error(w, "secondary rate limit", http.StatusForbidden)
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

func TestCopyPrebuiltGeneInfoPartToWriterDetectsStagedSizeChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "part")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write part: %v", err)
	}
	err := copyPrebuiltGeneInfoPartToWriter(io.Discard, 0, path, 4)
	if err == nil {
		t.Fatal("copyPrebuiltGeneInfoPartToWriter() error = nil, want size change")
	}
	if !strings.Contains(err.Error(), "staged part size changed") {
		t.Fatalf("error=%v, want staged part size changed", err)
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

func TestDownloadPrebuiltGeneInfoDatabaseFromGitHubFull(t *testing.T) {
	if os.Getenv(fullGitHubDBEnv) != "1" {
		t.Skip("set PHGO_TEST_GITHUB_FULL_SYMBOLNAME=1 to download and verify the full GitHub symbol name database")
	}
	manifest, err := FetchPrebuiltGeneInfoManifest(t.Context())
	if err != nil {
		t.Fatalf("FetchPrebuiltGeneInfoManifest() error = %v", err)
	}
	dest := fullGitHubSymbolNameDBPath(t)
	if info, err := InspectGeneInfoDatabase(dest); err == nil &&
		(manifest.RecordCount <= 0 || info.RecordCount == manifest.RecordCount) &&
		(strings.TrimSpace(manifest.SourceLastModified) == "" || info.LastModifiedRaw == manifest.SourceLastModified) {
		t.Logf("reusing verified full symbol name database: %s", dest)
	} else {
		start := time.Now()
		t.Logf("downloading full symbol name database to %s (%s compressed)", dest, formatBytes(manifest.downloadSize()))
		if err := DownloadPrebuiltGeneInfoDatabase(t.Context(), dest, manifest, DownloadOptions{
			Progress: func(event GeneInfoProgress) {
				if event.Stage == "download" && event.TotalBytes > 0 && event.CurrentBytes > 0 {
					t.Logf("%s", FormatGeneInfoProgress(event))
				}
			},
		}); err != nil {
			t.Fatalf("DownloadPrebuiltGeneInfoDatabase() error = %v", err)
		}
		t.Logf("full symbol name database download/install finished in %s", time.Since(start).Round(time.Second))
	}
	info, err := InspectGeneInfoDatabase(dest)
	if err != nil {
		t.Fatalf("InspectGeneInfoDatabase() error = %v", err)
	}
	if manifest.RecordCount > 0 && info.RecordCount != manifest.RecordCount {
		t.Fatalf("RecordCount=%d, want manifest %d", info.RecordCount, manifest.RecordCount)
	}
	SetDefaultGeneInfoDatabasePath(dest)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })
	cases := []AliasRankRequest{
		{DBXrefs: []string{"GeneID:838863", "TAIR:AT5G62380"}, Synonyms: []string{"ANAC101"}},
		{Aliases: []string{"PAL1"}, DBXrefs: []string{"TAIR:AT2G37040"}},
		{Aliases: []string{"CYP73A5", "C4H"}, SearchTerm: "cinnamate 4-hydroxylase"},
	}
	results := RankAliasBatch(cases)
	for i, result := range results {
		if len(result.RankedAliases) == 0 {
			t.Fatalf("full DB result %d is empty for request %#v", i, cases[i])
		}
		t.Logf("full DB result %d: %v", i, result.RankedAliases[:minIntForTest(3, len(result.RankedAliases))])
	}
	if !containsAliasForTest(results[0].RankedAliases, "VND6") || looksLikeLocusForTest(results[0].RankedAliases[0]) {
		t.Fatalf("full DB VND6/NAC result = %v, want symbol-name aliases before locus-like values", results[0].RankedAliases[:minIntForTest(8, len(results[0].RankedAliases))])
	}
	if results[1].RankedAliases[0] != "PAL1" {
		t.Fatalf("full DB PAL result = %v, want PAL1 first", results[1].RankedAliases[:minIntForTest(8, len(results[1].RankedAliases))])
	}
	if !containsAliasForTest(results[2].RankedAliases, "C4H") || looksLikeLocusForTest(results[2].RankedAliases[0]) {
		t.Fatalf("full DB C4H result = %v, want C4H-family symbol names before locus-like values", results[2].RankedAliases[:minIntForTest(8, len(results[2].RankedAliases))])
	}
}

func TestRankAliasBatchPerformanceWithGitHubFullDatabase(t *testing.T) {
	if os.Getenv(fullGitHubDBEnv) != "1" {
		t.Skip("set PHGO_TEST_GITHUB_FULL_SYMBOLNAME=1 to run full DB ranking performance checks")
	}
	dest := fullGitHubSymbolNameDBPath(t)
	if _, err := InspectGeneInfoDatabase(dest); err != nil {
		t.Fatalf("full symbol name database is not ready at %s: %v", dest, err)
	}
	SetDefaultGeneInfoDatabasePath(dest)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })
	base := []AliasRankRequest{
		{Aliases: []string{"PAL1"}, DBXrefs: []string{"TAIR:AT2G37040"}, SearchTerm: "phenylalanine ammonia-lyase"},
		{Aliases: []string{"C4H", "CYP73A5"}, SearchTerm: "cinnamate 4-hydroxylase"},
		{Aliases: []string{"F5H1", "CYP84A1"}, SearchTerm: "ferulate 5-hydroxylase"},
		{Aliases: []string{"LAC2", "TT10"}, SearchTerm: "laccase"},
		{Aliases: []string{"VND6", "ANAC101"}, DBXrefs: []string{"TAIR:AT5G62380"}},
		{Aliases: []string{"4CLL4"}, SearchTerm: "4-coumarate CoA ligase"},
	}
	requests := make([]AliasRankRequest, 0, 240)
	for i := 0; i < 40; i++ {
		for _, request := range base {
			request.TaskTimestamp = "full-perf"
			request.ItemIndex = len(requests)
			requests = append(requests, request)
		}
	}
	start := time.Now()
	first := RankAliasBatch(requests)
	firstElapsed := time.Since(start)
	start = time.Now()
	second := RankAliasBatch(requests)
	secondElapsed := time.Since(start)
	if len(first) != len(requests) || len(second) != len(requests) {
		t.Fatalf("RankAliasBatch results lengths = %d/%d, want %d", len(first), len(second), len(requests))
	}
	for i := range first {
		if strings.Join(first[i].RankedAliases, "\x00") != strings.Join(second[i].RankedAliases, "\x00") {
			t.Fatalf("cached result %d differs: first=%v second=%v", i, first[i].RankedAliases, second[i].RankedAliases)
		}
	}
	t.Logf("full DB RankAliasBatch %d requests: first=%s cached=%s", len(requests), firstElapsed, secondElapsed)
}

func TestRankAliasBatchWithFullDatabaseRealOutputHeaders(t *testing.T) {
	if os.Getenv(fullGitHubDBEnv) != "1" {
		t.Skip("set PHGO_TEST_GITHUB_FULL_SYMBOLNAME=1 to run full DB real-output ranking checks")
	}
	dest := fullGitHubSymbolNameDBPath(t)
	if _, err := InspectGeneInfoDatabase(dest); err != nil {
		t.Fatalf("full symbol name database is not ready at %s: %v", dest, err)
	}
	outputDir := realOutputDirForTest()
	if info, err := os.Stat(outputDir); err != nil || !info.IsDir() {
		t.Skipf("real output directory is not available: %s", outputDir)
	}
	SetDefaultGeneInfoDatabasePath(dest)
	t.Cleanup(func() { SetDefaultGeneInfoDatabasePath("") })

	sourceBatch := realOutputAliasRequestsFromFasta(t, outputDir, "Monolignol Biosynthesis.fasta", 64)
	if len(sourceBatch) < 6 {
		t.Fatalf("source batch headers=%d, want at least 6", len(sourceBatch))
	}
	sourceResults := RankAliasBatch(sourceBatch)
	assertRealOutputTopAlias(t, sourceResults, "PAL1")
	assertRealOutputTopAlias(t, sourceResults, "C4H")
	assertRealOutputTopAlias(t, sourceResults, "FAH1")
	assertRealOutputTopAlias(t, sourceResults, "4CL1")
	assertRealOutputTopAlias(t, sourceResults, "4CL2")
	assertNoLocusFirstForTest(t, sourceResults)

	tableBatch := realOutputAliasRequestsFromFasta(t, outputDir, filepath.Join("Monolignol_Biosynthesis", "4CL.fasta"), 12)
	if len(tableBatch) < 4 {
		t.Fatalf("4CL table headers=%d, want at least 4", len(tableBatch))
	}
	tableResults := RankAliasBatch(tableBatch)
	assertRealOutputTopAlias(t, tableResults, "4CL1")
	assertRealOutputTopAlias(t, tableResults, "4CL2")
	assertNoLocusFirstForTest(t, tableResults)

	c4hBatch := realOutputAliasRequestsFromFasta(t, outputDir, filepath.Join("Monolignol_Biosynthesis", "C4H.fasta"), 8)
	if len(c4hBatch) < 2 {
		t.Fatalf("C4H table headers=%d, want at least 2", len(c4hBatch))
	}
	c4hResults := RankAliasBatch(c4hBatch)
	assertRealOutputTopAlias(t, c4hResults, "C4H")
	assertNoLocusFirstForTest(t, c4hResults)
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

func fullGitHubSymbolNameDBPath(t testing.TB) string {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv("PHGO_TEST_GITHUB_FULL_SYMBOLNAME_PATH")); value != "" {
		return filepath.Clean(value)
	}
	dir := filepath.Join(os.TempDir(), "phytozome-go-symbolname-full")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create full symbol name test cache: %v", err)
	}
	return filepath.Join(dir, DefaultGeneInfoPGD)
}

func realOutputDirForTest() string {
	if value := strings.TrimSpace(os.Getenv(realOutputEnv)); value != "" {
		return filepath.Clean(value)
	}
	return filepath.Clean(`C:\Users\wangsychn\OneDrive - Kyoto University\研\2026年5月14日\output`)
}

func realOutputAliasRequestsFromFasta(t testing.TB, outputDir string, relativePath string, limit int) []AliasRankRequest {
	t.Helper()
	path := filepath.Join(outputDir, relativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read real output FASTA %s: %v", path, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	requests := make([]AliasRankRequest, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ">") {
			continue
		}
		request, ok := aliasRankRequestFromRealOutputHeader(line)
		if !ok {
			continue
		}
		request.TaskTimestamp = "real-output"
		request.ItemIndex = len(requests)
		requests = append(requests, request)
		if limit > 0 && len(requests) >= limit {
			break
		}
	}
	return requests
}

func aliasRankRequestFromRealOutputHeader(header string) (AliasRankRequest, bool) {
	header = strings.TrimSpace(strings.TrimPrefix(header, ">"))
	if !strings.HasPrefix(strings.ToLower(header), "phgo://") {
		return AliasRankRequest{}, false
	}
	body := strings.TrimSpace(header[len("phgo://"):])
	groups := strings.Split(body, `\`)
	mainParts := strings.SplitN(groups[0], "/", 3)
	if len(mainParts) < 3 {
		return AliasRankRequest{}, false
	}
	request := AliasRankRequest{
		SearchTerm: strings.TrimSpace(mainParts[1]),
		Aliases:    []string{strings.TrimSpace(mainParts[1])},
	}
	fillIDFieldsForTest(&request, strings.TrimSpace(mainParts[2]))
	if len(groups) >= 2 && strings.Contains(groups[1], "/") {
		sourceParts := strings.SplitN(groups[1], "/", 2)
		if len(sourceParts) >= 1 {
			request.Aliases = append(request.Aliases, strings.TrimSpace(sourceParts[0]))
		}
		if len(sourceParts) >= 2 {
			fillIDFieldsForTest(&request, strings.TrimSpace(sourceParts[1]))
		}
	}
	request.Aliases = uniqueStrings(request.Aliases)
	return request, len(request.Aliases) > 0 || request.GeneID != "" || request.ProteinID != ""
}

func fillIDFieldsForTest(request *AliasRankRequest, value string) {
	value = strings.TrimSpace(value)
	if value == "" || value == "~" {
		return
	}
	request.ProteinID = firstNonEmptyForTest(request.ProteinID, value)
	request.SequenceID = firstNonEmptyForTest(request.SequenceID, value)
	geneID := stripTranscriptSuffixForTest(value)
	request.GeneID = firstNonEmptyForTest(request.GeneID, geneID)
	request.LocusTag = firstNonEmptyForTest(request.LocusTag, geneID)
	request.DBXrefs = append(request.DBXrefs, value, geneID, "TAIR:"+geneID, "Araport:"+geneID)
}

func stripTranscriptSuffixForTest(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.LastIndex(value, "."); i > 0 && i+1 < len(value) {
		allDigits := true
		for _, r := range value[i+1:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return value[:i]
		}
	}
	if i := strings.LastIndex(value, "_T"); i > 0 && i+2 < len(value) {
		allDigits := true
		for _, r := range value[i+2:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return value[:i]
		}
	}
	return value
}

func firstNonEmptyForTest(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func assertRealOutputTopAlias(t testing.TB, results []AliasRankResult, want string) {
	t.Helper()
	for _, result := range results {
		if len(result.RankedAliases) == 0 {
			continue
		}
		if result.RankedAliases[0] == want {
			t.Logf("real output %s aliases: %v", want, result.RankedAliases[:minIntForTest(6, len(result.RankedAliases))])
			return
		}
	}
	t.Fatalf("no real-output result ranked %s first; top aliases by row: %v", want, topAliasesForTest(results, 24))
}

func assertNoLocusFirstForTest(t testing.TB, results []AliasRankResult) {
	t.Helper()
	for _, result := range results {
		if len(result.RankedAliases) > 0 && looksLikeLocusForTest(result.RankedAliases[0]) {
			t.Fatalf("locus-like alias ranked first: %v", result.RankedAliases[:minIntForTest(8, len(result.RankedAliases))])
		}
	}
}

func topAliasesForTest(results []AliasRankResult, limit int) []string {
	out := make([]string, 0, minIntForTest(limit, len(results)))
	for _, result := range results {
		if len(result.RankedAliases) > 0 {
			out = append(out, result.RankedAliases[0])
		} else {
			out = append(out, "")
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func minIntForTest(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func looksLikeLocusForTest(value string) bool {
	return LooksLikeDatabaseIdentifier(value) || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(value)), "LOC")
}

func containsAliasForTest(aliases []string, want string) bool {
	for _, alias := range aliases {
		if alias == want {
			return true
		}
	}
	return false
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
		SourceURL:          "https://example.test/GENE_INFO/",
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
