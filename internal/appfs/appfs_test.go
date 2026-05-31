package appfs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomicConcurrentWritersLeaveCompleteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	values := [][]byte{
		[]byte(`{"value":"first"}`),
		[]byte(`{"value":"second"}`),
		[]byte(`{"value":"third"}`),
	}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		data := values[i%len(values)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WriteFileAtomic(path, data, 0o644); err != nil {
				t.Errorf("WriteFileAtomic: %v", err)
			}
		}()
	}
	wg.Wait()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	for _, value := range values {
		if string(got) == string(value) {
			return
		}
	}
	t.Fatalf("final file is not one complete write: %q", got)
}

func TestCachePathRejectsEscapingComponents(t *testing.T) {
	root := t.TempDir()
	unsafeParts := []string{"..", "../escape", filepath.Join("safe", "..", "..", "escape")}
	if runtime.GOOS == "windows" {
		unsafeParts = append(unsafeParts, `..\escape`)
	}
	for _, part := range unsafeParts {
		if _, err := cachePath(root, part); err == nil {
			t.Fatalf("cachePath accepted unsafe component %q", part)
		}
	}
	if _, err := cachePath(root, filepath.Join("safe", "child")); err != nil {
		t.Fatalf("cachePath rejected safe nested component: %v", err)
	}
}

func TestRemoveCacheSubtreeRejectsRootDeletion(t *testing.T) {
	err := RemoveCacheSubtree("")
	if err == nil {
		t.Fatal("expected root deletion to be rejected")
	}
	if !strings.Contains(err.Error(), "entire cache root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectFolderDisabledUsesDefaultDir(t *testing.T) {
	defaultDir := filepath.Join(t.TempDir(), "output")
	t.Setenv("PHYTOZOME_GO_DISABLE_FOLDER_PICKER", "1")

	got, err := SelectFolder("Export", defaultDir)
	if err != nil {
		t.Fatalf("SelectFolder returned error: %v", err)
	}
	want, err := filepath.Abs(defaultDir)
	if err != nil {
		t.Fatalf("Abs default dir: %v", err)
	}
	if got != want {
		t.Fatalf("SelectFolder = %q, want %q", got, want)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("selected default dir was not created: info=%#v err=%v", info, err)
	}
}
