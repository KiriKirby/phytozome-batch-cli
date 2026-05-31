package phylo

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed viewer_assets/**
var viewerAssets embed.FS

type ViewerState struct {
	mu       sync.RWMutex
	payloads map[string]ViewerPayload
	previews map[string]map[string]any
	states   map[string]json.RawMessage
	seq      uint64
	sessions map[string]uint64
	notify   chan struct{}
}

type ViewerServer struct {
	addr  string
	state ViewerState
	srv   *http.Server
	ln    net.Listener
}

func NewViewerServer(addr string) *ViewerServer {
	return &ViewerServer{
		addr: addr,
		state: ViewerState{
			payloads: map[string]ViewerPayload{},
			previews: map[string]map[string]any{},
			states:   map[string]json.RawMessage{},
			sessions: map[string]uint64{},
			notify:   make(chan struct{}),
		},
	}
}

func (v *ViewerServer) Start(ctx context.Context) error {
	if strings.TrimSpace(v.addr) == "" {
		v.addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", v.addr)
	if err != nil {
		return err
	}
	v.ln = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/health", v.handleHealth)
	mux.HandleFunc("/sessions/", v.handleSession)
	mux.HandleFunc("/events/", v.handleEvents)
	mux.HandleFunc("/assets/", v.handleAsset)
	mux.HandleFunc("/phgo-icon.png", v.handleAsset)
	mux.HandleFunc("/favicon.ico", v.handleAsset)
	v.srv = &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = v.srv.Shutdown(context.Background())
	}()
	go func() {
		_ = v.srv.Serve(ln)
	}()
	return nil
}

func (v *ViewerServer) URL() string {
	if v == nil || v.ln == nil {
		return ""
	}
	return "http://" + v.ln.Addr().String()
}

func (v *ViewerServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (v *ViewerServer) handleSession(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	sessionID := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		v.handleSessionPage(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "payload" && r.Method == http.MethodGet:
		v.handlePayloadGet(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "payload" && r.Method == http.MethodPut:
		v.handlePayloadPut(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "preview" && r.Method == http.MethodGet:
		v.handlePreviewGet(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "preview" && r.Method == http.MethodPut:
		v.handlePreviewPut(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodGet:
		v.handleViewerStateGet(w, r, sessionID)
	case len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodPut:
		v.handleViewerStatePut(w, r, sessionID)
	default:
		http.NotFound(w, r)
	}
}

func (v *ViewerServer) handleSessionPage(w http.ResponseWriter, r *http.Request, sessionID string) {
	v.state.mu.RLock()
	payload := v.state.payloads[sessionID]
	seq := v.state.sessions[sessionID]
	v.state.mu.RUnlock()
	empty := payload.SessionID == "" || !strings.EqualFold(payload.SessionID, sessionID)
	state := "empty"
	if !empty {
		state = "ready"
	}
	html, err := viewerAsset("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	html = bytesReplaceAll(html, []byte("__PHGO_SESSION__"), []byte(htmlEscape(sessionID)))
	html = bytesReplaceAll(html, []byte("__PHGO_STATE__"), []byte(htmlEscape(state)))
	html = bytesReplaceAll(html, []byte("__PHGO_SEQ__"), []byte(strconv.FormatUint(seq, 10)))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setViewerNoStoreHeaders(w)
	_, _ = w.Write(html)
}

func (v *ViewerServer) handlePayloadGet(w http.ResponseWriter, r *http.Request, sessionID string) {
	v.state.mu.RLock()
	payload, ok := v.state.payloads[sessionID]
	seq := v.state.sessions[sessionID]
	v.state.mu.RUnlock()
	if !ok {
		payload = ViewerPayload{SchemaVersion: 1, SessionID: sessionID, UpdatedAt: time.Now()}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", strconv.FormatUint(seq, 10))
	_ = json.NewEncoder(w).Encode(payload)
}

func (v *ViewerServer) handlePayloadPut(w http.ResponseWriter, r *http.Request, sessionID string) {
	defer r.Body.Close()
	var payload ViewerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.SessionID = strings.TrimSpace(sessionID)
	if payload.UpdatedAt.IsZero() {
		payload.UpdatedAt = time.Now()
	}
	v.state.mu.Lock()
	v.state.payloads[sessionID] = payload
	v.state.seq++
	v.state.sessions[sessionID] = v.state.seq
	v.state.broadcastLocked()
	v.state.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (v *ViewerServer) handlePreviewGet(w http.ResponseWriter, r *http.Request, sessionID string) {
	v.state.mu.RLock()
	preview := clonePreview(v.state.previews[sessionID])
	seq := v.state.sessions[sessionID]
	v.state.mu.RUnlock()
	if preview == nil {
		preview = map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", strconv.FormatUint(seq, 10))
	_ = json.NewEncoder(w).Encode(preview)
}

func (v *ViewerServer) handlePreviewPut(w http.ResponseWriter, r *http.Request, sessionID string) {
	defer r.Body.Close()
	var preview map[string]any
	if err := json.NewDecoder(r.Body).Decode(&preview); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v.state.mu.Lock()
	if v.state.previews == nil {
		v.state.previews = map[string]map[string]any{}
	}
	if v.state.previews[sessionID] == nil {
		v.state.previews[sessionID] = map[string]any{}
	}
	for key, value := range preview {
		v.state.previews[sessionID][key] = value
	}
	v.state.seq++
	v.state.sessions[sessionID] = v.state.seq
	v.state.broadcastLocked()
	v.state.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (v *ViewerServer) handleViewerStateGet(w http.ResponseWriter, r *http.Request, sessionID string) {
	v.state.mu.RLock()
	state := cloneRawJSON(v.state.states[sessionID])
	seq := v.state.sessions[sessionID]
	v.state.mu.RUnlock()
	if len(bytes.TrimSpace(state)) == 0 {
		state = json.RawMessage(`{}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", strconv.FormatUint(seq, 10))
	_, _ = w.Write(state)
}

func (v *ViewerServer) handleViewerStatePut(w http.ResponseWriter, r *http.Request, sessionID string) {
	defer r.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		http.Error(w, "viewer state must be valid JSON", http.StatusBadRequest)
		return
	}
	v.state.mu.Lock()
	if v.state.states == nil {
		v.state.states = map[string]json.RawMessage{}
	}
	v.state.states[sessionID] = cloneRawJSON(raw)
	v.state.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (v *ViewerServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if sessionID == "" {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeEvent := func() {
		v.state.mu.RLock()
		seq := v.state.sessions[sessionID]
		v.state.mu.RUnlock()
		_, _ = fmt.Fprintf(w, "event: update\ndata: {\"session_id\":%q,\"seq\":%d}\n\n", sessionID, seq)
		flusher.Flush()
	}
	writeEvent()
	for {
		v.state.mu.RLock()
		notify := v.state.notify
		v.state.mu.RUnlock()
		select {
		case <-r.Context().Done():
			return
		case <-notify:
			writeEvent()
		}
	}
}

func (v *ViewerServer) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	data, err := viewerAsset(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js", ".mjs":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		w.Header().Set("Content-Type", http.DetectContentType(data))
	}
	setViewerNoStoreHeaders(w)
	_, _ = w.Write(data)
}

func viewerAsset(name string) ([]byte, error) {
	name = strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(name)), "/")
	if strings.EqualFold(name, "favicon.ico") {
		name = "phgo-icon.png"
	}
	if name == "" || name == "." || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid viewer asset path")
	}
	return viewerAssets.ReadFile(path.Join("viewer_assets", name))
}

func setViewerNoStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func (v *ViewerState) broadcastLocked() {
	old := v.notify
	v.notify = make(chan struct{})
	close(old)
}

func clonePreview(preview map[string]any) map[string]any {
	if len(preview) == 0 {
		return nil
	}
	out := make(map[string]any, len(preview))
	for key, value := range preview {
		out[key] = value
	}
	return out
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}

func htmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return value
}

func bytesReplaceAll(data []byte, old []byte, replacement []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), string(old), string(replacement)))
}
