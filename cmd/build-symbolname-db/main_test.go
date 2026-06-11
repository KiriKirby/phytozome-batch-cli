package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
)

func TestDirectoryMetadata(t *testing.T) {
	first := time.Date(2026, 6, 10, 5, 28, 0, 0, time.UTC)
	second := time.Date(2026, 6, 10, 5, 29, 0, 0, time.UTC)
	got := directoryMetadata("https://example.test/GENE_INFO/", []labelname.GeneInfoSourceFile{
		{Name: "All_Fungi.gene_info.gz", LastModified: first, ContentLength: 100},
		{Name: "Organelles.gene_info.gz", LastModified: second, ContentLength: 25},
	})
	if got.URL != "https://example.test/GENE_INFO/" {
		t.Fatalf("URL=%q", got.URL)
	}
	if got.ContentLength != 125 {
		t.Fatalf("ContentLength=%d, want 125", got.ContentLength)
	}
	if got.LastModifiedRaw != second.Format(http.TimeFormat) {
		t.Fatalf("LastModifiedRaw=%q, want %q", got.LastModifiedRaw, second.Format(http.TimeFormat))
	}
}

func TestSafeSourceFilename(t *testing.T) {
	got := safeSourceFilename(labelname.GeneInfoSourceFile{URL: "https://example.test/a/b/All_Plants.gene_info.gz"})
	if got != "All_Plants.gene_info.gz" {
		t.Fatalf("safeSourceFilename fallback=%q", got)
	}
	got = safeSourceFilename(labelname.GeneInfoSourceFile{Name: `bad\name/gene_info.gz`})
	if got != "bad_name_gene_info.gz" {
		t.Fatalf("safeSourceFilename sanitized=%q", got)
	}
}
