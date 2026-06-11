package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListPublishFilesSortedAndSkipsGit(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"symbolname/symbolname.pgd.zst.part002",
		"symbolname/manifest.json",
		"README.md",
		".git/config",
	} {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(path), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	got, err := listPublishFiles(dir)
	if err != nil {
		t.Fatalf("listPublishFiles() error = %v", err)
	}
	want := []string{"README.md", "symbolname/manifest.json", "symbolname/symbolname.pgd.zst.part002"}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d: %#v", len(got), len(want), got)
	}
	for i, wantPath := range want {
		if got[i].Path != wantPath {
			t.Fatalf("path[%d]=%q, want %q", i, got[i].Path, wantPath)
		}
	}
}
