package labelname

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/netconfig"
	kgzip "github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	bolt "go.etcd.io/bbolt"
)

const (
	DefaultGeneInfoPGD              = "symbolname.pgd"
	DefaultPrebuiltGeneInfoManifest = "https://raw.githubusercontent.com/KiriKirby/phytozome-go-symbolname-db/symbolname-db/symbolname/manifest.json"
	geneDBSchemaVersion             = "3"
	geneDBBucketMeta                = "meta"
	geneDBBucketRecords             = "records"
	geneDBBucketIndex               = "index"
	geneDBMaxTokenHits              = 2048
	geneDBMaxRankedAliases          = 24
	prebuiltCopyBufferSize          = 4 * 1024 * 1024
)

type GeneInfoMetadata struct {
	URL             string
	LastModified    time.Time
	LastModifiedRaw string
	ContentLength   int64
}

type GeneDatabaseInfo struct {
	Path            string
	URL             string
	SchemaVersion   string
	LastModified    time.Time
	LastModifiedRaw string
	DownloadedAt    time.Time
	ContentLength   int64
	RecordCount     int64
}

type PrebuiltGeneInfoManifest struct {
	SchemaVersion       string                 `json:"schema_version"`
	URL                 string                 `json:"url"`
	Parts               []PrebuiltGeneInfoPart `json:"parts,omitempty"`
	SHA256              string                 `json:"sha256,omitempty"`
	ContentLength       int64                  `json:"content_length,omitempty"`
	RecordCount         int64                  `json:"record_count,omitempty"`
	GeneratedAt         string                 `json:"generated_at,omitempty"`
	SourceURL           string                 `json:"source_url,omitempty"`
	SourceLastModified  string                 `json:"source_last_modified,omitempty"`
	SourceContentLength int64                  `json:"source_content_length,omitempty"`
}

type PrebuiltGeneInfoPart struct {
	URL           string `json:"url"`
	ContentLength int64  `json:"content_length,omitempty"`
}

type GeneInfoInstallPlan struct {
	Remote   GeneInfoMetadata
	Prebuilt *PrebuiltGeneInfoManifest
}

type DownloadOptions struct {
	Workers  int
	Stdout   io.Writer
	Progress func(GeneInfoProgress)
}

type GeneInfoProgress struct {
	Stage          string
	Message        string
	CurrentBytes   int64
	TotalBytes     int64
	BytesPerSecond float64
	Records        int64
	Workers        int
	Done           bool
}

type geneRecord struct {
	ID                 uint64 `json:"id"`
	TaxID              string `json:"tax_id,omitempty"`
	GeneID             string `json:"gene_id,omitempty"`
	Symbol             string `json:"symbol,omitempty"`
	LocusTag           string `json:"locus_tag,omitempty"`
	Synonyms           string `json:"synonyms,omitempty"`
	DBXrefs            string `json:"db_xrefs,omitempty"`
	Chromosome         string `json:"chromosome,omitempty"`
	MapLocation        string `json:"map_location,omitempty"`
	Description        string `json:"description,omitempty"`
	TypeOfGene         string `json:"type_of_gene,omitempty"`
	SymbolAuthority    string `json:"symbol_from_nomenclature_authority,omitempty"`
	FullNameAuthority  string `json:"full_name_from_nomenclature_authority,omitempty"`
	NomenclatureStatus string `json:"nomenclature_status,omitempty"`
	OtherDesignations  string `json:"other_designations,omitempty"`
	ModificationDate   string `json:"modification_date,omitempty"`
	FeatureType        string `json:"feature_type,omitempty"`
}

type geneDB struct {
	path    string
	db      *bolt.DB
	rankMu  sync.Mutex
	rankLRU []string
	rankMap map[string][]rankedAlias
}

var (
	geneDBDefaultMu     sync.Mutex
	geneDBDefaultPath   string
	geneDBDefault       *geneDB
	geneDBInstallMu     sync.Mutex
	geneInfoHTTP        = netconfig.DefaultHTTPClient()
	symbolTokenRx       = regexp.MustCompile(`[A-Za-z][A-Za-z0-9'._-]{1,31}`)
	geneDBRankCacheSize = netconfig.ConfiguredInt("PHGO_SYMBOL_NAME_RANK_CACHE", 8192)
	prebuiltCopyPool    = sync.Pool{New: func() any {
		buffer := make([]byte, prebuiltCopyBufferSize)
		return &buffer
	}}
	geneInfoStopKeys = map[string]struct{}{
		"gene": {}, "genes": {}, "protein": {}, "proteins": {}, "domain": {}, "domains": {},
		"family": {}, "like": {}, "related": {}, "putative": {}, "hypothetical": {},
		"made": {}, "up": {},
		"tair": {}, "geneid": {}, "ncbi": {}, "ensembl": {}, "uniprot": {}, "refseq": {},
		"genbank": {}, "hgnc": {}, "mgi": {}, "rgd": {}, "zfin": {}, "sgd": {},
		"wormbase": {}, "flybase": {}, "mirbase": {},
	}
)

var ErrGeneInfoDatabaseMissing = errors.New("symbol name database is required")

func DefaultGeneInfoDatabasePath(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return DefaultGeneInfoPGD
	}
	return filepath.Join(root, DefaultGeneInfoPGD)
}

func DefaultDownloadWorkers() int {
	workers := runtime.GOMAXPROCS(0) * 4
	if workers < 8 {
		workers = 8
	}
	if networkWorkers := netconfig.DefaultNetworkWorkers(); networkWorkers > 0 && workers < networkWorkers/2 {
		workers = networkWorkers / 2
	}
	if workers > 64 {
		workers = 64
	}
	return workers
}

func resolvePrebuiltGeneInfoManifestURL() string {
	if value := strings.TrimSpace(os.Getenv("PHGO_SYMBOL_NAME_PGD_MANIFEST_URL")); value != "" {
		return value
	}
	return DefaultPrebuiltGeneInfoManifest
}

func FetchPrebuiltGeneInfoManifest(ctx context.Context) (PrebuiltGeneInfoManifest, error) {
	rawURL := resolvePrebuiltGeneInfoManifestURL()
	if rawURL == "" {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("prebuilt symbol name manifest URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("build prebuilt symbol name manifest request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go-symbolname")
	resp, err := geneInfoHTTP.Do(req)
	if err != nil {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("query prebuilt symbol name manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("prebuilt symbol name manifest returned %s", resp.Status)
	}
	var manifest PrebuiltGeneInfoManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("decode prebuilt symbol name manifest: %w", err)
	}
	if strings.TrimSpace(manifest.SchemaVersion) != geneDBSchemaVersion {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("prebuilt symbol name manifest schema %q does not match %q", manifest.SchemaVersion, geneDBSchemaVersion)
	}
	if strings.TrimSpace(manifest.URL) == "" && len(manifest.Parts) == 0 {
		return PrebuiltGeneInfoManifest{}, fmt.Errorf("prebuilt symbol name manifest is missing database URL")
	}
	return manifest, nil
}

func (m PrebuiltGeneInfoManifest) remoteMetadata() GeneInfoMetadata {
	lastRaw := strings.TrimSpace(m.SourceLastModified)
	lastModified, _ := http.ParseTime(lastRaw)
	return GeneInfoMetadata{
		URL:             m.SourceURL,
		LastModified:    lastModified,
		LastModifiedRaw: lastRaw,
		ContentLength:   m.SourceContentLength,
	}
}

func (m PrebuiltGeneInfoManifest) downloadSize() int64 {
	if m.ContentLength > 0 {
		return m.ContentLength
	}
	var total int64
	for _, part := range m.Parts {
		total += part.ContentLength
	}
	if total > 0 {
		return total
	}
	return m.SourceContentLength
}

func PreferredGeneInfoInstallPlan(ctx context.Context) (GeneInfoInstallPlan, error) {
	manifest, err := FetchPrebuiltGeneInfoManifest(ctx)
	if err != nil {
		return GeneInfoInstallPlan{}, err
	}
	return GeneInfoInstallPlan{
		Remote:   manifest.remoteMetadata(),
		Prebuilt: &manifest,
	}, nil
}

func (p GeneInfoInstallPlan) DownloadSize() int64 {
	if p.Prebuilt != nil {
		return p.Prebuilt.downloadSize()
	}
	return p.Remote.ContentLength
}

func (p GeneInfoInstallPlan) SourceLabel() string {
	if p.Prebuilt != nil {
		return "GitHub prebuilt symbol name database"
	}
	return "GitHub prebuilt symbol name database"
}

func (p GeneInfoInstallPlan) SourceURL() string {
	if p.Prebuilt != nil {
		return firstNonEmptyGeneInfoText(p.Prebuilt.URL, resolvePrebuiltGeneInfoManifestURL())
	}
	return p.Remote.URL
}

func (p GeneInfoInstallPlan) Install(ctx context.Context, dest string, options DownloadOptions) error {
	if p.Prebuilt != nil {
		return DownloadPrebuiltGeneInfoDatabase(ctx, dest, *p.Prebuilt, options)
	}
	return fmt.Errorf("prebuilt symbol name database manifest is required")
}

func InspectGeneInfoDatabase(path string) (GeneDatabaseInfo, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return GeneDatabaseInfo{}, fmt.Errorf("symbol name database path is empty")
	}
	db, err := bolt.Open(path, 0o444, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return GeneDatabaseInfo{}, err
	}
	defer db.Close()
	info := GeneDatabaseInfo{Path: path}
	err = db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(geneDBBucketMeta))
		if meta == nil {
			return fmt.Errorf("symbol name database metadata is missing")
		}
		info.URL = string(meta.Get([]byte("url")))
		info.SchemaVersion = string(meta.Get([]byte("schema_version")))
		info.LastModifiedRaw = string(meta.Get([]byte("last_modified")))
		info.LastModified, _ = http.ParseTime(info.LastModifiedRaw)
		info.DownloadedAt, _ = time.Parse(time.RFC3339Nano, string(meta.Get([]byte("downloaded_at"))))
		info.ContentLength, _ = strconv.ParseInt(string(meta.Get([]byte("content_length"))), 10, 64)
		info.RecordCount, _ = strconv.ParseInt(string(meta.Get([]byte("record_count"))), 10, 64)
		if info.SchemaVersion != geneDBSchemaVersion {
			return fmt.Errorf("unsupported symbol name database schema %q", info.SchemaVersion)
		}
		return nil
	})
	if err != nil {
		return GeneDatabaseInfo{}, err
	}
	return info, nil
}

func DownloadPrebuiltGeneInfoDatabase(ctx context.Context, dest string, manifest PrebuiltGeneInfoManifest, options DownloadOptions) error {
	dest = filepath.Clean(strings.TrimSpace(dest))
	if dest == "" {
		return fmt.Errorf("symbol name database path is empty")
	}
	rawURL := strings.TrimSpace(manifest.URL)
	if rawURL == "" && len(manifest.Parts) == 0 {
		return fmt.Errorf("prebuilt symbol name database URL is empty")
	}
	options.emitProgress(GeneInfoProgress{
		Stage:      "prepare",
		Message:    "Preparing prebuilt symbol name database download...",
		TotalBytes: manifest.downloadSize(),
	})
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create symbol name database directory: %w", err)
	}
	tmpDB := dest + ".tmp-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	defer os.Remove(tmpDB)
	out, err := os.Create(tmpDB)
	if err != nil {
		return fmt.Errorf("create prebuilt symbol name database %s: %w", tmpDB, err)
	}
	reporterWorkers := 1
	if len(manifest.Parts) > 0 {
		reporterWorkers = prebuiltPartDownloadWorkers(len(manifest.Parts))
	}
	reporter := newGeneInfoProgressReporter(options.Progress, "download", manifest.downloadSize(), reporterWorkers)
	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)
	options.emitProgress(GeneInfoProgress{
		Stage:      "extract",
		Message:    "Decompressing and writing symbol name database...",
		TotalBytes: manifest.downloadSize(),
		Workers:    reporterWorkers,
	})
	if len(manifest.Parts) > 0 {
		pipeReader, pipeWriter := io.Pipe()
		downloadErrCh := make(chan error, 1)
		partCtx, partCancel := context.WithCancel(ctx)
		go func() {
			err := downloadPrebuiltGeneInfoParts(partCtx, filepath.Dir(dest), manifest, pipeWriter, reporter)
			_ = pipeWriter.CloseWithError(err)
			downloadErrCh <- err
		}()
		if err := copyCompressedPrebuiltDatabase(writer, pipeReader, prebuiltArchiveURL(manifest)); err != nil {
			partCancel()
			_ = pipeReader.Close()
			if downloadErr := <-downloadErrCh; downloadErr != nil {
				out.Close()
				if !isPipeClosedAfterReaderFailure(downloadErr) {
					return downloadErr
				}
			}
			out.Close()
			return fmt.Errorf("write prebuilt symbol name database %s: %w", tmpDB, err)
		}
		_ = pipeReader.Close()
		partCancel()
		if downloadErr := <-downloadErrCh; downloadErr != nil {
			out.Close()
			return downloadErr
		}
	} else if isCompressedPrebuiltArchive(rawURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			out.Close()
			return fmt.Errorf("build prebuilt symbol name database request: %w", err)
		}
		req.Header.Set("User-Agent", "phytozome-go-symbolname")
		resp, err := geneInfoHTTP.Do(req)
		if err != nil {
			out.Close()
			return fmt.Errorf("download prebuilt symbol name database: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			out.Close()
			return fmt.Errorf("download prebuilt symbol name database returned %s", resp.Status)
		}
		if err := copyCompressedPrebuiltDatabase(writer, &progressReader{reader: resp.Body, reporter: reporter}, rawURL); err != nil {
			out.Close()
			return fmt.Errorf("write prebuilt symbol name database %s: %w", tmpDB, err)
		}
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			out.Close()
			return fmt.Errorf("build prebuilt symbol name database request: %w", err)
		}
		req.Header.Set("User-Agent", "phytozome-go-symbolname")
		resp, err := geneInfoHTTP.Do(req)
		if err != nil {
			out.Close()
			return fmt.Errorf("download prebuilt symbol name database: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			out.Close()
			return fmt.Errorf("download prebuilt symbol name database returned %s", resp.Status)
		}
		if _, err := copyWithPrebuiltBuffer(&progressWriter{writer: writer, reporter: reporter}, resp.Body); err != nil {
			out.Close()
			return fmt.Errorf("write prebuilt symbol name database %s: %w", tmpDB, err)
		}
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close prebuilt symbol name database %s: %w", tmpDB, err)
	}
	reporter.finish("Downloaded prebuilt symbol name database.")
	if want := strings.TrimSpace(strings.ToLower(manifest.SHA256)); want != "" {
		if got := fmt.Sprintf("%x", hasher.Sum(nil)); !strings.EqualFold(got, want) {
			return fmt.Errorf("prebuilt symbol name database sha256 mismatch: got %s want %s", got, want)
		}
	}
	info, err := InspectGeneInfoDatabase(tmpDB)
	if err != nil {
		return fmt.Errorf("inspect prebuilt symbol name database %s: %w", tmpDB, err)
	}
	if manifest.RecordCount > 0 && info.RecordCount != manifest.RecordCount {
		return fmt.Errorf("prebuilt symbol name database record count mismatch: got %d want %d", info.RecordCount, manifest.RecordCount)
	}
	if remote := manifest.remoteMetadata(); strings.TrimSpace(remote.LastModifiedRaw) != "" && info.LastModifiedRaw != remote.LastModifiedRaw {
		return fmt.Errorf("prebuilt symbol name database Last-Modified mismatch: got %q want %q", info.LastModifiedRaw, remote.LastModifiedRaw)
	}
	if err := os.Rename(tmpDB, dest); err != nil {
		return fmt.Errorf("install symbol name database %s: %w", dest, err)
	}
	resetDefaultGeneDB()
	options.emitProgress(GeneInfoProgress{
		Stage:        "complete",
		Message:      "Symbol name database is ready.",
		CurrentBytes: manifest.downloadSize(),
		TotalBytes:   manifest.downloadSize(),
		Done:         true,
	})
	return nil
}

func isPipeClosedAfterReaderFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	return strings.Contains(err.Error(), io.ErrClosedPipe.Error())
}

func prebuiltArchiveURL(manifest PrebuiltGeneInfoManifest) string {
	if rawURL := strings.TrimSpace(manifest.URL); rawURL != "" {
		return rawURL
	}
	if len(manifest.Parts) == 0 {
		return ""
	}
	rawURL := strings.TrimSpace(manifest.Parts[0].URL)
	lower := strings.ToLower(rawURL)
	for _, suffix := range []string{".part001", ".part01", ".001", ".01"} {
		if strings.HasSuffix(lower, suffix) {
			return rawURL[:len(rawURL)-len(suffix)]
		}
	}
	if idx := strings.LastIndex(lower, ".part"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
}

func isCompressedPrebuiltArchive(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".zst") || strings.HasSuffix(lower, ".zstd")
}

func copyCompressedPrebuiltDatabase(writer io.Writer, reader io.Reader, rawURL string) error {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if strings.HasSuffix(lower, ".zst") || strings.HasSuffix(lower, ".zstd") {
		zr, err := zstd.NewReader(reader)
		if err != nil {
			return fmt.Errorf("open prebuilt symbol name database zstd stream: %w", err)
		}
		defer zr.Close()
		if _, err := copyWithPrebuiltBuffer(writer, zr); err != nil {
			return err
		}
		return nil
	}
	gzReader, err := kgzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("open prebuilt symbol name database gzip stream: %w", err)
	}
	defer gzReader.Close()
	if _, err := copyWithPrebuiltBuffer(writer, gzReader); err != nil {
		return err
	}
	return nil
}

func copyWithPrebuiltBuffer(dst io.Writer, src io.Reader) (int64, error) {
	bufferPtr := prebuiltCopyPool.Get().(*[]byte)
	defer prebuiltCopyPool.Put(bufferPtr)
	return io.CopyBuffer(dst, src, *bufferPtr)
}

func downloadPrebuiltGeneInfoParts(ctx context.Context, tempRoot string, manifest PrebuiltGeneInfoManifest, writer io.Writer, reporter *geneInfoProgressReporter) error {
	if len(manifest.Parts) == 0 {
		return nil
	}
	tempRoot = filepath.Clean(strings.TrimSpace(tempRoot))
	if tempRoot == "" {
		tempRoot = os.TempDir()
	}
	tempDir, err := os.MkdirTemp(tempRoot, "symbolname-parts-*")
	if err != nil {
		return fmt.Errorf("create prebuilt symbol name database part staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	workers := prebuiltPartDownloadWorkers(len(manifest.Parts))
	type partResult struct {
		index int
		path  string
		size  int64
		err   error
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	results := make(chan partResult)
	var wg sync.WaitGroup
	stopWorkers := func() {
		close(jobs)
		wg.Wait()
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				path, size, err := downloadPrebuiltGeneInfoPart(ctx, tempDir, index, manifest.Parts[index], reporter)
				select {
				case results <- partResult{index: index, path: path, size: size, err: err}:
				case <-ctx.Done():
					if path != "" {
						_ = os.Remove(path)
					}
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	nextJob := 0
	inFlight := 0
	sendJob := func() bool {
		if nextJob >= len(manifest.Parts) {
			return false
		}
		select {
		case jobs <- nextJob:
			nextJob++
			inFlight++
			return true
		case <-ctx.Done():
			return false
		}
	}
	for inFlight < workers && sendJob() {
	}
	nextWrite := 0
	pending := make(map[int]partResult, workers*2)
	for nextWrite < len(manifest.Parts) {
		if result, ok := pending[nextWrite]; ok {
			if err := copyPrebuiltGeneInfoPartToWriter(writer, result.index, result.path, result.size); err != nil {
				cancel()
				stopWorkers()
				return fmt.Errorf("write prebuilt symbol name database part %d: %w", nextWrite+1, err)
			}
			_ = os.Remove(result.path)
			delete(pending, nextWrite)
			nextWrite++
			for inFlight < workers && len(pending) < workers*2 && sendJob() {
			}
			continue
		}
		result, ok := <-results
		if !ok {
			break
		}
		inFlight--
		if result.err != nil {
			cancel()
			stopWorkers()
			return result.err
		}
		if result.index == nextWrite {
			if err := copyPrebuiltGeneInfoPartToWriter(writer, result.index, result.path, result.size); err != nil {
				cancel()
				stopWorkers()
				return fmt.Errorf("write prebuilt symbol name database part %d: %w", result.index+1, err)
			}
			_ = os.Remove(result.path)
			nextWrite++
		} else {
			pending[result.index] = result
		}
		for inFlight < workers && len(pending) < workers*2 && sendJob() {
		}
	}
	stopWorkers()
	if nextWrite != len(manifest.Parts) {
		return fmt.Errorf("prebuilt symbol name database split download ended after %d/%d parts", nextWrite, len(manifest.Parts))
	}
	return nil
}

func copyPrebuiltGeneInfoPartToWriter(writer io.Writer, index int, path string, size int64) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("part %d was not staged", index+1)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	copied, err := copyWithPrebuiltBuffer(writer, file)
	if err != nil {
		return err
	}
	if size > 0 && copied != size {
		return fmt.Errorf("staged part size changed while writing: got %d want %d", copied, size)
	}
	return nil
}

func prebuiltPartDownloadWorkers(total int) int {
	workers := netconfig.NetworkWorkerCount(total)
	if workers > 8 {
		workers = 8
	}
	if configured := netconfig.ConfiguredInt("PHGO_SYMBOL_NAME_PREBUILT_PART_WORKERS", 0); configured > 0 {
		workers = configured
		if workers > total {
			workers = total
		}
		if workers > 32 {
			workers = 32
		}
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func downloadPrebuiltGeneInfoPart(ctx context.Context, tempDir string, index int, part PrebuiltGeneInfoPart, reporter *geneInfoProgressReporter) (string, int64, error) {
	rawURL := strings.TrimSpace(part.URL)
	if rawURL == "" {
		return "", 0, fmt.Errorf("prebuilt symbol name database part %d is missing URL", index+1)
	}
	var lastErr error
	for attempt := 1; attempt <= 6; attempt++ {
		path, size, err := downloadPrebuiltGeneInfoPartOnce(ctx, tempDir, index, rawURL, part.ContentLength, reporter)
		if err == nil {
			return path, size, nil
		}
		lastErr = err
		if !isRetryableDownloadError(err) || attempt == 6 {
			break
		}
		delay := retryDelayForDownloadError(err, attempt)
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", 0, fmt.Errorf("download prebuilt symbol name database part %d %s: %w", index+1, rawURL, lastErr)
}

func downloadPrebuiltGeneInfoPartOnce(ctx context.Context, tempDir string, index int, rawURL string, expected int64, reporter *geneInfoProgressReporter) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("build prebuilt symbol name database part request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go-symbolname")
	resp, err := geneInfoHTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, githubDownloadStatusError{
			statusCode: resp.StatusCode,
			status:     resp.Status,
			retryAfter: githubRetryAfter(resp.Header, time.Now()),
		}
	}
	file, err := os.CreateTemp(tempDir, fmt.Sprintf("symbolname-part-%03d-*", index+1))
	if err != nil {
		return "", 0, fmt.Errorf("create prebuilt symbol name database part file: %w", err)
	}
	path := file.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	limit := int64(0)
	if expected > 0 {
		limit = expected + 1
	}
	reader := resp.Body
	if limit > 0 {
		reader = io.NopCloser(io.LimitReader(resp.Body, limit))
	}
	copied, err := copyWithPrebuiltBuffer(&progressWriter{writer: file, reporter: reporter}, reader)
	if err != nil {
		_ = file.Close()
		return "", 0, err
	}
	if err := file.Close(); err != nil {
		return "", 0, fmt.Errorf("close prebuilt symbol name database part file: %w", err)
	}
	if expected > 0 && copied != expected {
		return "", 0, prebuiltPartSizeError{got: copied, want: expected}
	}
	ok = true
	return path, copied, nil
}

type prebuiltPartSizeError struct {
	got  int64
	want int64
}

func (e prebuiltPartSizeError) Error() string {
	return fmt.Sprintf("prebuilt symbol name database part size mismatch: got %d want %d", e.got, e.want)
}

type githubDownloadStatusError struct {
	statusCode int
	status     string
	retryAfter time.Duration
}

func (e githubDownloadStatusError) Error() string {
	return e.status
}

func isRetryableDownloadError(err error) bool {
	var sizeErr prebuiltPartSizeError
	if errors.As(err, &sizeErr) {
		return false
	}
	var status githubDownloadStatusError
	if errors.As(err, &status) {
		if status.statusCode == http.StatusTooManyRequests || status.statusCode >= 500 {
			return true
		}
		return status.statusCode == http.StatusForbidden && status.retryAfter > 0
	}
	return true
}

func retryDelayForDownloadError(err error, attempt int) time.Duration {
	var status githubDownloadStatusError
	if errors.As(err, &status) && status.retryAfter > 0 {
		return status.retryAfter
	}
	delay := time.Duration(attempt*attempt) * 500 * time.Millisecond
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func githubRetryAfter(header http.Header, now time.Time) time.Duration {
	if raw := strings.TrimSpace(header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if when, err := http.ParseTime(raw); err == nil && when.After(now) {
			return when.Sub(now)
		}
	}
	if raw := strings.TrimSpace(header.Get("X-RateLimit-Reset")); raw != "" {
		remaining := strings.TrimSpace(header.Get("X-RateLimit-Remaining"))
		if remaining != "" && remaining != "0" {
			return 0
		}
		if unixSeconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if when := time.Unix(unixSeconds, 0); when.After(now) {
				return when.Sub(now)
			}
		}
	}
	return 0
}

func SetDefaultGeneInfoDatabasePath(path string) {
	geneDBDefaultMu.Lock()
	defer geneDBDefaultMu.Unlock()
	path = filepath.Clean(strings.TrimSpace(path))
	if geneDBDefaultPath == path {
		return
	}
	if geneDBDefault != nil {
		_ = geneDBDefault.db.Close()
		geneDBDefault = nil
	}
	geneDBDefaultPath = path
}

func DefaultGeneInfoDatabaseCurrentPath() string {
	geneDBDefaultMu.Lock()
	path := geneDBDefaultPath
	geneDBDefaultMu.Unlock()
	return resolveDefaultGeneInfoDatabasePath(path)
}

func DefaultGeneInfoDatabaseAvailable() bool {
	db, ok := openDefaultGeneDB()
	return ok && db != nil
}

func EnsureDefaultGeneInfoDatabase(ctx context.Context, path string, progress func(string)) error {
	return EnsureDefaultGeneInfoDatabaseProgress(ctx, path, func(event GeneInfoProgress) {
		if progress != nil {
			progress(FormatGeneInfoProgress(event))
		}
	})
}

func EnsureDefaultGeneInfoDatabaseProgress(ctx context.Context, path string, progress func(GeneInfoProgress)) error {
	path = resolveDefaultGeneInfoDatabasePath(path)
	if path == "" {
		return fmt.Errorf("%w: database path is empty", ErrGeneInfoDatabaseMissing)
	}
	SetDefaultGeneInfoDatabasePath(path)
	if DefaultGeneInfoDatabaseAvailable() {
		return nil
	}
	geneDBInstallMu.Lock()
	defer geneDBInstallMu.Unlock()
	if DefaultGeneInfoDatabaseAvailable() {
		return nil
	}
	if progress != nil {
		progress(GeneInfoProgress{Stage: "metadata", Message: "Checking symbol name library metadata..."})
	}
	plan, err := PreferredGeneInfoInstallPlan(ctx)
	if err != nil {
		return fmt.Errorf("prepare symbol name database install: %w", err)
	}
	if progress != nil {
		progress(GeneInfoProgress{Stage: "download", Message: fmt.Sprintf("Downloading symbol name library (%s)...", formatBytes(plan.DownloadSize())), TotalBytes: plan.DownloadSize()})
	}
	if err := plan.Install(ctx, path, DownloadOptions{
		Workers:  DefaultDownloadWorkers(),
		Progress: progress,
	}); err != nil {
		return err
	}
	if progress != nil {
		progress(GeneInfoProgress{Stage: "complete", Message: "Symbol name database is ready.", TotalBytes: plan.DownloadSize(), CurrentBytes: plan.DownloadSize(), Done: true})
	}
	return nil
}

func (o DownloadOptions) emitProgress(event GeneInfoProgress) {
	if o.Progress != nil {
		o.Progress(event)
	}
}

func resetDefaultGeneDB() {
	geneDBDefaultMu.Lock()
	defer geneDBDefaultMu.Unlock()
	if geneDBDefault != nil {
		_ = geneDBDefault.db.Close()
		geneDBDefault = nil
	}
}

func openDefaultGeneDB() (*geneDB, bool) {
	geneDBDefaultMu.Lock()
	defer geneDBDefaultMu.Unlock()
	path := geneDBDefaultPath
	if path == "" {
		path = strings.TrimSpace(os.Getenv("PHGO_SYMBOL_NAME_PGD"))
	}
	if path == "" {
		if exe, err := os.Executable(); err == nil {
			path = DefaultGeneInfoDatabasePath(filepath.Dir(exe))
		}
	}
	if path == "" {
		return nil, false
	}
	path = filepath.Clean(path)
	if geneDBDefault != nil && sameCleanPath(geneDBDefault.path, path) {
		return geneDBDefault, true
	}
	if geneDBDefault != nil {
		_ = geneDBDefault.db.Close()
		geneDBDefault = nil
	}
	db, err := bolt.Open(path, 0o444, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return nil, false
	}
	if err := validateGeneInfoDatabaseHandle(db); err != nil {
		_ = db.Close()
		return nil, false
	}
	geneDBDefaultPath = path
	geneDBDefault = &geneDB{path: path, db: db}
	return geneDBDefault, true
}

func validateGeneInfoDatabaseHandle(db *bolt.DB) error {
	return db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(geneDBBucketMeta))
		if meta == nil {
			return fmt.Errorf("symbol name database metadata is missing")
		}
		if version := string(meta.Get([]byte("schema_version"))); version != geneDBSchemaVersion {
			return fmt.Errorf("unsupported symbol name database schema %q", version)
		}
		if tx.Bucket([]byte(geneDBBucketRecords)) == nil || tx.Bucket([]byte(geneDBBucketIndex)) == nil {
			return fmt.Errorf("symbol name database buckets are missing")
		}
		return nil
	})
}

func resolveDefaultGeneInfoDatabasePath(path string) string {
	path = strings.TrimSpace(path)
	if path != "" {
		return filepath.Clean(path)
	}
	if envPath := strings.TrimSpace(os.Getenv("PHGO_SYMBOL_NAME_PGD")); envPath != "" {
		return filepath.Clean(envPath)
	}
	if exe, err := os.Executable(); err == nil {
		return DefaultGeneInfoDatabasePath(filepath.Dir(exe))
	}
	return ""
}

func (g *geneDB) rank(request AliasRankRequest) ([]rankedAlias, bool) {
	if g == nil || g.db == nil {
		return nil, false
	}
	cacheKey := aliasRankCacheKey(request)
	if cached, ok := g.rankCacheGet(cacheKey); ok {
		return cached, true
	}
	queryTerms := request.geneInfoTerms()
	if len(queryTerms) == 0 {
		g.rankCachePut(cacheKey, nil)
		return nil, true
	}
	scores := make(map[string]rankedAlias, 32)
	_ = g.db.View(func(tx *bolt.Tx) error {
		index := tx.Bucket([]byte(geneDBBucketIndex))
		records := tx.Bucket([]byte(geneDBBucketRecords))
		if index == nil || records == nil {
			return nil
		}
		recordHits := make(map[uint64][]geneInfoTerm, len(queryTerms)*8)
		for _, term := range queryTerms {
			prefix := []byte(term.Key + "\x00")
			cursor := index.Cursor()
			hits := 0
			for key, value := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = cursor.Next() {
				if hits >= geneDBMaxTokenHits {
					break
				}
				hits++
				id := binary.BigEndian.Uint64(key[len(prefix):])
				weight := 0
				if len(value) > 0 {
					weight = int(value[0])
				}
				recordHits[id] = append(recordHits[id], geneInfoTerm{
					Raw:    term.Raw,
					Weight: weight + term.Weight,
					TaxID:  term.TaxID,
				})
			}
		}
		if len(recordHits) == 0 {
			return nil
		}
		for id, hits := range recordHits {
			record, ok := decodeGeneRecord(records.Get(u64key(id)))
			if !ok {
				continue
			}
			for _, candidate := range record.symbolNameCandidates() {
				if !isUsableRankedAliasText(candidate) {
					continue
				}
				key := normalizeAliasKey(candidate)
				if key == "" {
					continue
				}
				bestScore := -1
				for _, term := range hits {
					score := term.Weight
					if strings.EqualFold(term.Raw, candidate) {
						score += 220
					} else if record.hasSymbolNameCandidateEqualTo(term.Raw) {
						score += 20
					}
					if strings.EqualFold(term.Raw, record.GeneID) || strings.EqualFold(term.Raw, record.LocusTag) {
						score += 60
					}
					if term.TaxID != "" && term.TaxID == record.TaxID {
						score += 40
					}
					if score > bestScore {
						bestScore = score
					}
				}
				if bestScore < 0 {
					continue
				}
				score := bestScore + symbolNameCandidateScore(candidate)
				if strings.EqualFold(candidate, record.Symbol) {
					score += 40
				}
				current := scores[key]
				if score > current.Score || (score == current.Score && len(candidate) < len(current.Text)) {
					scores[key] = rankedAlias{Text: candidate, Score: score, Family: symbolFamily(candidate)}
				}
			}
		}
		return nil
	})
	if len(scores) == 0 {
		g.rankCachePut(cacheKey, nil)
		return nil, true
	}
	out := make([]rankedAlias, 0, len(scores))
	for _, item := range scores {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if len(out[i].Text) != len(out[j].Text) {
			return len(out[i].Text) < len(out[j].Text)
		}
		return strings.ToLower(out[i].Text) < strings.ToLower(out[j].Text)
	})
	if len(out) > geneDBMaxRankedAliases {
		out = out[:geneDBMaxRankedAliases]
	}
	g.rankCachePut(cacheKey, out)
	return out, true
}

func (g *geneDB) rankCacheGet(key string) ([]rankedAlias, bool) {
	if g == nil || strings.TrimSpace(key) == "" || geneDBRankCacheSize <= 0 {
		return nil, false
	}
	g.rankMu.Lock()
	defer g.rankMu.Unlock()
	if g.rankMap == nil {
		return nil, false
	}
	items, ok := g.rankMap[key]
	if !ok {
		return nil, false
	}
	return cloneRankedAliases(items), true
}

func (g *geneDB) rankCachePut(key string, items []rankedAlias) {
	if g == nil || strings.TrimSpace(key) == "" || geneDBRankCacheSize <= 0 {
		return
	}
	g.rankMu.Lock()
	defer g.rankMu.Unlock()
	if g.rankMap == nil {
		g.rankMap = make(map[string][]rankedAlias, geneDBRankCacheSize)
	}
	if _, exists := g.rankMap[key]; !exists {
		g.rankLRU = append(g.rankLRU, key)
	}
	g.rankMap[key] = cloneRankedAliases(items)
	for len(g.rankLRU) > geneDBRankCacheSize {
		oldest := g.rankLRU[0]
		copy(g.rankLRU, g.rankLRU[1:])
		g.rankLRU = g.rankLRU[:len(g.rankLRU)-1]
		delete(g.rankMap, oldest)
	}
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "unknown size"
	}
	units := []string{"B", "KiB", "MiB", "GiB"}
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

func FormatGeneInfoProgress(event GeneInfoProgress) string {
	message := strings.TrimSpace(event.Message)
	if message == "" {
		switch event.Stage {
		case "download":
			message = "Downloading NCBI Gene symbol name library"
		case "extract":
			message = "Decompressing and writing symbol name database"
		case "build":
			message = "Building symbol name database"
		default:
			message = "Preparing symbol name database"
		}
	}
	parts := []string{message}
	if event.TotalBytes > 0 && event.CurrentBytes > 0 {
		percent := float64(event.CurrentBytes) * 100 / float64(event.TotalBytes)
		parts = append(parts, fmt.Sprintf("%.1f%%", percent))
		parts = append(parts, fmt.Sprintf("%s/%s", formatBytes(event.CurrentBytes), formatBytes(event.TotalBytes)))
	} else if event.CurrentBytes > 0 {
		parts = append(parts, formatBytes(event.CurrentBytes))
	} else if event.TotalBytes > 0 {
		parts = append(parts, formatBytes(event.TotalBytes))
	}
	if event.BytesPerSecond > 0 && !event.Done {
		parts = append(parts, fmt.Sprintf("%s/s", formatBytes(int64(event.BytesPerSecond))))
	}
	if event.Workers > 1 && event.Stage == "download" {
		parts = append(parts, fmt.Sprintf("%d workers", event.Workers))
	}
	if event.Records > 0 {
		parts = append(parts, fmt.Sprintf("%d records", event.Records))
	}
	return strings.Join(parts, " | ")
}

type geneInfoProgressReporter struct {
	mu       sync.Mutex
	progress func(GeneInfoProgress)
	stage    string
	total    int64
	current  int64
	records  int64
	workers  int
	started  time.Time
	last     time.Time
}

func newGeneInfoProgressReporter(progress func(GeneInfoProgress), stage string, total int64, workers int) *geneInfoProgressReporter {
	now := time.Now()
	reporter := &geneInfoProgressReporter{
		progress: progress,
		stage:    stage,
		total:    total,
		workers:  workers,
		started:  now,
		last:     now.Add(-time.Second),
	}
	reporter.emit("Starting "+stage+"...", false, true)
	return reporter
}

func (r *geneInfoProgressReporter) addBytes(n int) {
	if r == nil || n <= 0 {
		return
	}
	r.mu.Lock()
	r.current += int64(n)
	r.emitLocked("", false, false)
	r.mu.Unlock()
}

func (r *geneInfoProgressReporter) setRecords(records int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.records = records
	r.emitLocked("", false, false)
	r.mu.Unlock()
}

func (r *geneInfoProgressReporter) finish(message string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.total > 0 && r.current < r.total {
		r.current = r.total
	}
	r.emitLocked(message, true, true)
	r.mu.Unlock()
}

func (r *geneInfoProgressReporter) emit(message string, done bool, force bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emitLocked(message, done, force)
}

func (r *geneInfoProgressReporter) emitLocked(message string, done bool, force bool) {
	if r.progress == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(r.last) < 250*time.Millisecond {
		return
	}
	r.last = now
	elapsed := now.Sub(r.started).Seconds()
	speed := 0.0
	if elapsed > 0 && r.current > 0 {
		speed = float64(r.current) / elapsed
	}
	r.progress(GeneInfoProgress{
		Stage:          r.stage,
		Message:        messageForProgressStage(r.stage, message),
		CurrentBytes:   r.current,
		TotalBytes:     r.total,
		BytesPerSecond: speed,
		Records:        r.records,
		Workers:        r.workers,
		Done:           done,
	})
}

func messageForProgressStage(stage string, message string) string {
	message = strings.TrimSpace(message)
	if message != "" {
		return message
	}
	switch stage {
	case "download":
		return "Downloading prebuilt symbol name database..."
	case "extract":
		return "Decompressing and writing symbol name database..."
	case "build":
		return "Building symbol name database..."
	default:
		return "Preparing symbol name database..."
	}
}

type progressWriter struct {
	writer   io.Writer
	reporter *geneInfoProgressReporter
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.reporter.addBytes(n)
	}
	return n, err
}

type progressReader struct {
	reader   io.Reader
	reporter *geneInfoProgressReporter
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.reporter.addBytes(n)
	}
	return n, err
}

func encodeGeneRecord(record geneRecord) []byte {
	fields := [...]string{record.TaxID, record.GeneID, record.Symbol, record.LocusTag, record.Synonyms}
	size := 1
	for _, field := range fields {
		size += binary.MaxVarintLen64 + len(field)
	}
	out := make([]byte, 0, size)
	out = append(out, 2)
	for _, field := range fields {
		out = binary.AppendUvarint(out, uint64(len(field)))
		out = append(out, field...)
	}
	return out
}

func decodeGeneRecord(data []byte) (geneRecord, bool) {
	if len(data) == 0 || data[0] != 2 {
		return geneRecord{}, false
	}
	data = data[1:]
	fields := [5]string{}
	for i := range fields {
		length, n := binary.Uvarint(data)
		if n <= 0 || length > uint64(len(data[n:])) {
			return geneRecord{}, false
		}
		start := n
		end := start + int(length)
		fields[i] = string(data[start:end])
		data = data[end:]
	}
	return geneRecord{
		TaxID:    fields[0],
		GeneID:   fields[1],
		Symbol:   fields[2],
		LocusTag: fields[3],
		Synonyms: fields[4],
	}, true
}

func parseGeneInfoLine(line string) (geneRecord, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) < 16 {
		return geneRecord{}, false
	}
	return geneRecord{
		TaxID:              cleanGeneInfoValue(fields[0]),
		GeneID:             cleanGeneInfoValue(fields[1]),
		Symbol:             cleanGeneInfoValue(fields[2]),
		LocusTag:           cleanGeneInfoValue(fields[3]),
		Synonyms:           cleanGeneInfoValue(fields[4]),
		DBXrefs:            cleanGeneInfoValue(fields[5]),
		Chromosome:         cleanGeneInfoValue(fields[6]),
		MapLocation:        cleanGeneInfoValue(fields[7]),
		Description:        cleanGeneInfoValue(fields[8]),
		TypeOfGene:         cleanGeneInfoValue(fields[9]),
		SymbolAuthority:    cleanGeneInfoValue(fields[10]),
		FullNameAuthority:  cleanGeneInfoValue(fields[11]),
		NomenclatureStatus: cleanGeneInfoValue(fields[12]),
		OtherDesignations:  cleanGeneInfoValue(fields[13]),
		ModificationDate:   cleanGeneInfoValue(fields[14]),
		FeatureType:        cleanGeneInfoValue(fields[15]),
	}, true
}

type geneInfoTerm struct {
	Key    string
	Raw    string
	Weight int
	TaxID  string
}

func (r geneRecord) indexTerms() []geneInfoTerm {
	var out []geneInfoTerm
	add := func(weight int, values ...string) {
		for _, value := range values {
			out = append(out, geneInfoIndexTerms(value, weight, r.TaxID)...)
		}
	}
	add(100, r.Symbol, r.SymbolAuthority)
	add(92, r.GeneID)
	add(88, r.LocusTag)
	add(82, splitGeneInfoList(r.Synonyms)...)
	add(72, splitGeneInfoList(r.DBXrefs)...)
	add(62, splitGeneInfoList(r.OtherDesignations)...)
	add(45, r.FullNameAuthority, r.Description)
	add(25, r.TypeOfGene, r.FeatureType, r.Chromosome, r.MapLocation)
	return compactGeneInfoTerms(out)
}

func (r AliasRankRequest) geneInfoTerms() []geneInfoTerm {
	var out []geneInfoTerm
	taxID := strings.TrimSpace(r.TaxID)
	add := func(weight int, values ...string) {
		for _, value := range values {
			out = append(out, geneInfoIndexTerms(value, weight, taxID)...)
		}
	}
	add(100, r.Aliases...)
	add(98, r.Symbol, r.SymbolAuthority)
	add(95, r.SearchTerm)
	add(92, r.GeneID)
	add(90, r.ProteinID, r.TranscriptID, r.SequenceID)
	add(86, r.LocusTag)
	add(80, r.Synonyms...)
	add(74, r.DBXrefs...)
	add(64, r.OtherDesignations...)
	add(44, r.FullNameAuthority, r.Description)
	add(24, r.TypeOfGene, r.FeatureType, r.Chromosome, r.MapLocation)
	return compactGeneInfoTerms(out)
}

func geneInfoIndexTerms(value string, weight int, taxID string) []geneInfoTerm {
	value = cleanGeneInfoValue(value)
	if value == "" {
		return nil
	}
	values := []string{value}
	values = append(values, splitGeneInfoList(value)...)
	for _, token := range symbolTokenRx.FindAllString(value, -1) {
		values = append(values, token)
	}
	out := make([]geneInfoTerm, 0, len(values))
	for _, candidate := range values {
		key := normalizeGeneInfoKey(candidate)
		if key == "" {
			continue
		}
		if _, stop := geneInfoStopKeys[key]; stop {
			continue
		}
		out = append(out, geneInfoTerm{Key: key, Raw: strings.TrimSpace(candidate), Weight: weight, TaxID: taxID})
	}
	return out
}

func compactGeneInfoTerms(values []geneInfoTerm) []geneInfoTerm {
	best := make(map[string]geneInfoTerm, len(values))
	for _, value := range values {
		if value.Key == "" {
			continue
		}
		current, ok := best[value.Key]
		if !ok || value.Weight > current.Weight {
			best[value.Key] = value
		}
	}
	out := make([]geneInfoTerm, 0, len(best))
	for _, value := range best {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func putIndexTerm(bucket *bolt.Bucket, key string, id uint64, weight int) error {
	if key == "" {
		return nil
	}
	if weight > 255 {
		weight = 255
	}
	if weight < 0 {
		weight = 0
	}
	indexKey := make([]byte, 0, len(key)+9)
	indexKey = append(indexKey, key...)
	indexKey = append(indexKey, 0)
	indexKey = append(indexKey, u64key(id)...)
	return bucket.Put(indexKey, []byte{byte(weight)})
}

func splitGeneInfoList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '|' || r == ';' || r == ',' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanGeneInfoValue(part)
		if part != "" {
			out = append(out, part)
			if i := strings.LastIndex(part, ":"); i >= 0 && i+1 < len(part) {
				out = append(out, strings.TrimSpace(part[i+1:]))
			}
		}
	}
	return out
}

func (r geneRecord) symbolNameCandidates() []string {
	values := make([]string, 0, 1+strings.Count(r.Synonyms, "|")+strings.Count(r.Synonyms, ";")+strings.Count(r.Synonyms, ","))
	values = append(values, r.Symbol)
	values = append(values, splitGeneInfoList(r.Synonyms)...)
	return uniqueStrings(values)
}

func (r geneRecord) hasSymbolNameCandidateEqualTo(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, candidate := range r.symbolNameCandidates() {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

func symbolNameCandidateScore(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1000
	}
	if isPrimarySymbolNameCandidate(value) {
		return 80 + AliasPreferenceScore(value) + QueryAliasPrimarySymbolBonus(value)
	}
	return -80 + AliasPreferenceScore(value)
}

func cleanGeneInfoValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}

func normalizeGeneInfoKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, `"'()[]{}<>`)
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func u64key(id uint64) []byte {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], id)
	return key[:]
}

func sameCleanPath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func firstNonEmptyGeneInfoText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
