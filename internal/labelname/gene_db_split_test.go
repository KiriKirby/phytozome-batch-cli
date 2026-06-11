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
