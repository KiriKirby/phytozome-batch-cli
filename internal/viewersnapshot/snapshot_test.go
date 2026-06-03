package viewersnapshot

import (
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/phylo"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 30, 14, 0, 0, 0, time.UTC)
	in := New(phylo.ViewerPayload{
		SchemaVersion: 1,
		SessionID:     "viewer",
		UpdatedAt:     now,
		Newick:        "(A,B);",
		Metadata: phylo.Metadata{
			SchemaVersion: 1,
			GeneratedAt:   now,
		},
	}, []byte(`{"schema_version":1,"reactree":{"layout":"rectangular"}}`), now)

	data, err := Encode(in)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if !strings.Contains(string(data), `"format": "phgo-viewer-snapshot"`) {
		t.Fatalf("encoded snapshot missing format: %s", data)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if got.SchemaVersion != SchemaVersion || got.Payload.Newick != "(A,B);" {
		t.Fatalf("unexpected decoded snapshot: %#v", got)
	}
	if !strings.Contains(string(got.ViewerState), `"layout": "rectangular"`) {
		t.Fatalf("viewer state not preserved: %s", got.ViewerState)
	}
}

func TestDecodeRejectsUnsupportedVersion(t *testing.T) {
	_, err := Decode([]byte(`{"format":"phgo-viewer-snapshot","schema_version":99,"payload":{"schema_version":1},"viewer_state":{}}`))
	if err == nil {
		t.Fatal("Decode returned nil error for unsupported version")
	}
}

func TestDecodeAcceptsV1Snapshot(t *testing.T) {
	got, err := Decode([]byte(`{"format":"phgo-viewer-snapshot","schema_version":1,"payload":{"schema_version":1,"newick":"(A,B);"},"viewer_state":{"schema_version":1}}`))
	if err != nil {
		t.Fatalf("Decode returned error for v1 snapshot: %v", err)
	}
	if got.SchemaVersion != 1 || got.Payload.Newick != "(A,B);" {
		t.Fatalf("unexpected v1 snapshot decode: %#v", got)
	}
}
