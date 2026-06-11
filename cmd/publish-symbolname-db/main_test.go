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

func TestValidatePublishFilesRejectsOversizedBlob(t *testing.T) {
	_, err := validatePublishFiles([]publishFile{
		{Path: "symbolname/symbolname.pgd.zst.part001", Size: defaultMaxGitHubBlobBytes + 1},
	}, defaultMaxGitHubBlobBytes)
	if err == nil {
		t.Fatal("validatePublishFiles() error = nil, want oversized blob error")
	}
}

func TestValidatePublishFilesTotalsAcceptedFiles(t *testing.T) {
	total, err := validatePublishFiles([]publishFile{
		{Path: "README.md", Size: 10},
		{Path: "symbolname/manifest.json", Size: 20},
		{Path: "symbolname/symbolname.pgd.zst.part001", Size: defaultMaxGitHubBlobBytes},
	}, defaultMaxGitHubBlobBytes)
	if err != nil {
		t.Fatalf("validatePublishFiles() error = %v", err)
	}
	want := defaultMaxGitHubBlobBytes + 30
	if total != want {
		t.Fatalf("total=%d, want %d", total, want)
	}
}
