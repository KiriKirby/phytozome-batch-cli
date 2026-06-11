package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
	"github.com/KiriKirby/phytozome-go/internal/netconfig"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	var outPath string
	var manifestPath string
	var downloadURL string
	var sourceURL string
	var downloadWorkers int
	flag.StringVar(&outPath, "out", "", "output compressed .pgd.gz path")
	flag.StringVar(&manifestPath, "manifest", "", "output manifest.json path")
	flag.StringVar(&downloadURL, "download-url", "", "final raw download URL for the .pgd file")
	flag.StringVar(&sourceURL, "source-url", labelname.GeneInfoDirectoryURL, "NCBI GENE_INFO directory URL")
	flag.IntVar(&downloadWorkers, "download-workers", 8, "parallel NCBI split-file downloads")
	flag.Parse()

	if outPath == "" {
		return fmt.Errorf("-out is required")
	}
	if manifestPath == "" {
		return fmt.Errorf("-manifest is required")
	}
	if downloadURL == "" {
		return fmt.Errorf("-download-url is required")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	parts, err := labelname.FetchGeneInfoDirectoryParts(ctx, sourceURL)
	if err != nil {
		return err
	}
	remote := directoryMetadata(sourceURL, parts)
	_, _ = fmt.Fprintf(os.Stdout, "Using %d NCBI GENE_INFO split sources (%s total)\n", len(parts), formatBytes(remote.ContentLength))

	sourceDir, err := os.MkdirTemp("", "phgo-gene-info-source-*")
	if err != nil {
		return fmt.Errorf("create source download directory: %w", err)
	}
	defer os.RemoveAll(sourceDir)
	gzPaths, err := downloadSourceParts(ctx, parts, sourceDir, downloadWorkers)
	if err != nil {
		return err
	}

	tempDB := outPath + ".builddb"
	defer os.Remove(tempDB)

	if err := labelname.BuildGeneInfoDatabaseFromGZFiles(gzPaths, tempDB, remote, labelname.DownloadOptions{
		Workers: labelname.DefaultBuildWorkers(),
		Progress: func(event labelname.GeneInfoProgress) {
			_, _ = fmt.Fprintln(os.Stdout, labelname.FormatGeneInfoProgress(event))
		},
	}); err != nil {
		return err
	}

	info, err := labelname.InspectGeneInfoDatabase(tempDB)
	if err != nil {
		return err
	}
	sum, err := fileSHA256(tempDB)
	if err != nil {
		return err
	}
	if err := gzipFile(tempDB, outPath); err != nil {
		return err
	}
	archiveInfo, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("stat compressed database: %w", err)
	}

	manifest := labelname.PrebuiltGeneInfoManifest{
		SchemaVersion:       info.SchemaVersion,
		URL:                 downloadURL,
		SHA256:              sum,
		ContentLength:       archiveInfo.Size(),
		RecordCount:         info.RecordCount,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		SourceURL:           remote.URL,
		SourceLastModified:  remote.LastModifiedRaw,
		SourceContentLength: remote.ContentLength,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func directoryMetadata(sourceURL string, parts []labelname.GeneInfoSourceFile) labelname.GeneInfoMetadata {
	var total int64
	var latest time.Time
	for _, part := range parts {
		total += part.ContentLength
		if part.LastModified.After(latest) {
			latest = part.LastModified
		}
	}
	lastRaw := ""
	if !latest.IsZero() {
		lastRaw = latest.UTC().Format(http.TimeFormat)
	}
	return labelname.GeneInfoMetadata{
		URL:             strings.TrimSpace(sourceURL),
		LastModified:    latest,
		LastModifiedRaw: lastRaw,
		ContentLength:   total,
		Parts:           append([]labelname.GeneInfoSourceFile(nil), parts...),
	}
}

func downloadSourceParts(ctx context.Context, parts []labelname.GeneInfoSourceFile, dir string, workers int) ([]string, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("no NCBI GENE_INFO split sources to download")
	}
	if workers <= 0 {
		workers = 8
	}
	if max := netconfig.NetworkWorkerCount(len(parts)); workers > max {
		workers = max
	}
	if workers > 12 {
		workers = 12
	}
	if workers < 1 {
		workers = 1
	}
	paths := make([]string, len(parts))
	jobs := make(chan int)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var completed atomic.Int64
	var bytesDone atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				part := parts[idx]
				path, err := downloadSourcePart(ctx, part, dir)
				if err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
				paths[idx] = path
				bytes := bytesDone.Add(part.ContentLength)
				done := completed.Add(1)
				_, _ = fmt.Fprintf(os.Stdout, "Downloaded split source %d/%d (%s/%s): %s\n", done, len(parts), formatBytes(bytes), formatBytes(totalSourceBytes(parts)), part.Name)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for idx := range parts {
			select {
			case <-ctx.Done():
				return
			case jobs <- idx:
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case err := <-errCh:
		return nil, err
	}
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	for idx, path := range paths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("missing downloaded split source %s", parts[idx].Name)
		}
	}
	return paths, nil
}

func downloadSourcePart(ctx context.Context, part labelname.GeneInfoSourceFile, dir string) (string, error) {
	name := safeSourceFilename(part)
	path := filepath.Join(dir, name)
	if stat, err := os.Stat(path); err == nil && stat.Size() > 0 && (part.ContentLength <= 0 || stat.Size() == part.ContentLength) {
		return path, nil
	}
	client := netconfig.DefaultHTTPClient()
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, part.URL, nil)
		if err != nil {
			return "", fmt.Errorf("build NCBI split download request: %w", err)
		}
		req.Header.Set("User-Agent", "phytozome-go-symbolname-builder")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			err = writeSourcePart(path, resp, part)
			if err == nil {
				return path, nil
			}
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}
	}
	return "", fmt.Errorf("download NCBI split source %s: %w", part.URL, lastErr)
}

func writeSourcePart(path string, resp *http.Response, part labelname.GeneInfoSourceFile) error {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NCBI split source returned %s", resp.Status)
	}
	tmp := path + ".tmp-" + fmt.Sprint(time.Now().UnixNano())
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create split source %s: %w", tmp, err)
	}
	written, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write split source %s: %w", tmp, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close split source %s: %w", tmp, closeErr)
	}
	if part.ContentLength > 0 && written != part.ContentLength {
		_ = os.Remove(tmp)
		return fmt.Errorf("split source %s size mismatch: got %d want %d", part.Name, written, part.ContentLength)
	}
	if written <= 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("split source %s is empty", part.Name)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install split source %s: %w", path, err)
	}
	return nil
}

func safeSourceFilename(part labelname.GeneInfoSourceFile) string {
	name := strings.TrimSpace(part.Name)
	if name == "" {
		parsed, err := url.Parse(part.URL)
		if err == nil {
			name = filepath.Base(parsed.Path)
		}
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func totalSourceBytes(parts []labelname.GeneInfoSourceFile) int64 {
	var total int64
	for _, part := range parts {
		total += part.ContentLength
	}
	return total
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

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open database for sha256: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash database: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func gzipFile(sourcePath string, destPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open database for gzip: %w", err)
	}
	defer input.Close()
	output, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create compressed database: %w", err)
	}
	defer output.Close()
	writer := gzip.NewWriter(output)
	if _, err := io.Copy(writer, input); err != nil {
		writer.Close()
		return fmt.Errorf("gzip database: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return nil
}
