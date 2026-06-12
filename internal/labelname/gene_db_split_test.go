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

func minIntForTest(a int, b int) int {
	if a < b {
		return a
	}
	return b
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
