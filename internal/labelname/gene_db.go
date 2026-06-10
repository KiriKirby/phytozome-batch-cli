package labelname

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
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
	bolt "go.etcd.io/bbolt"
)

const (
	GeneInfoURL          = "https://ftp.ncbi.nlm.nih.gov/gene/DATA/gene_info.gz"
	DefaultGeneInfoPGD   = "symbolname.pgd"
	geneDBSchemaVersion  = "2"
	geneDBBucketMeta     = "meta"
	geneDBBucketRecords  = "records"
	geneDBBucketIndex    = "index"
	geneDBMaxTokenHits   = 2048
	geneDBBatchRecordCap = 10000
	geneDBBuildQueueCap  = 4096
)

type GeneInfoMetadata struct {
	URL             string
	LastModified    time.Time
	LastModifiedRaw string
	ContentLength   int64
	AcceptRanges    bool
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
	path string
	db   *bolt.DB
}

type preparedGeneRecord struct {
	id      uint64
	encoded []byte
	terms   []geneInfoTerm
}

var (
	geneDBDefaultMu   sync.Mutex
	geneDBDefaultPath string
	geneDBDefault     *geneDB
	geneDBInstallMu   sync.Mutex
	geneInfoHTTP      = netconfig.DefaultHTTPClient()
	symbolTokenRx     = regexp.MustCompile(`[A-Za-z][A-Za-z0-9'._-]{1,31}`)
	geneInfoStopKeys  = map[string]struct{}{
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

func DefaultBuildWorkers() int {
	cpu := runtime.GOMAXPROCS(0)
	if cpu < 1 {
		cpu = runtime.NumCPU()
	}
	if cpu < 1 {
		cpu = 1
	}
	workers := cpu * 8
	if workers < 8 {
		workers = 8
	}
	if workers > 64 {
		workers = 64
	}
	return netconfig.ConfiguredInt("PHGO_SYMBOL_NAME_BUILD_WORKERS", workers)
}

func FetchRemoteGeneInfoMetadata(ctx context.Context) (GeneInfoMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, GeneInfoURL, nil)
	if err != nil {
		return GeneInfoMetadata{}, fmt.Errorf("build NCBI Gene HEAD request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go-symbolname")
	resp, err := geneInfoHTTP.Do(req)
	if err != nil {
		return GeneInfoMetadata{}, fmt.Errorf("query NCBI Gene metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GeneInfoMetadata{}, fmt.Errorf("NCBI Gene metadata returned %s", resp.Status)
	}
	lastRaw := strings.TrimSpace(resp.Header.Get("Last-Modified"))
	lastModified, _ := http.ParseTime(lastRaw)
	contentLength := resp.ContentLength
	if contentLength <= 0 {
		contentLength, _ = strconv.ParseInt(strings.TrimSpace(resp.Header.Get("Content-Length")), 10, 64)
	}
	return GeneInfoMetadata{
		URL:             GeneInfoURL,
		LastModified:    lastModified,
		LastModifiedRaw: lastRaw,
		ContentLength:   contentLength,
		AcceptRanges:    strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes"),
	}, nil
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

func DownloadAndBuildGeneInfoDatabase(ctx context.Context, dest string, remote GeneInfoMetadata, options DownloadOptions) error {
	dest = filepath.Clean(strings.TrimSpace(dest))
	if dest == "" {
		return fmt.Errorf("symbol name database path is empty")
	}
	if strings.TrimSpace(remote.URL) == "" {
		remote.URL = GeneInfoURL
	}
	options.emitProgress(GeneInfoProgress{
		Stage:      "prepare",
		Message:    "Preparing symbol name database download...",
		TotalBytes: remote.ContentLength,
	})
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create symbol name database directory: %w", err)
	}
	tempDir := filepath.Dir(dest)
	gzFile, err := os.CreateTemp(tempDir, "gene_info-*.gz")
	if err != nil {
		return fmt.Errorf("create temporary NCBI Gene download: %w", err)
	}
	gzPath := gzFile.Name()
	_ = gzFile.Close()
	defer os.Remove(gzPath)
	if err := downloadGeneInfoGZ(ctx, remote, gzPath, options); err != nil {
		return err
	}
	tmpDB := dest + ".tmp-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := buildGeneInfoDatabaseFromGZ(gzPath, tmpDB, remote, options); err != nil {
		_ = os.Remove(tmpDB)
		return err
	}
	if err := os.Rename(tmpDB, dest); err != nil {
		_ = os.Remove(tmpDB)
		return fmt.Errorf("install symbol name database %s: %w", dest, err)
	}
	resetDefaultGeneDB()
	options.emitProgress(GeneInfoProgress{
		Stage:        "complete",
		Message:      "Symbol name database is ready.",
		CurrentBytes: remote.ContentLength,
		TotalBytes:   remote.ContentLength,
		Done:         true,
	})
	return nil
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
		progress(GeneInfoProgress{Stage: "metadata", Message: "Checking NCBI Gene symbol name library metadata..."})
	}
	remote, err := FetchRemoteGeneInfoMetadata(ctx)
	if err != nil {
		return fmt.Errorf("prepare symbol name database install: %w", err)
	}
	if progress != nil {
		progress(GeneInfoProgress{Stage: "download", Message: fmt.Sprintf("Downloading NCBI Gene symbol name library (%s)...", formatBytes(remote.ContentLength)), TotalBytes: remote.ContentLength})
	}
	if err := DownloadAndBuildGeneInfoDatabase(ctx, path, remote, DownloadOptions{
		Workers:  DefaultDownloadWorkers(),
		Progress: progress,
	}); err != nil {
		return err
	}
	if progress != nil {
		progress(GeneInfoProgress{Stage: "complete", Message: "Symbol name database is ready.", TotalBytes: remote.ContentLength, CurrentBytes: remote.ContentLength, Done: true})
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
	queryTerms := request.geneInfoTerms()
	if len(queryTerms) == 0 {
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
			bestScore := -1
			for _, term := range hits {
				score := term.Weight
				symbol := strings.TrimSpace(record.Symbol)
				if symbol == "" || symbol == "-" {
					continue
				}
				if strings.EqualFold(term.Raw, symbol) {
					score += 80
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
			symbol := strings.TrimSpace(record.Symbol)
			if symbol == "" || symbol == "-" {
				continue
			}
			key := normalizeAliasKey(symbol)
			current := scores[key]
			if bestScore > current.Score || (bestScore == current.Score && len(symbol) < len(current.Text)) {
				scores[key] = rankedAlias{Text: symbol, Score: bestScore, Family: symbolFamily(symbol)}
			}
		}
		return nil
	})
	if len(scores) == 0 {
		return nil, true
	}
	out := make([]rankedAlias, 0, len(scores))
	for _, item := range scores {
		out = append(out, item)
	}
	return out, true
}

func downloadGeneInfoGZ(ctx context.Context, remote GeneInfoMetadata, dest string, options DownloadOptions) error {
	options.emitProgress(GeneInfoProgress{
		Stage:      "download",
		Message:    "Starting single-stream NCBI Gene download...",
		TotalBytes: remote.ContentLength,
		Workers:    1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote.URL, nil)
	if err != nil {
		return fmt.Errorf("build NCBI Gene download request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go-symbolname")
	resp, err := geneInfoHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("download NCBI Gene gene_info.gz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download NCBI Gene gene_info.gz returned %s", resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create NCBI Gene download %s: %w", dest, err)
	}
	defer out.Close()
	reporter := newGeneInfoProgressReporter(options.Progress, "download", remote.ContentLength, 1)
	writer := &progressWriter{writer: out, reporter: reporter}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("write NCBI Gene download %s: %w", dest, err)
	}
	reporter.finish("Downloaded NCBI Gene gene_info.gz.")
	return nil
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
		return "Downloading NCBI Gene gene_info.gz..."
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

func buildGeneInfoDatabaseFromGZ(gzPath string, dbPath string, remote GeneInfoMetadata, options DownloadOptions) error {
	source, err := os.Open(gzPath)
	if err != nil {
		return fmt.Errorf("open NCBI Gene download %s: %w", gzPath, err)
	}
	defer source.Close()
	stat, _ := source.Stat()
	buildTotal := remote.ContentLength
	if buildTotal <= 0 && stat != nil {
		buildTotal = stat.Size()
	}
	workers := options.Workers
	if workers <= 0 {
		workers = DefaultBuildWorkers()
	}
	if maxWorkers := DefaultBuildWorkers(); workers > maxWorkers {
		workers = maxWorkers
	}
	if workers < 1 {
		workers = 1
	}
	reporter := newGeneInfoProgressReporter(options.Progress, "build", buildTotal, workers)
	hash := sha256.New()
	countedSource := &progressReader{reader: source, reporter: reporter}
	gzReader, err := kgzip.NewReader(io.TeeReader(countedSource, hash))
	if err != nil {
		return fmt.Errorf("open NCBI Gene gzip stream: %w", err)
	}
	defer gzReader.Close()
	db, err := bolt.Open(dbPath, 0o644, &bolt.Options{
		Timeout:        time.Second,
		NoFreelistSync: true,
		FreelistType:   bolt.FreelistMapType,
	})
	if err != nil {
		return fmt.Errorf("create symbol name database %s: %w", dbPath, err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{geneDBBucketMeta, geneDBBucketRecords, geneDBBucketIndex} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(gzReader)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	type buildLine struct {
		id   uint64
		text string
	}
	jobs := make(chan buildLine, geneDBBuildQueueCap)
	prepared := make(chan preparedGeneRecord, geneDBBuildQueueCap)
	done := make(chan struct{})
	defer close(done)
	var workerWG sync.WaitGroup
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for {
				var job buildLine
				var ok bool
				select {
				case <-done:
					return
				case job, ok = <-jobs:
				}
				if !ok {
					return
				}
				record, ok := parseGeneInfoLine(job.text)
				if !ok {
					continue
				}
				record.ID = job.id
				item := preparedGeneRecord{
					id:      record.ID,
					encoded: encodeGeneRecord(record),
					terms:   record.indexTerms(),
				}
				select {
				case prepared <- item:
				case <-done:
					return
				}
			}
		}()
	}
	go func() {
		workerWG.Wait()
		close(prepared)
	}()
	scanErrCh := make(chan error, 1)
	go func() {
		defer close(jobs)
		var lineID uint64
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			lineID++
			select {
			case jobs <- buildLine{id: lineID, text: line}:
			case <-done:
				scanErrCh <- nil
				return
			}
		}
		if err := scanner.Err(); err != nil {
			scanErrCh <- fmt.Errorf("read NCBI Gene gene_info.gz: %w", err)
			return
		}
		scanErrCh <- nil
	}()
	var count uint64
	batch := make([]preparedGeneRecord, 0, geneDBBatchRecordCap)
	for item := range prepared {
		count++
		batch = append(batch, item)
		if len(batch) >= geneDBBatchRecordCap {
			if err := flushPreparedGeneBatch(db, batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
		if count%10000 == 0 {
			reporter.setRecords(int64(count))
		}
	}
	if err := <-scanErrCh; err != nil {
		return err
	}
	if len(batch) > 0 {
		if err := flushPreparedGeneBatch(db, batch); err != nil {
			return err
		}
	}
	if err := db.Sync(); err != nil {
		return err
	}
	reporter.setRecords(int64(count))
	reporter.finish("Built symbol name database.")
	sum := fmt.Sprintf("%x", hash.Sum(nil))
	return db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(geneDBBucketMeta))
		values := map[string]string{
			"schema_version": geneDBSchemaVersion,
			"url":            remote.URL,
			"last_modified":  remote.LastModifiedRaw,
			"downloaded_at":  time.Now().UTC().Format(time.RFC3339Nano),
			"content_length": strconv.FormatInt(remote.ContentLength, 10),
			"record_count":   strconv.FormatUint(count, 10),
			"sha256":         sum,
		}
		for key, value := range values {
			if err := meta.Put([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	})
}

func encodeGeneRecord(record geneRecord) []byte {
	fields := [...]string{record.TaxID, record.GeneID, record.Symbol, record.LocusTag}
	size := 1
	for _, field := range fields {
		size += binary.MaxVarintLen64 + len(field)
	}
	out := make([]byte, 0, size)
	out = append(out, 1)
	for _, field := range fields {
		out = binary.AppendUvarint(out, uint64(len(field)))
		out = append(out, field...)
	}
	return out
}

type geneIndexEntry struct {
	key    string
	id     uint64
	weight int
}

func flushPreparedGeneBatch(db *bolt.DB, batch []preparedGeneRecord) error {
	if len(batch) == 0 {
		return nil
	}
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].id < batch[j].id
	})
	indexEntryCount := 0
	for _, item := range batch {
		indexEntryCount += len(item.terms)
	}
	entries := make([]geneIndexEntry, 0, indexEntryCount)
	for _, item := range batch {
		for _, term := range item.terms {
			if term.Key == "" {
				continue
			}
			entries = append(entries, geneIndexEntry{key: term.Key, id: item.id, weight: term.Weight})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].key != entries[j].key {
			return entries[i].key < entries[j].key
		}
		return entries[i].id < entries[j].id
	})
	return db.Update(func(tx *bolt.Tx) error {
		records := tx.Bucket([]byte(geneDBBucketRecords))
		index := tx.Bucket([]byte(geneDBBucketIndex))
		if records == nil || index == nil {
			return fmt.Errorf("symbol name database buckets are missing")
		}
		records.FillPercent = 0.95
		index.FillPercent = 0.95
		for _, item := range batch {
			if err := records.Put(u64key(item.id), item.encoded); err != nil {
				return err
			}
		}
		for _, entry := range entries {
			if err := putIndexTerm(index, entry.key, entry.id, entry.weight); err != nil {
				return err
			}
		}
		return nil
	})
}

func decodeGeneRecord(data []byte) (geneRecord, bool) {
	if len(data) == 0 || data[0] != 1 {
		return geneRecord{}, false
	}
	data = data[1:]
	fields := [4]string{}
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
