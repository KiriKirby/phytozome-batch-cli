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
	"github.com/klauspost/compress/zstd"
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
	var partSize int64
	var simulateArchiveSize int64
	var sample bool
	flag.StringVar(&outPath, "out", "", "output compressed .pgd.zst path")
	flag.StringVar(&manifestPath, "manifest", "", "output manifest.json path")
	flag.StringVar(&downloadURL, "download-url", "", "final raw download URL for the .pgd file")
	flag.StringVar(&sourceURL, "source-url", labelname.GeneInfoDirectoryURL, "NCBI GENE_INFO directory URL")
	flag.IntVar(&downloadWorkers, "download-workers", 8, "parallel NCBI split-file downloads")
	flag.Int64Var(&partSize, "part-size", 4*1024*1024, "compressed archive part size in bytes")
	flag.Int64Var(&simulateArchiveSize, "simulate-archive-size", 0, "write a deterministic fake compressed archive of this size and split it without downloading/building")
	flag.BoolVar(&sample, "sample", false, "build from a bundled small sample instead of NCBI")
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
	if simulateArchiveSize > 0 {
		return runSimulatedArchiveBuild(outPath, manifestPath, downloadURL, partSize, simulateArchiveSize)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	sourceDir, err := os.MkdirTemp("", "phgo-gene-info-source-*")
	if err != nil {
		return fmt.Errorf("create source download directory: %w", err)
	}
	defer os.RemoveAll(sourceDir)
	gzPaths, remote, err := buildSourceGZPaths(ctx, sourceDir, sample, sourceURL, downloadWorkers)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "Using %d source file(s) (%s total)\n", len(gzPaths), formatBytes(remote.ContentLength))

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
	if err := compressFile(tempDB, outPath); err != nil {
		return err
	}
	archiveInfo, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("stat compressed database: %w", err)
	}

	manifest := labelname.PrebuiltGeneInfoManifest{
		SchemaVersion:       info.SchemaVersion,
		URL:                 "",
		SHA256:              sum,
		ContentLength:       archiveInfo.Size(),
		RecordCount:         info.RecordCount,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		SourceURL:           remote.URL,
		SourceLastModified:  remote.LastModifiedRaw,
		SourceContentLength: remote.ContentLength,
	}
	if err := splitArchive(outPath, partSize, &manifest, downloadURL); err != nil {
		return err
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

func runSimulatedArchiveBuild(outPath string, manifestPath string, downloadURL string, partSize int64, archiveSize int64) error {
	if archiveSize <= 0 {
		return fmt.Errorf("-simulate-archive-size must be positive")
	}
	if err := writeDeterministicArchive(outPath, archiveSize); err != nil {
		return err
	}
	sum, err := fileSHA256(outPath)
	if err != nil {
		return err
	}
	fixed := time.Date(2026, 6, 10, 5, 29, 0, 0, time.UTC)
	manifest := labelname.PrebuiltGeneInfoManifest{
		SchemaVersion:       "2",
		URL:                 "",
		SHA256:              sum,
		ContentLength:       archiveSize,
		RecordCount:         0,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		SourceURL:           "simulate://archive",
		SourceLastModified:  fixed.Format(http.TimeFormat),
		SourceContentLength: archiveSize,
	}
	if err := splitArchive(outPath, partSize, &manifest, downloadURL); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal simulated manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write simulated manifest: %w", err)
	}
	if len(manifest.Parts) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Simulated archive %s split into %d part(s) of at most %s\n", formatBytes(archiveSize), len(manifest.Parts), formatBytes(partSize))
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "Simulated archive %s kept as single file\n", formatBytes(archiveSize))
	}
	return nil
}

func writeDeterministicArchive(path string, size int64) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create simulated archive: %w", err)
	}
	defer file.Close()
	buf := make([]byte, 1024*1024)
	for i := range buf {
		buf[i] = byte((i*31 + 17) % 251)
	}
	remaining := size
	for remaining > 0 {
		n := int64(len(buf))
		if remaining < n {
			n = remaining
		}
		if _, err := file.Write(buf[:n]); err != nil {
			return fmt.Errorf("write simulated archive: %w", err)
		}
		remaining -= n
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close simulated archive: %w", err)
	}
	return nil
}

func buildSourceGZPaths(ctx context.Context, sourceDir string, sample bool, sourceURL string, downloadWorkers int) ([]string, labelname.GeneInfoMetadata, error) {
	if sample {
		path, remote, err := buildSampleGeneInfoSource(sourceDir)
		if err != nil {
			return nil, labelname.GeneInfoMetadata{}, err
		}
		return []string{path}, remote, nil
	}
	parts, err := labelname.FetchGeneInfoDirectoryParts(ctx, sourceURL)
	if err != nil {
		return nil, labelname.GeneInfoMetadata{}, err
	}
	remote := directoryMetadata(sourceURL, parts)
	_, _ = fmt.Fprintf(os.Stdout, "Using %d NCBI GENE_INFO split sources (%s total)\n", len(parts), formatBytes(remote.ContentLength))
	gzPaths, err := downloadSourceParts(ctx, parts, sourceDir, downloadWorkers)
	if err != nil {
		return nil, labelname.GeneInfoMetadata{}, err
	}
	return gzPaths, remote, nil
}

func buildSampleGeneInfoSource(sourceDir string) (string, labelname.GeneInfoMetadata, error) {
	path := filepath.Join(sourceDir, "symbolname-sample.gene_info.gz")
	content := strings.Join([]string{
		"3702\t1\tVND6\tAT5G62380\tVND6A\tGeneID:1\t-\t-\tvascular-related NAC-domain 6\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"3702\t2\tPAL1\tAT2G37040\tPAL1A\tGeneID:2\t-\t-\tphenyalanine ammonia-lyase 1\tprotein-coding\t-\t-\t-\t-\t20260610\t-",
		"",
	}, "\n")
	file, err := os.Create(path)
	if err != nil {
		return "", labelname.GeneInfoMetadata{}, fmt.Errorf("create sample gene_info: %w", err)
	}
	gz := gzip.NewWriter(file)
	if _, err := gz.Write([]byte(content)); err != nil {
		gz.Close()
		file.Close()
		return "", labelname.GeneInfoMetadata{}, fmt.Errorf("write sample gene_info: %w", err)
	}
	if err := gz.Close(); err != nil {
		file.Close()
		return "", labelname.GeneInfoMetadata{}, fmt.Errorf("close sample gene_info gzip: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", labelname.GeneInfoMetadata{}, fmt.Errorf("close sample gene_info file: %w", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return "", labelname.GeneInfoMetadata{}, fmt.Errorf("stat sample gene_info: %w", err)
	}
	fixed := time.Date(2026, 6, 10, 5, 29, 0, 0, time.UTC)
	remote := labelname.GeneInfoMetadata{
		URL:             "sample://gene_info.gz",
		LastModified:    fixed,
		LastModifiedRaw: fixed.Format(http.TimeFormat),
		ContentLength:   stat.Size(),
	}
	return path, remote, nil
}

func splitArchive(path string, partSize int64, manifest *labelname.PrebuiltGeneInfoManifest, downloadURL string) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if partSize <= 0 {
		partSize = 4 * 1024 * 1024
	}
	if downloadURL == "" {
		manifest.URL = path
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat archive for split: %w", err)
	}
	if info.Size() <= partSize {
		manifest.URL = downloadURL
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive for split: %w", err)
	}
	defer file.Close()
	plannedParts := planArchiveParts(info.Size(), partSize, downloadURL)
	parts := make([]labelname.PrebuiltGeneInfoPart, 0, len(plannedParts))
	for {
		buf := make([]byte, partSize)
		n, readErr := io.ReadFull(file, buf)
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			if n == 0 {
				break
			}
		} else if readErr != nil {
			return fmt.Errorf("read archive for split: %w", readErr)
		}
		partIndex := len(parts)
		if partIndex >= len(plannedParts) {
			return fmt.Errorf("archive split produced more parts than planned")
		}
		partPath := path + fmt.Sprintf(".part%03d", partIndex+1)
		if err := os.WriteFile(partPath, buf[:n], 0o644); err != nil {
			return fmt.Errorf("write archive part %s: %w", partPath, err)
		}
		part := plannedParts[partIndex]
		part.ContentLength = int64(n)
		parts = append(parts, part)
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	manifest.URL = ""
	manifest.Parts = parts
	manifest.ContentLength = 0
	return nil
}

func planArchiveParts(totalSize int64, partSize int64, downloadURL string) []labelname.PrebuiltGeneInfoPart {
	if totalSize <= 0 || partSize <= 0 || strings.TrimSpace(downloadURL) == "" || totalSize <= partSize {
		return nil
	}
	count := int((totalSize + partSize - 1) / partSize)
	parts := make([]labelname.PrebuiltGeneInfoPart, 0, count)
	for index := 1; index <= count; index++ {
		size := partSize
		if index == count {
			if remainder := totalSize % partSize; remainder > 0 {
				size = remainder
			}
		}
		parts = append(parts, labelname.PrebuiltGeneInfoPart{
			URL:           fmt.Sprintf("%s.part%03d", downloadURL, index),
			ContentLength: size,
		})
	}
	return parts
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

func compressFile(sourcePath string, destPath string) error {
	if strings.HasSuffix(strings.ToLower(destPath), ".zst") {
		return zstdFile(sourcePath, destPath)
	}
	return gzipFile(sourcePath, destPath)
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
	writer, err := gzip.NewWriterLevel(output, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := io.Copy(writer, input); err != nil {
		writer.Close()
		return fmt.Errorf("gzip database: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return nil
}

func zstdFile(sourcePath string, destPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open database for zstd: %w", err)
	}
	defer input.Close()
	output, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create compressed database: %w", err)
	}
	defer output.Close()
	writer, err := zstd.NewWriter(output, zstd.WithEncoderLevel(zstd.SpeedBestCompression), zstd.WithEncoderConcurrency(0))
	if err != nil {
		return fmt.Errorf("create zstd writer: %w", err)
	}
	if _, err := io.Copy(writer, input); err != nil {
		writer.Close()
		return fmt.Errorf("zstd database: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close zstd writer: %w", err)
	}
	return nil
}
