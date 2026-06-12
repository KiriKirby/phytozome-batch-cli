package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
	"github.com/KiriKirby/phytozome-go/internal/netconfig"
)

const defaultMaxGitHubBlobBytes int64 = 4 * 1024 * 1024
const defaultMaxReleaseAssetBytes int64 = 1900 * 1024 * 1024

type githubClient struct {
	repo       string
	token      string
	client     *http.Client
	apiURL     string
	retryDelay func(int) time.Duration
}

type refResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type blobResponse struct {
	SHA string `json:"sha"`
}

type treeResponse struct {
	SHA string `json:"sha"`
}

type commitResponse struct {
	SHA string `json:"sha"`
}

type releaseResponse struct {
	ID        int64          `json:"id"`
	UploadURL string         `json:"upload_url"`
	Assets    []releaseAsset `json:"assets,omitempty"`
}

type releaseAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	var repo string
	var branch string
	var sourceDir string
	var token string
	var message string
	var maxBlobBytes int64
	var maxReleaseAssetBytes int64
	var orphan bool
	var dryRun bool
	var releaseTag string
	var releaseName string
	var releaseAssetPrefix string
	var cleanupOldAssetsPrefix string
	var prerelease bool
	flag.StringVar(&repo, "repo", strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")), "owner/repo")
	flag.StringVar(&branch, "branch", "", "target branch")
	flag.StringVar(&sourceDir, "source", "", "directory containing README.md and symbolname/")
	flag.StringVar(&token, "token", strings.TrimSpace(os.Getenv("GITHUB_TOKEN")), "GitHub token")
	flag.StringVar(&message, "message", "Update prebuilt symbolname.pgd", "commit message")
	flag.Int64Var(&maxBlobBytes, "max-blob-bytes", defaultMaxGitHubBlobBytes, "maximum file size allowed for GitHub API blob upload")
	flag.Int64Var(&maxReleaseAssetBytes, "max-release-asset-bytes", defaultMaxReleaseAssetBytes, "maximum file size allowed for GitHub release asset upload")
	flag.BoolVar(&orphan, "orphan", true, "publish an orphan commit so old large database snapshots do not accumulate in branch history")
	flag.BoolVar(&dryRun, "dry-run", false, "validate and list the publish set without contacting GitHub")
	flag.StringVar(&releaseTag, "release-tag", "", "when set, upload database archive files to this GitHub Release tag and publish only manifest/README to the branch")
	flag.StringVar(&releaseName, "release-name", "", "GitHub Release name used when creating -release-tag")
	flag.StringVar(&releaseAssetPrefix, "release-asset-prefix", "", "unique prefix for uploaded database asset names")
	flag.StringVar(&cleanupOldAssetsPrefix, "cleanup-old-assets-prefix", "", "delete old release assets with this prefix after the manifest branch is published")
	flag.BoolVar(&prerelease, "prerelease", false, "mark newly created release as prerelease")
	flag.Parse()
	if strings.TrimSpace(repo) == "" {
		return fmt.Errorf("-repo is required")
	}
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("-branch is required")
	}
	if strings.TrimSpace(sourceDir) == "" {
		return fmt.Errorf("-source is required")
	}
	if strings.TrimSpace(token) == "" && !dryRun {
		return fmt.Errorf("-token or GITHUB_TOKEN is required")
	}
	if releaseName == "" && releaseTag != "" {
		releaseName = releaseTag
	}
	if releaseAssetPrefix == "" && releaseTag != "" {
		releaseAssetPrefix = branch + "-" + time.Now().UTC().Format("20060102T150405Z")
	}
	files, err := listPublishFiles(sourceDir)
	if err != nil {
		return err
	}
	branchSource := sourceDir
	var assetFiles []publishFile
	var currentAssetNames []string
	if strings.TrimSpace(releaseTag) != "" {
		branchSource, assetFiles, currentAssetNames, err = prepareReleaseAssetPublish(sourceDir, files, releaseAssetPrefix, maxReleaseAssetBytes, dryRun)
		if err != nil {
			return err
		}
		if branchSource != sourceDir {
			defer os.RemoveAll(branchSource)
		}
		files, err = listPublishFiles(branchSource)
		if err != nil {
			return err
		}
	}
	total, err := validatePublishFiles(files, maxBlobBytes)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Publishing %d files to %s:%s (%s)\n", len(files), repo, branch, formatBytes(total))
	if dryRun {
		if len(assetFiles) > 0 {
			var assetTotal int64
			for _, file := range assetFiles {
				assetTotal += file.Size
			}
			fmt.Fprintf(os.Stdout, "Dry run release assets: %d files to %s (%s)\n", len(assetFiles), releaseTag, formatBytes(assetTotal))
		}
		fmt.Fprintf(os.Stdout, "Dry run passed; largest file is at most %s\n", formatBytes(maxBlobBytes))
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	client := githubClient{repo: repo, token: token, client: netconfig.DefaultHTTPClient(), apiURL: "https://api.github.com"}
	if strings.TrimSpace(releaseTag) != "" {
		if err := publishReleaseAssets(ctx, client, releaseTag, releaseName, sourceDir, branchSource, assetFiles, currentAssetNames, prerelease); err != nil {
			return err
		}
	}
	entries := make([]treeEntry, 0, len(files))
	var done int64
	for _, file := range files {
		sha, err := client.createBlob(ctx, filepath.Join(branchSource, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		done += file.Size
		fmt.Fprintf(os.Stdout, "Uploaded blob %s (%s/%s)\n", file.Path, formatBytes(done), formatBytes(total))
		entries = append(entries, treeEntry{Path: file.Path, Mode: "100644", Type: "blob", SHA: sha})
	}
	existingSHA, _ := client.getRef(ctx, branch)
	parentSHA := ""
	if !orphan {
		parentSHA = existingSHA
	}
	treeSHA, err := client.createTree(ctx, entries)
	if err != nil {
		return err
	}
	commitSHA, err := client.createCommit(ctx, message, treeSHA, parentSHA)
	if err != nil {
		return err
	}
	if err := client.updateRef(ctx, branch, commitSHA, existingSHA == ""); err != nil {
		return err
	}
	if strings.TrimSpace(releaseTag) != "" && strings.TrimSpace(cleanupOldAssetsPrefix) != "" {
		if err := cleanupReleaseAssets(ctx, client, releaseTag, cleanupOldAssetsPrefix, currentAssetNames); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stdout, "Published %s to %s\n", commitSHA, branch)
	return nil
}

func prepareReleaseAssetPublish(sourceDir string, files []publishFile, assetPrefix string, maxAssetBytes int64, dryRun bool) (string, []publishFile, []string, error) {
	manifestPath := filepath.Join(sourceDir, "symbolname", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("read symbolname manifest for release asset publish: %w", err)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", nil, nil, fmt.Errorf("decode symbolname manifest for release asset publish: %w", err)
	}
	assetFiles, err := databaseAssetFiles(files, manifest)
	if err != nil {
		return "", nil, nil, err
	}
	if len(assetFiles) == 0 {
		return sourceDir, nil, nil, nil
	}
	if _, err := validatePublishFiles(assetFiles, maxAssetBytes); err != nil {
		return "", nil, nil, fmt.Errorf("release asset validation failed: %w", err)
	}
	rewritten := manifest
	currentAssetNames := make([]string, 0, len(assetFiles))
	if len(rewritten.Parts) > 0 {
		for idx := range rewritten.Parts {
			name := releaseAssetName(assetPrefix, filepath.Base(assetFiles[idx].Path))
			currentAssetNames = append(currentAssetNames, name)
			rewritten.Parts[idx].URL = browserDownloadURLPlaceholder(name)
		}
	} else {
		name := releaseAssetName(assetPrefix, filepath.Base(assetFiles[0].Path))
		currentAssetNames = append(currentAssetNames, name)
		rewritten.URL = browserDownloadURLPlaceholder(name)
	}
	branchDir, err := os.MkdirTemp("", "phgo-symbolname-branch-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("create manifest-only publish directory: %w", err)
	}
	if err := copyIfExists(filepath.Join(sourceDir, "README.md"), filepath.Join(branchDir, "README.md")); err != nil {
		_ = os.RemoveAll(branchDir)
		return "", nil, nil, err
	}
	if err := os.MkdirAll(filepath.Join(branchDir, "symbolname"), 0o755); err != nil {
		_ = os.RemoveAll(branchDir)
		return "", nil, nil, fmt.Errorf("create manifest-only symbolname directory: %w", err)
	}
	out, err := json.MarshalIndent(rewritten, "", "  ")
	if err != nil {
		_ = os.RemoveAll(branchDir)
		return "", nil, nil, fmt.Errorf("marshal release-asset manifest: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(filepath.Join(branchDir, "symbolname", "manifest.json"), out, 0o644); err != nil {
		_ = os.RemoveAll(branchDir)
		return "", nil, nil, fmt.Errorf("write release-asset manifest: %w", err)
	}
	return branchDir, assetFiles, currentAssetNames, nil
}

func databaseAssetFiles(files []publishFile, manifest labelname.PrebuiltGeneInfoManifest) ([]publishFile, error) {
	byBase := make(map[string]publishFile, len(files))
	for _, file := range files {
		byBase[filepath.Base(file.Path)] = file
	}
	if len(manifest.Parts) > 0 {
		out := make([]publishFile, 0, len(manifest.Parts))
		for _, part := range manifest.Parts {
			base, err := urlBase(part.URL)
			if err != nil {
				return nil, err
			}
			file, ok := byBase[base]
			if !ok {
				return nil, fmt.Errorf("manifest part %s has no matching local file", base)
			}
			out = append(out, file)
		}
		return out, nil
	}
	if strings.TrimSpace(manifest.URL) == "" {
		return nil, nil
	}
	base, err := urlBase(manifest.URL)
	if err != nil {
		return nil, err
	}
	file, ok := byBase[base]
	if !ok {
		return nil, fmt.Errorf("manifest archive %s has no matching local file", base)
	}
	return []publishFile{file}, nil
}

func urlBase(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("manifest asset URL is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Path != "" {
		return filepath.Base(parsed.Path), nil
	}
	return filepath.Base(trimmed), nil
}

func releaseAssetName(prefix string, base string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "-_./ ")
	base = filepath.Base(strings.TrimSpace(base))
	if prefix == "" {
		return base
	}
	return prefix + "-" + base
}

func browserDownloadURLPlaceholder(assetName string) string {
	return "release-asset://" + assetName
}

func copyIfExists(source string, dest string) error {
	input, err := os.Open(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
	}
	output, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return fmt.Errorf("copy %s to %s: %w", source, dest, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dest, err)
	}
	return nil
}

func publishReleaseAssets(ctx context.Context, client githubClient, tag string, name string, assetSourceDir string, manifestSourceDir string, assetFiles []publishFile, currentAssetNames []string, prerelease bool) error {
	if len(assetFiles) == 0 {
		return nil
	}
	release, err := client.getOrCreateRelease(ctx, tag, name, prerelease)
	if err != nil {
		return err
	}
	assetURLs := make(map[string]string, len(assetFiles))
	for idx, file := range assetFiles {
		if idx >= len(currentAssetNames) {
			return fmt.Errorf("release asset file/name mismatch")
		}
		assetName := currentAssetNames[idx]
		if existing := release.assetByName(assetName); existing != nil {
			if err := client.deleteReleaseAsset(ctx, existing.ID); err != nil {
				return fmt.Errorf("replace existing release asset %s: %w", assetName, err)
			}
		}
		asset, err := client.uploadReleaseAsset(ctx, release.UploadURL, assetName, filepath.Join(assetSourceDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		assetURLs[browserDownloadURLPlaceholder(assetName)] = asset.BrowserDownloadURL
		fmt.Fprintf(os.Stdout, "Uploaded release asset %s (%s)\n", assetName, formatBytes(file.Size))
	}
	manifestPath := filepath.Join(manifestSourceDir, "symbolname", "manifest.json")
	if err := rewriteManifestAssetURLs(manifestPath, assetURLs); err != nil {
		return err
	}
	return nil
}

func cleanupReleaseAssets(ctx context.Context, client githubClient, tag string, cleanupPrefix string, keepNames []string) error {
	keep := make(map[string]bool, len(keepNames))
	for _, name := range keepNames {
		keep[name] = true
	}
	release, err := client.getReleaseByTag(ctx, tag)
	if err != nil {
		return err
	}
	for _, asset := range release.Assets {
		if keep[asset.Name] || !strings.HasPrefix(asset.Name, cleanupPrefix) {
			continue
		}
		if err := client.deleteReleaseAsset(ctx, asset.ID); err != nil {
			return fmt.Errorf("delete old release asset %s: %w", asset.Name, err)
		}
		fmt.Fprintf(os.Stdout, "Deleted old release asset %s\n", asset.Name)
	}
	return nil
}

func rewriteManifestAssetURLs(manifestPath string, replacements map[string]string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest for release URL rewrite: %w", err)
	}
	var manifest labelname.PrebuiltGeneInfoManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode manifest for release URL rewrite: %w", err)
	}
	if manifest.URL != "" {
		next, ok := replacements[manifest.URL]
		if !ok {
			return fmt.Errorf("missing release download URL for %s", manifest.URL)
		}
		manifest.URL = next
	}
	for idx := range manifest.Parts {
		next, ok := replacements[manifest.Parts[idx].URL]
		if !ok {
			return fmt.Errorf("missing release download URL for %s", manifest.Parts[idx].URL)
		}
		manifest.Parts[idx].URL = next
	}
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest after release URL rewrite: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
		return fmt.Errorf("write manifest after release URL rewrite: %w", err)
	}
	return nil
}

func (r releaseResponse) assetByName(name string) *releaseAsset {
	for idx := range r.Assets {
		if r.Assets[idx].Name == name {
			return &r.Assets[idx]
		}
	}
	return nil
}

func (c githubClient) getOrCreateRelease(ctx context.Context, tag string, name string, prerelease bool) (releaseResponse, error) {
	release, err := c.getReleaseByTag(ctx, tag)
	if err == nil {
		return release, nil
	}
	var status githubStatusError
	if !errorAs(err, &status) || status.StatusCode != http.StatusNotFound {
		return releaseResponse{}, fmt.Errorf("get release %s: %w", tag, err)
	}
	body := map[string]any{
		"tag_name":   tag,
		"name":       name,
		"prerelease": prerelease,
	}
	var out releaseResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/releases", body, &out, true); err != nil {
		return releaseResponse{}, fmt.Errorf("create release %s: %w", tag, err)
	}
	return out, nil
}

func (c githubClient) getReleaseByTag(ctx context.Context, tag string) (releaseResponse, error) {
	var out releaseResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/releases/tags/"+url.PathEscape(tag), nil, &out, false); err != nil {
		return releaseResponse{}, err
	}
	return out, nil
}

func (c githubClient) uploadReleaseAsset(ctx context.Context, uploadURL string, name string, path string) (releaseAsset, error) {
	trimmed := strings.TrimSpace(uploadURL)
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if trimmed == "" {
		return releaseAsset{}, fmt.Errorf("release upload URL is empty")
	}
	stat, err := os.Stat(path)
	if err != nil {
		return releaseAsset{}, fmt.Errorf("stat release asset %s: %w", path, err)
	}
	endpoint, err := url.Parse(trimmed)
	if err != nil {
		return releaseAsset{}, fmt.Errorf("parse release upload URL: %w", err)
	}
	q := endpoint.Query()
	q.Set("name", name)
	endpoint.RawQuery = q.Encode()
	attempts := 6
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		file, err := os.Open(path)
		if err != nil {
			return releaseAsset{}, fmt.Errorf("open release asset %s: %w", path, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), file)
		if err != nil {
			file.Close()
			return releaseAsset{}, err
		}
		req.ContentLength = stat.Size()
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("User-Agent", "phytozome-go-symbolname-publisher")
		resp, err := c.client.Do(req)
		if err == nil {
			var out releaseAsset
			err = decodeGitHubResponse(resp, &out)
			if err == nil {
				file.Close()
				return out, nil
			}
		}
		file.Close()
		lastErr = err
		if !isRetryableGitHubError(err) || attempt == attempts {
			break
		}
		delay := c.delay(attempt)
		fmt.Fprintf(os.Stdout, "GitHub release asset retry %d/%d after %s: %v\n", attempt+1, attempts, delay, err)
		select {
		case <-ctx.Done():
			return releaseAsset{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	return releaseAsset{}, fmt.Errorf("upload release asset %s: %w", name, lastErr)
}

func (c githubClient) deleteReleaseAsset(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	if err := c.requestJSON(ctx, http.MethodDelete, fmt.Sprintf("/releases/assets/%d", id), nil, nil, true); err != nil {
		return err
	}
	return nil
}

func validatePublishFiles(files []publishFile, maxBlobBytes int64) (int64, error) {
	var total int64
	for _, file := range files {
		if maxBlobBytes > 0 && file.Size > maxBlobBytes {
			return 0, fmt.Errorf("%s is %s, larger than -max-blob-bytes %s; reduce build -part-size before publishing", file.Path, formatBytes(file.Size), formatBytes(maxBlobBytes))
		}
		total += file.Size
	}
	return total, nil
}

type publishFile struct {
	Path string
	Size int64
}

func listPublishFiles(root string) ([]publishFile, error) {
	var out []publishFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, publishFile{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list publish files: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (c githubClient) getRef(ctx context.Context, branch string) (string, error) {
	var out refResponse
	err := c.requestJSON(ctx, http.MethodGet, "/git/ref/heads/"+branch, nil, &out, false)
	if err != nil {
		return "", err
	}
	return out.Object.SHA, nil
}

func (c githubClient) createBlob(ctx context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	body := map[string]string{
		"content":  base64.StdEncoding.EncodeToString(data),
		"encoding": "base64",
	}
	var out blobResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/git/blobs", body, &out, true); err != nil {
		return "", fmt.Errorf("upload blob %s: %w", path, err)
	}
	return out.SHA, nil
}

func (c githubClient) createTree(ctx context.Context, entries []treeEntry) (string, error) {
	body := map[string]any{"tree": entries}
	var out treeResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/git/trees", body, &out, true); err != nil {
		return "", fmt.Errorf("create tree: %w", err)
	}
	return out.SHA, nil
}

func (c githubClient) createCommit(ctx context.Context, message string, treeSHA string, parentSHA string) (string, error) {
	body := map[string]any{"message": message, "tree": treeSHA}
	if parentSHA != "" {
		body["parents"] = []string{parentSHA}
	}
	var out commitResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/git/commits", body, &out, true); err != nil {
		return "", fmt.Errorf("create commit: %w", err)
	}
	return out.SHA, nil
}

func (c githubClient) updateRef(ctx context.Context, branch string, commitSHA string, create bool) error {
	method := http.MethodPatch
	path := "/git/refs/heads/" + branch
	body := map[string]any{"sha": commitSHA, "force": true}
	if create {
		method = http.MethodPost
		path = "/git/refs"
		body = map[string]any{"ref": "refs/heads/" + branch, "sha": commitSHA}
	}
	if err := c.requestJSON(ctx, method, path, body, nil, true); err != nil {
		return fmt.Errorf("update ref %s: %w", branch, err)
	}
	return nil
}

func (c githubClient) requestJSON(ctx context.Context, method string, path string, body any, out any, retry bool) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	attempts := 1
	if retry {
		attempts = 6
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		base := strings.TrimRight(c.apiURL, "/")
		if base == "" {
			base = "https://api.github.com"
		}
		req, err := http.NewRequestWithContext(ctx, method, base+"/repos/"+c.repo+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "phytozome-go-symbolname-publisher")
		resp, err := c.client.Do(req)
		if err == nil {
			err = decodeGitHubResponse(resp, out)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry || !isRetryableGitHubError(err) || attempt == attempts {
			break
		}
		delay := c.delay(attempt)
		fmt.Fprintf(os.Stdout, "GitHub request retry %d/%d after %s: %v\n", attempt+1, attempts, delay, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func (c githubClient) delay(attempt int) time.Duration {
	if c.retryDelay != nil {
		return c.retryDelay(attempt)
	}
	return time.Duration(attempt*attempt) * 5 * time.Second
}

type githubStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e githubStatusError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return e.Status
	}
	return e.Status + ": " + e.Body
}

func decodeGitHubResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(data))}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func isRetryableGitHubError(err error) bool {
	var status githubStatusError
	if ok := errorAs(err, &status); ok {
		return status.StatusCode == http.StatusTooManyRequests || status.StatusCode >= 500
	}
	return true
}

func errorAs(err error, target *githubStatusError) bool {
	for err != nil {
		if status, ok := err.(githubStatusError); ok {
			*target = status
			return true
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
			continue
		}
		break
	}
	return false
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "unknown size"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	unit := units[0]
	for i := 1; i < len(units) && value >= 1024; i++ {
		value /= 1024
		unit = units[i]
	}
	if unit == "B" {
		return fmt.Sprintf("%d %s", size, unit)
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}
