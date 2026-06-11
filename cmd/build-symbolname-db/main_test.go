package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func TestSplitArchiveWritesPartManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "symbolname.pgd.zst")
	if err := os.WriteFile(path, []byte("abcdefghijkl"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	manifest := labelname.PrebuiltGeneInfoManifest{ContentLength: 12}
	if err := splitArchive(path, 5, &manifest, "https://example.test/symbolname/symbolname.pgd.zst"); err != nil {
		t.Fatalf("splitArchive() error = %v", err)
	}
	if manifest.URL != "" {
		t.Fatalf("URL=%q, want empty for split archive", manifest.URL)
	}
	if manifest.ContentLength != 0 {
		t.Fatalf("ContentLength=%d, want 0 when manifest uses part sizes", manifest.ContentLength)
	}
	if len(manifest.Parts) != 3 {
		t.Fatalf("Parts=%d, want 3", len(manifest.Parts))
	}
	if manifest.Parts[0].URL != "https://example.test/symbolname/symbolname.pgd.zst.part001" {
		t.Fatalf("first part URL=%q", manifest.Parts[0].URL)
	}
	if manifest.Parts[2].ContentLength != 2 {
		t.Fatalf("last part size=%d, want 2", manifest.Parts[2].ContentLength)
	}
}

func TestRunSimulatedArchiveBuildSplitsLargeArchive(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "symbolname.pgd.zst")
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := runSimulatedArchiveBuild(out, manifestPath, "https://example.test/symbolname/symbolname.pgd.zst", 4*1024, 8*1024+7); err != nil {
		t.Fatalf("runSimulatedArchiveBuild() error = %v", err)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.URL != "" {
		t.Fatalf("manifest URL=%q, want split manifest", manifest.URL)
	}
	if len(manifest.Parts) != 3 {
		t.Fatalf("parts=%d, want 3", len(manifest.Parts))
	}
	for idx, part := range manifest.Parts {
		if part.ContentLength > 4*1024 {
			t.Fatalf("part %d size=%d exceeds limit", idx, part.ContentLength)
		}
		if !strings.HasSuffix(part.URL, ".part"+fmt.Sprintf("%03d", idx+1)) {
			t.Fatalf("part %d URL=%q", idx+1, part.URL)
		}
		partPath := out + fmt.Sprintf(".part%03d", idx+1)
		if _, err := os.Stat(partPath); err != nil {
			t.Fatalf("stat part %d: %v", idx+1, err)
		}
	}
	if manifest.Parts[2].ContentLength != 7 {
		t.Fatalf("last part size=%d, want 7", manifest.Parts[2].ContentLength)
	}
}

func TestPlanArchivePartsForFullScaleArchive(t *testing.T) {
	const partSize = 4 * 1024 * 1024
	const archiveSize = int64(3_006_477_107)
	parts := planArchiveParts(archiveSize, partSize, "https://example.test/symbolname/symbolname.pgd.zst")
	if len(parts) != 717 {
		t.Fatalf("parts=%d, want 717", len(parts))
	}
	var total int64
	for idx, part := range parts {
		if part.ContentLength <= 0 || part.ContentLength > partSize {
			t.Fatalf("part %d size=%d outside limit", idx+1, part.ContentLength)
		}
		total += part.ContentLength
	}
	if total != archiveSize {
		t.Fatalf("planned total=%d, want %d", total, archiveSize)
	}
	if !strings.HasSuffix(parts[len(parts)-1].URL, ".part717") {
		t.Fatalf("last URL=%q", parts[len(parts)-1].URL)
	}
}
