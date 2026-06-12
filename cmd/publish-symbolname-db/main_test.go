package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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

	"github.com/KiriKirby/phytozome-go/internal/labelname"
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

func TestPrepareReleaseAssetPublishWritesManifestOnlyBranch(t *testing.T) {
	dir := writePublishSource(t, []string{"https://raw.example/symbolname.pgd.zst.part001", "https://raw.example/symbolname.pgd.zst.part002"})
	files, err := listPublishFiles(dir)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	branchDir, assets, names, err := prepareReleaseAssetPublish(dir, files, "run-1", 1024, false)
	if err != nil {
		t.Fatalf("prepareReleaseAssetPublish() error = %v", err)
	}
	defer os.RemoveAll(branchDir)
	if branchDir == dir {
		t.Fatal("branchDir reused source dir; want manifest-only temp dir")
	}
	if len(assets) != 2 || len(names) != 2 {
		t.Fatalf("assets=%d names=%d, want 2", len(assets), len(names))
	}
	branchFiles, err := listPublishFiles(branchDir)
	if err != nil {
		t.Fatalf("list branch files: %v", err)
	}
	gotPaths := make([]string, 0, len(branchFiles))
	for _, file := range branchFiles {
		gotPaths = append(gotPaths, file.Path)
	}
	want := strings.Join([]string{"README.md", "symbolname/manifest.json"}, "\n")
	if strings.Join(gotPaths, "\n") != want {
		t.Fatalf("branch files:\n%s\nwant:\n%s", strings.Join(gotPaths, "\n"), want)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	data, err := os.ReadFile(filepath.Join(branchDir, "symbolname", "manifest.json"))
	if err != nil {
		t.Fatalf("read branch manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode branch manifest: %v", err)
	}
	if !strings.HasPrefix(manifest.Parts[0].URL, "release-asset://run-1-") {
		t.Fatalf("part URL was not rewritten to placeholder: %q", manifest.Parts[0].URL)
	}
}

func TestRewriteManifestAssetURLs(t *testing.T) {
	dir := writePublishSource(t, []string{"release-asset://run-1-symbolname.pgd.zst.part001"})
	manifestPath := filepath.Join(dir, "symbolname", "manifest.json")
	err := rewriteManifestAssetURLs(manifestPath, map[string]string{
		"release-asset://run-1-symbolname.pgd.zst.part001": "https://github.com/owner/repo/releases/download/tag/run-1-symbolname.pgd.zst.part001",
	})
	if err != nil {
		t.Fatalf("rewriteManifestAssetURLs() error = %v", err)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if !strings.Contains(manifest.Parts[0].URL, "/releases/download/") {
		t.Fatalf("part URL=%q, want release download URL", manifest.Parts[0].URL)
	}
}

func TestUploadReleaseAssetRetriesServerError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upload" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "asset.bin" {
			t.Fatalf("asset name query=%q", r.URL.RawQuery)
		}
		if attempts.Add(1) == 1 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"asset.bin","browser_download_url":"https://download.test/asset.bin"}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "asset.bin")
	if err := os.WriteFile(path, []byte("asset"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	client := githubClient{token: "token", client: server.Client(), retryDelay: func(int) time.Duration { return 0 }}
	asset, err := client.uploadReleaseAsset(t.Context(), server.URL+"/upload{?name,label}", "asset.bin", path)
	if err != nil {
		t.Fatalf("uploadReleaseAsset() error = %v", err)
	}
	if asset.BrowserDownloadURL != "https://download.test/asset.bin" {
		t.Fatalf("download URL=%q", asset.BrowserDownloadURL)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts=%d, want 2", attempts.Load())
	}
}

func TestPublishReleaseAssetsRewritesManifestAfterUploads(t *testing.T) {
	dir := writePublishSource(t, []string{"https://raw.example/symbolname.pgd.zst.part001", "https://raw.example/symbolname.pgd.zst.part002"})
	files, err := listPublishFiles(dir)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	branchDir, assets, names, err := prepareReleaseAssetPublish(dir, files, "run-2", 1024, false)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer os.RemoveAll(branchDir)
	var uploads int
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/releases/tags/symbolname-db-simulate-assets":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":1,"upload_url":"%s/upload{?name,label}","assets":[]}`, serverURL)
		case r.Method == http.MethodPost && r.URL.Path == "/upload":
			name := r.URL.Query().Get("name")
			body, _ := io.ReadAll(r.Body)
			if len(body) != 2 {
				t.Fatalf("uploaded %s length=%d, want 2", name, len(body))
			}
			uploads++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":%d,"name":%q,"browser_download_url":"%s/download/%s"}`, uploads, name, serverURL, name)
		default:
			t.Fatalf("unexpected GitHub request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	serverURL = server.URL
	client := githubClient{repo: "owner/repo", token: "token", client: server.Client(), apiURL: server.URL, retryDelay: func(int) time.Duration { return 0 }}
	if err := publishReleaseAssets(t.Context(), client, "symbolname-db-simulate-assets", "simulate", dir, branchDir, assets, names, true); err != nil {
		t.Fatalf("publishReleaseAssets() error = %v", err)
	}
	if uploads != 2 {
		t.Fatalf("uploads=%d, want 2", uploads)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	data, err := os.ReadFile(filepath.Join(branchDir, "symbolname", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	for _, part := range manifest.Parts {
		if !strings.Contains(part.URL, "/download/run-2-") {
			t.Fatalf("part URL=%q, want release download URL", part.URL)
		}
	}
}

func TestBranchPublishUploadsOnlyManifestFilesAfterReleasePrepare(t *testing.T) {
	dir := writePublishSource(t, []string{"https://raw.example/symbolname.pgd.zst.part001"})
	files, err := listPublishFiles(dir)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	branchDir, _, _, err := prepareReleaseAssetPublish(dir, files, "run-3", 1024, false)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer os.RemoveAll(branchDir)
	branchFiles, err := listPublishFiles(branchDir)
	if err != nil {
		t.Fatalf("list branch files: %v", err)
	}
	var uploaded [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/git/blobs":
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode blob: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode blob content: %v", err)
			}
			uploaded = append(uploaded, data)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"sha":"blob-%d"}`, len(uploaded))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/git/ref/heads/symbolname-db-simulate":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/git/trees":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"tree-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/git/commits":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"commit-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/git/refs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ref":"refs/heads/symbolname-db-simulate"}`))
		default:
			t.Fatalf("unexpected GitHub request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	client := githubClient{repo: "owner/repo", token: "token", client: server.Client(), apiURL: server.URL, retryDelay: func(int) time.Duration { return 0 }}
	var entries []treeEntry
	for _, file := range branchFiles {
		sha, err := client.createBlob(t.Context(), filepath.Join(branchDir, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatalf("createBlob(%s): %v", file.Path, err)
		}
		entries = append(entries, treeEntry{Path: file.Path, Mode: "100644", Type: "blob", SHA: sha})
	}
	treeSHA, err := client.createTree(t.Context(), entries)
	if err != nil {
		t.Fatalf("createTree: %v", err)
	}
	commitSHA, err := client.createCommit(t.Context(), "test", treeSHA, "")
	if err != nil {
		t.Fatalf("createCommit: %v", err)
	}
	if err := client.updateRef(t.Context(), "symbolname-db-simulate", commitSHA, true); err != nil {
		t.Fatalf("updateRef: %v", err)
	}
	if len(uploaded) != 2 {
		t.Fatalf("uploaded blobs=%d, want README and manifest only", len(uploaded))
	}
	for _, data := range uploaded {
		if bytes.Contains(data, []byte{0, 1}) {
			t.Fatal("database part bytes were uploaded to branch blob API")
		}
	}
}

func writePublishSource(t *testing.T, partURLs []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "symbolname"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	parts := make([]labelname.PrebuiltGeneInfoPart, 0, len(partURLs))
	for idx, rawURL := range partURLs {
		base := filepath.Base(rawURL)
		if strings.HasPrefix(rawURL, "release-asset://") {
			base = strings.TrimPrefix(rawURL, "release-asset://")
			base = strings.TrimPrefix(base, "run-1-")
		}
		if err := os.WriteFile(filepath.Join(dir, "symbolname", base), []byte{byte(idx), byte(idx + 1)}, 0o644); err != nil {
			t.Fatalf("write part %s: %v", base, err)
		}
		parts = append(parts, labelname.PrebuiltGeneInfoPart{URL: rawURL, ContentLength: 2})
	}
	manifest := labelname.PrebuiltGeneInfoManifest{
		SchemaVersion: "2",
		Parts:         parts,
		SHA256:        strings.Repeat("0", 64),
		RecordCount:   1,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "symbolname", "manifest.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}
