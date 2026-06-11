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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/netconfig"
)

const defaultMaxGitHubBlobBytes int64 = 4 * 1024 * 1024

type githubClient struct {
	repo   string
	token  string
	client *http.Client
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
	var orphan bool
	var dryRun bool
	flag.StringVar(&repo, "repo", strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")), "owner/repo")
	flag.StringVar(&branch, "branch", "", "target branch")
	flag.StringVar(&sourceDir, "source", "", "directory containing README.md and symbolname/")
	flag.StringVar(&token, "token", strings.TrimSpace(os.Getenv("GITHUB_TOKEN")), "GitHub token")
	flag.StringVar(&message, "message", "Update prebuilt symbolname.pgd", "commit message")
	flag.Int64Var(&maxBlobBytes, "max-blob-bytes", defaultMaxGitHubBlobBytes, "maximum file size allowed for GitHub API blob upload")
	flag.BoolVar(&orphan, "orphan", true, "publish an orphan commit so old large database snapshots do not accumulate in branch history")
	flag.BoolVar(&dryRun, "dry-run", false, "validate and list the publish set without contacting GitHub")
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
	files, err := listPublishFiles(sourceDir)
	if err != nil {
		return err
	}
	total, err := validatePublishFiles(files, maxBlobBytes)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Publishing %d files to %s:%s (%s)\n", len(files), repo, branch, formatBytes(total))
	if dryRun {
		fmt.Fprintf(os.Stdout, "Dry run passed; largest file is at most %s\n", formatBytes(maxBlobBytes))
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	client := githubClient{repo: repo, token: token, client: netconfig.DefaultHTTPClient()}
	entries := make([]treeEntry, 0, len(files))
	var done int64
	for _, file := range files {
		sha, err := client.createBlob(ctx, filepath.Join(sourceDir, filepath.FromSlash(file.Path)))
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
	fmt.Fprintf(os.Stdout, "Published %s to %s\n", commitSHA, branch)
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
		req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com/repos/"+c.repo+path, bytes.NewReader(payload))
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
		delay := time.Duration(attempt*attempt) * 5 * time.Second
		fmt.Fprintf(os.Stdout, "GitHub request retry %d/%d after %s: %v\n", attempt+1, attempts, delay, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
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
