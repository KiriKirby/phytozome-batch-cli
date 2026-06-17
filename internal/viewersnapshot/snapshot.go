package viewersnapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/phylo"
)

const (
	Format           = "phgo-viewer-snapshot"
	SchemaVersion    = 3
	MinSchemaVersion = 1
)

type Snapshot struct {
	Format        string              `json:"format"`
	SchemaVersion int                 `json:"schema_version"`
	CreatedAt     time.Time           `json:"created_at"`
	Producer      string              `json:"producer,omitempty"`
	Payload       phylo.ViewerPayload `json:"payload"`
	ViewerState   json.RawMessage     `json:"viewer_state,omitempty"`
}

func New(payload phylo.ViewerPayload, viewerState json.RawMessage, now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now()
	}
	viewerState = bytes.TrimSpace(viewerState)
	if len(viewerState) == 0 {
		viewerState = json.RawMessage(`{}`)
	}
	return Snapshot{
		Format:        Format,
		SchemaVersion: SchemaVersion,
		CreatedAt:     now,
		Producer:      "phytozome-go tree viewer",
		Payload:       payload,
		ViewerState:   viewerState,
	}
}

func Encode(snapshot Snapshot) ([]byte, error) {
	snapshot.Format = Format
	if snapshot.SchemaVersion == 0 {
		snapshot.SchemaVersion = SchemaVersion
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}
	if len(bytes.TrimSpace(snapshot.ViewerState)) == 0 {
		snapshot.ViewerState = json.RawMessage(`{}`)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func Decode(data []byte) (Snapshot, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Snapshot{}, fmt.Errorf("PGV file is empty")
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("parse PGV JSON: %w", err)
	}
	if snapshot.Format != Format {
		return Snapshot{}, fmt.Errorf("unsupported PGV format %q", snapshot.Format)
	}
	if snapshot.SchemaVersion < MinSchemaVersion || snapshot.SchemaVersion > SchemaVersion {
		return Snapshot{}, fmt.Errorf("unsupported PGV schema version %d", snapshot.SchemaVersion)
	}
	if snapshot.Payload.SchemaVersion == 0 {
		return Snapshot{}, fmt.Errorf("PGV payload is missing")
	}
	if len(bytes.TrimSpace(snapshot.ViewerState)) == 0 {
		snapshot.ViewerState = json.RawMessage(`{}`)
	}
	if !json.Valid(snapshot.ViewerState) {
		return Snapshot{}, fmt.Errorf("PGV viewer_state is not valid JSON")
	}
	return snapshot, nil
}
