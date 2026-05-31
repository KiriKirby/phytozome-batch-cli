package phylo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
)

type ArtifactSet struct {
	BaseDir         string
	Manifest        RunManifest
	Metadata        Metadata
	Payload         ViewerPayload
	InputFASTA      string
	RuntimeRequest  string
	RuntimeResponse string
	AlignedFASTA    string
	Newick          string
}

func (a ArtifactSet) Write() error {
	if strings.TrimSpace(a.BaseDir) == "" {
		return fmt.Errorf("artifact base directory is empty")
	}
	if err := os.MkdirAll(a.BaseDir, 0o755); err != nil {
		return err
	}
	write := func(name string, data []byte) error {
		return appfs.WriteFileAtomic(filepath.Join(a.BaseDir, name), data, 0o644)
	}
	if err := write("input.fasta", []byte(a.InputFASTA)); err != nil {
		return err
	}
	if data, err := MarshalMetadata(a.Metadata); err != nil {
		return err
	} else if err := write("input.meta.json", data); err != nil {
		return err
	}
	if strings.TrimSpace(a.RuntimeRequest) != "" {
		if err := write("runtime-request.json", []byte(a.RuntimeRequest)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(a.RuntimeResponse) != "" {
		if err := write("runtime-response.json", []byte(a.RuntimeResponse)); err != nil {
			return err
		}
	}
	if data, err := json.MarshalIndent(a.Payload, "", "  "); err != nil {
		return err
	} else if err := write("viewer.payload.json", data); err != nil {
		return err
	}
	if strings.TrimSpace(a.AlignedFASTA) != "" {
		if err := write("aligned.fasta", []byte(a.AlignedFASTA)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(a.Newick) != "" {
		if err := write("tree.nwk", []byte(strings.TrimSpace(a.Newick)+"\n")); err != nil {
			return err
		}
	}
	if data, err := json.MarshalIndent(a.Manifest, "", "  "); err != nil {
		return err
	} else if err := write("run.manifest.json", data); err != nil {
		return err
	}
	return nil
}
