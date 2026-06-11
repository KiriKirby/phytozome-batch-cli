package labelname

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
