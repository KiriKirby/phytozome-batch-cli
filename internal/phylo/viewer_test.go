package phylo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestViewerServerStartsEmptyAndAcceptsPayload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewViewerServer("127.0.0.1:0")
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	base := server.URL()
	if base == "" {
		t.Fatal("server URL is empty")
	}
	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	resp, err = http.Get(base + "/sessions/test")
	if err != nil {
		t.Fatalf("session request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if cache := resp.Header.Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Fatalf("session page Cache-Control = %q, want no-store", cache)
	}
	if !looksLikeViewerAppShell(string(body)) {
		t.Fatalf("initial page should serve the Reactree app shell: %s", body)
	}
	resp, err = http.Get(base + "/phgo-icon.png")
	if err != nil {
		t.Fatalf("icon request failed: %v", err)
	}
	iconBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(iconBody) == 0 {
		t.Fatalf("icon status/body = %d/%d, want embedded icon", resp.StatusCode, len(iconBody))
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "image/png") {
		t.Fatalf("icon Content-Type = %q, want image/png", resp.Header.Get("Content-Type"))
	}
	resp, err = http.Get(base + "/sessions/test/payload")
	if err != nil {
		t.Fatalf("initial payload request failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `"session_id":"test"`) || !strings.Contains(string(body), `"newick":""`) {
		t.Fatalf("initial payload should be empty for the session: %s", body)
	}
	payload := []byte(`{"schema_version":1,"newick":"(PHGOT000001);","updated_at":"` + time.Now().Format(time.RFC3339Nano) + `"}`)
	req, err := http.NewRequest(http.MethodPut, base+"/sessions/test/payload", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("payload request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("payload status = %d", resp.StatusCode)
	}
	resp, err = http.Get(base + "/sessions/test/payload")
	if err != nil {
		t.Fatalf("payload get failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `(PHGOT000001);`) {
		t.Fatalf("payload should contain Newick after update: %s", body)
	}
}

func TestViewerAssetsEmbedReactreeApp(t *testing.T) {
	index, err := viewerAsset("index.html")
	if err != nil {
		t.Fatalf("viewer index asset missing: %v", err)
	}
	if !looksLikeViewerAppShell(string(index)) {
		t.Fatalf("viewer index does not look like the built Reactree app: %s", index)
	}
}

func looksLikeViewerAppShell(html string) bool {
	return strings.Contains(html, "<title>PHgo-Viewer</title>") &&
		strings.Contains(html, `href="/phgo-icon.png"`) &&
		strings.Contains(html, `/assets/`) &&
		strings.Contains(html, `id="root"`)
}

func TestViewerServerSSEUpdatesAfterPayloadAndPreviewChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewViewerServer("127.0.0.1:0")
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	streamCtx, streamCancel := context.WithTimeout(ctx, 5*time.Second)
	defer streamCancel()
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, server.URL()+"/events/test", nil)
	if err != nil {
		t.Fatalf("build event request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("event request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("event status = %d", resp.StatusCode)
	}
	reader := bufio.NewReader(resp.Body)
	initial := readViewerSSEUpdate(t, reader)
	if !strings.Contains(initial, `"session_id":"test"`) || !strings.Contains(initial, `"seq":0`) {
		t.Fatalf("initial event should describe empty session: %q", initial)
	}

	putViewerPayload(t, server.URL()+"/sessions/test/payload", []byte(`{"schema_version":1,"newick":"(PHGOT000001);","updated_at":"`+time.Now().Format(time.RFC3339Nano)+`"}`))
	payloadUpdate := readViewerSSEUpdate(t, reader)
	if !strings.Contains(payloadUpdate, `"session_id":"test"`) || !strings.Contains(payloadUpdate, `"seq":1`) {
		t.Fatalf("payload update event should increment seq: %q", payloadUpdate)
	}

	putViewerPayload(t, server.URL()+"/sessions/test/preview", []byte(`{"layout":"circular"}`))
	previewUpdate := readViewerSSEUpdate(t, reader)
	if !strings.Contains(previewUpdate, `"session_id":"test"`) || !strings.Contains(previewUpdate, `"seq":2`) {
		t.Fatalf("preview update event should increment seq: %q", previewUpdate)
	}
}

func TestViewerServerPreviewEndpointMergesState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewViewerServer("127.0.0.1:0")
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	preview := getViewerPreview(t, server.URL()+"/sessions/test/preview")
	if len(preview) != 0 {
		t.Fatalf("initial preview should be empty: %#v", preview)
	}
	putViewerPayload(t, server.URL()+"/sessions/test/preview", []byte(`{"layout":"circular","show_alignment":true}`))
	putViewerPayload(t, server.URL()+"/sessions/test/preview", []byte(`{"show_alignment":false,"theme":"tree-yellow"}`))
	preview = getViewerPreview(t, server.URL()+"/sessions/test/preview")
	if preview["layout"] != "circular" {
		t.Fatalf("preview layout should be preserved after merge: %#v", preview)
	}
	if preview["show_alignment"] != false {
		t.Fatalf("preview show_alignment should be overwritten by latest value: %#v", preview)
	}
	if preview["theme"] != "tree-yellow" {
		t.Fatalf("preview theme should be stored: %#v", preview)
	}
}

func TestViewerServerStateEndpointRoundTripsWithoutSSEBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewViewerServer("127.0.0.1:0")
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	stateURL := server.URL() + "/sessions/test/state"
	initial := getViewerState(t, stateURL)
	if strings.TrimSpace(initial) != "{}" {
		t.Fatalf("initial state = %s", initial)
	}
	putViewerPayload(t, stateURL, []byte(`{"schema_version":1,"reactree":{"layout":"circular"}}`))
	got := getViewerState(t, stateURL)
	if !strings.Contains(got, `"layout":"circular"`) {
		t.Fatalf("viewer state did not round-trip: %s", got)
	}
}

func readViewerSSEUpdate(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE event: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
			continue
		}
		lines = append(lines, line)
	}
}

func putViewerPayload(t *testing.T, url string, payload []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d", resp.StatusCode)
	}
}

func getViewerPreview(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("preview request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d", resp.StatusCode)
	}
	var preview map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	return preview
}

func getViewerState(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("state request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	return string(body)
}
