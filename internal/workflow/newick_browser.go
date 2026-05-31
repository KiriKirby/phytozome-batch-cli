package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/phylo"
	"github.com/KiriKirby/phytozome-go/internal/prompt"
	"github.com/KiriKirby/phytozome-go/internal/viewersnapshot"
)

const maxNewickBrowserBytes = 8 << 20
const maxViewerSnapshotBytes = 64 << 20

func (w *BlastWizard) runNewickBrowserTool(ctx context.Context) error {
	viewerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	server := phylo.NewViewerServer("127.0.0.1:0")
	if err := server.Start(viewerCtx); err != nil {
		return err
	}

	sessionSeq := 0
	for {
		target, err := w.prompt.NewickBrowserTarget(prompt.ErrBackToDatabaseSelection)
		if err != nil {
			return err
		}
		sessionSeq++
		sessionID := fmt.Sprintf("nwk-browser-%d-%03d", time.Now().UnixNano(), sessionSeq)
		payload, viewerState, err := loadTreeBrowserSession(ctx, w.httpClient, sessionID, target, time.Now())
		if err != nil {
			if infoErr := w.showInfo("Tree viewer browser", err.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
				return infoErr
			}
			continue
		}
		if err := putViewerPayload(ctx, server, sessionID, payload); err != nil {
			if infoErr := w.showInfo("Tree viewer browser", err.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
				return infoErr
			}
			continue
		}
		if len(bytes.TrimSpace(viewerState)) > 0 {
			if err := putViewerState(ctx, server, sessionID, viewerState); err != nil {
				if infoErr := w.showInfo("Tree viewer browser", err.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
					return infoErr
				}
				continue
			}
		}
		if err := openBrowserURL(ctx, server.URL()+"/sessions/"+url.PathEscape(sessionID)); err != nil {
			if infoErr := w.showInfo("Tree viewer browser", err.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
				return infoErr
			}
			continue
		}
	}
}

func loadTreeBrowserSession(ctx context.Context, client *http.Client, sessionID string, target string, now time.Time) (phylo.ViewerPayload, json.RawMessage, error) {
	target = strings.TrimSpace(target)
	if isViewerSnapshotTarget(target) {
		snapshot, err := loadViewerSnapshot(ctx, client, target)
		if err != nil {
			return phylo.ViewerPayload{}, nil, err
		}
		payload := snapshot.Payload
		payload.SessionID = strings.TrimSpace(sessionID)
		payload.Title = newickBrowserViewerTitle(target)
		if payload.UpdatedAt.IsZero() {
			payload.UpdatedAt = now
		}
		return payload, snapshot.ViewerState, nil
	}
	payload, err := loadNewickBrowserPayload(ctx, client, sessionID, target, now)
	return payload, nil, err
}

func loadNewickBrowserPayload(ctx context.Context, client *http.Client, sessionID string, target string, now time.Time) (phylo.ViewerPayload, error) {
	newick, err := loadNewickBrowserText(ctx, client, target)
	if err != nil {
		return phylo.ViewerPayload{}, err
	}
	return phylo.ViewerPayload{
		SchemaVersion: 1,
		SessionID:     strings.TrimSpace(sessionID),
		Title:         newickBrowserViewerTitle(target),
		UpdatedAt:     now,
		Newick:        newick,
		Metadata: phylo.Metadata{
			SchemaVersion:     1,
			GeneratedAt:       now,
			DisplayNameSource: phylo.DefaultDisplayNameSource,
		},
	}, nil
}

func newickBrowserViewerTitle(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "Tree viewer"
	}
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
		if base := strings.TrimSpace(path.Base(parsed.Path)); base != "" && base != "." && base != "/" {
			return base
		}
		if parsed.Host != "" {
			return parsed.Host
		}
	}
	if base := strings.TrimSpace(filepath.Base(target)); base != "" && base != "." && base != string(filepath.Separator) {
		return base
	}
	return "Tree viewer"
}

func loadViewerSnapshot(ctx context.Context, client *http.Client, rawTarget string) (viewersnapshot.Snapshot, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return viewersnapshot.Snapshot{}, fmt.Errorf("PGV path or URL cannot be empty")
	}
	if strings.ContainsAny(target, "\r\n") {
		return viewersnapshot.Snapshot{}, fmt.Errorf("PGV browser accepts exactly one line")
	}
	if strings.Contains(target, "\t") {
		return viewersnapshot.Snapshot{}, fmt.Errorf("PGV browser input cannot contain tabs")
	}
	lower := strings.ToLower(target)
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		data, err = fetchRemoteViewerSnapshot(ctx, client, target)
	case strings.HasPrefix(lower, "file://"):
		path, pathErr := fileURLToPath(target)
		if pathErr != nil {
			return viewersnapshot.Snapshot{}, pathErr
		}
		data, err = readLocalViewerSnapshotBytes(path)
	case strings.Contains(lower, "://"):
		return viewersnapshot.Snapshot{}, fmt.Errorf("unsupported PGV URL scheme in %q", target)
	default:
		data, err = readLocalViewerSnapshotBytes(target)
	}
	if err != nil {
		return viewersnapshot.Snapshot{}, err
	}
	return viewersnapshot.Decode(data)
}

func loadNewickBrowserText(ctx context.Context, client *http.Client, rawTarget string) (string, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", fmt.Errorf("NWK path or URL cannot be empty")
	}
	if strings.ContainsAny(target, "\r\n") {
		return "", fmt.Errorf("NWK browser accepts exactly one line")
	}
	if strings.Contains(target, "\t") {
		return "", fmt.Errorf("NWK browser input cannot contain tabs")
	}

	lower := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		return fetchRemoteNewick(ctx, client, target)
	case strings.HasPrefix(lower, "file://"):
		path, err := fileURLToPath(target)
		if err != nil {
			return "", err
		}
		return readLocalNewick(path)
	case strings.Contains(lower, "://"):
		return "", fmt.Errorf("unsupported NWK URL scheme in %q", target)
	default:
		return readLocalNewick(target)
	}
}

func fetchRemoteViewerSnapshot(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse PGV URL: %w", err)
	}
	if !isViewerSnapshotPath(parsed.Path) {
		return nil, fmt.Errorf("remote URL must point to a .pgv file")
	}
	if client == nil {
		client = defaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download PGV failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxViewerSnapshotBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxViewerSnapshotBytes {
		return nil, fmt.Errorf("PGV file is larger than %d bytes", maxViewerSnapshotBytes)
	}
	return data, nil
}

func readLocalViewerSnapshotBytes(rawPath string) ([]byte, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return nil, fmt.Errorf("PGV file path cannot be empty")
	}
	if !isViewerSnapshotPath(path) {
		return nil, fmt.Errorf("local path must point to a .pgv file")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if len(data) > maxViewerSnapshotBytes {
		return nil, fmt.Errorf("PGV file is larger than %d bytes", maxViewerSnapshotBytes)
	}
	return data, nil
}

func fetchRemoteNewick(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse NWK URL: %w", err)
	}
	if !isNewickPath(parsed.Path) {
		return "", fmt.Errorf("remote URL must point to a .nwk file")
	}
	if client == nil {
		client = defaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download NWK failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxNewickBrowserBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxNewickBrowserBytes {
		return "", fmt.Errorf("NWK file is larger than %d bytes", maxNewickBrowserBytes)
	}
	return validateNewickBrowserText(string(data))
}

func readLocalNewick(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("NWK file path cannot be empty")
	}
	if !isNewickPath(path) {
		return "", fmt.Errorf("local path must point to a .nwk file")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if len(data) > maxNewickBrowserBytes {
		return "", fmt.Errorf("NWK file is larger than %d bytes", maxNewickBrowserBytes)
	}
	return validateNewickBrowserText(string(data))
}

func validateNewickBrowserText(text string) (string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("NWK file is empty")
	}
	if !looksLikeNewickText(text) {
		return "", fmt.Errorf("text does not look like a Newick tree")
	}
	return text, nil
}

func looksLikeNewickText(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "(") && strings.Contains(text, ")") && strings.HasSuffix(text, ";")
}

func isNewickPath(value string) bool {
	return strings.EqualFold(path.Ext(strings.TrimSpace(value)), ".nwk") || strings.EqualFold(filepath.Ext(strings.TrimSpace(value)), ".nwk")
}

func isViewerSnapshotPath(value string) bool {
	return strings.EqualFold(path.Ext(strings.TrimSpace(value)), ".pgv") || strings.EqualFold(filepath.Ext(strings.TrimSpace(value)), ".pgv")
}

func isViewerSnapshotTarget(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "file://") {
		if parsed, err := url.Parse(value); err == nil {
			return isViewerSnapshotPath(parsed.Path)
		}
	}
	return isViewerSnapshotPath(value)
}

func fileURLToPath(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse file URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "file") {
		return "", fmt.Errorf("unsupported file URL scheme")
	}
	filePath := parsed.Path
	if runtime.GOOS == "windows" && strings.HasPrefix(filePath, "/") && len(filePath) >= 3 && filePath[2] == ':' {
		filePath = filePath[1:]
	}
	if parsed.Host != "" && runtime.GOOS == "windows" {
		filePath = `\\` + parsed.Host + filepath.FromSlash(filePath)
	}
	filePath = filepath.Clean(filepath.FromSlash(filePath))
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file URL does not contain a usable path")
	}
	return filePath, nil
}

func putViewerPayload(ctx context.Context, server *phylo.ViewerServer, sessionID string, payload phylo.ViewerPayload) error {
	if server == nil {
		return fmt.Errorf("tree viewer server is unavailable")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, server.URL()+"/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/payload", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := canvasTreeViewerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("tree viewer update failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func putViewerState(ctx context.Context, server *phylo.ViewerServer, sessionID string, viewerState json.RawMessage) error {
	if server == nil {
		return fmt.Errorf("tree viewer server is unavailable")
	}
	viewerState = bytes.TrimSpace(viewerState)
	if len(viewerState) == 0 {
		viewerState = json.RawMessage(`{}`)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, server.URL()+"/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/state", bytes.NewReader(viewerState))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := canvasTreeViewerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("tree viewer state update failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
