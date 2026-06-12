package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
	"github.com/klauspost/compress/zstd"
	bolt "go.etcd.io/bbolt"
)

var workflowSymbolTokenRx = regexp.MustCompile(`[A-Za-z][A-Za-z0-9'._-]{1,31}`)

func TestMain(m *testing.M) {
	code := runWorkflowTestsWithSymbolNameDB(m)
	os.Exit(code)
}

func runWorkflowTestsWithSymbolNameDB(m *testing.M) int {
	dir, err := os.MkdirTemp("", "phgo-workflow-symbolname-*")
	if err != nil {
		return 1
	}
	defer os.RemoveAll(dir)

	sourceDB := filepath.Join(dir, "source-symbolname.pgd")
	if err := writeWorkflowTestSymbolNameDB(sourceDB, workflowTestGeneInfo()); err != nil {
		return 1
	}
	dbData, err := os.ReadFile(sourceDB)
	if err != nil {
		return 1
	}
	var archive bytes.Buffer
	writer, err := zstd.NewWriter(&archive, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return 1
	}
	if _, err := writer.Write(dbData); err != nil {
		return 1
	}
	if err := writer.Close(); err != nil {
		return 1
	}
	payload := archive.Bytes()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zstd")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dbPath := filepath.Join(dir, labelname.DefaultGeneInfoPGD)
	if err := labelname.DownloadPrebuiltGeneInfoDatabase(context.Background(), dbPath, labelname.PrebuiltGeneInfoManifest{
		SchemaVersion:      "3",
		URL:                server.URL + "/symbolname.pgd.zst",
		SHA256:             fmt.Sprintf("%x", sha256.Sum256(dbData)),
		RecordCount:        13,
		SourceURL:          "https://example.test/GENE_INFO/",
		SourceLastModified: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat),
		ContentLength:      int64(len(payload)),
	}, labelname.DownloadOptions{Workers: 1}); err != nil {
		return 1
	}
	labelname.SetDefaultGeneInfoDatabasePath(dbPath)
	return m.Run()
}

func workflowTestGeneInfo() string {
	return strings.Join([]string{
		"#tax_id\tGeneID\tSymbol\tLocusTag\tSynonyms\tdbXrefs\tchromosome\tmap_location\tdescription\ttype_of_gene\tSymbol_from_nomenclature_authority\tFull_name_from_nomenclature_authority\tNomenclature_status\tOther_designations\tModification_date\tFeature_type",
		"3702\t838863\tVND6\t-\tANAC101|AtVND6|HeaderName\tTAIR:AT5G62380|AT5G62380|AT5G62380.1\t5\t-\tvascular NAC domain protein\tprotein-coding\tVND6\tvascular-related NAC-domain 6\tO\tNAC domain protein 101\t20260610\t-",
		"3702\t100001\tANAC001\t-\tNAC001|OldName\tTAIR:AT1G01010|AT1G01010|AT1G01010.1\t1\t-\tNAC domain protein\tprotein-coding\tANAC001\tNAC domain protein 1\tO\t-\t20260610\t-",
		"3702\t100002\tPAL1\t-\tATPAL1|AT2G37040|AT2G37040.1|AT5G13930|AT5G13930.1\tTAIR:AT2G37040|TAIR:AT5G13930\t2\t-\tphenylalanine ammonia-lyase 1\tprotein-coding\tPAL1\tphenylalanine ammonia-lyase 1\tO\t-\t20260610\t-",
		"3702\t100003\tATPAL4\t-\tPAL4|AT3G10340|AT3G10340.1|PAC:19660032\tTAIR:AT3G10340\t3\t-\tphenylalanine ammonia-lyase 4\tprotein-coding\tATPAL4\tphenylalanine ammonia-lyase 4\tO\t-\t20260610\t-",
		"3702\t100004\tF5H1\t-\tCYP84A1|FAH1\tTAIR:AT4G36220\t4\t-\tferulate 5-hydroxylase\tprotein-coding\tF5H1\tferulate 5-hydroxylase\tO\t-\t20260610\t-",
		"3702\t100005\tCYP98A3\t-\tREF8|C3'H\tTAIR:AT2G40890\t2\t-\tcoumaroyl shikimate 3-hydroxylase\tprotein-coding\tCYP98A3\tcoumaroyl shikimate 3-hydroxylase\tO\t-\t20260610\t-",
		"3702\t100006\tCYP73A5\t-\tC4H|REF3|AT2G30490|AT2G30490.1|Sp9509d020g000340_T001\tTAIR:AT2G30490\t2\t-\tcinnamate 4-hydroxylase\tprotein-coding\tCYP73A5\tcinnamate 4-hydroxylase\tO\t-\t20260610\t-",
		"3702\t100007\tAUTO1\t-\tK12345|AT2G00000|AT2G00000.1\tTAIR:AT2G00000\t2\t-\tmade up protein\tprotein-coding\tAUTO1\tmade up protein\tO\t-\t20260610\t-",
		"3702\t100008\tVND7\t-\tAT1G71930|AT1G71930.1\tTAIR:AT1G71930\t1\t-\tvascular NAC domain protein 7\tprotein-coding\tVND7\tvascular-related NAC-domain 7\tO\t-\t20260610\t-",
		"3702\t100009\tSOMETHINGELSE1\t-\t-\t-\t1\t-\ttest symbol\tprotein-coding\tSOMETHINGELSE1\ttest symbol\tO\t-\t20260610\t-",
		"3702\t100010\tLAC2\t-\tTT10|AT2G29130|AT2G29130.1\tTAIR:AT2G29130\t2\t-\tlaccase 2\tprotein-coding\tLAC2\tlaccase 2\tO\t-\t20260610\t-",
		"3702\t100011\tTT10\t-\tLAC2|AT2G29130|AT2G29130.1\tTAIR:AT2G29130\t2\t-\ttransparent testa 10\tprotein-coding\tTT10\ttransparent testa 10\tO\t-\t20260610\t-",
		"3702\t100012\t4CLL4\t-\tOs03g0132000|LOC_Os03g04000|OsJ_09299|Sp9509d008g014760_T001\t-\t3\t-\t4-coumarate CoA ligase-like 4\tprotein-coding\t4CLL4\t4-coumarate CoA ligase-like 4\tO\t-\t20260610\t-",
	}, "\n") + "\n"
}

func writeWorkflowTestSymbolNameDB(path string, content string) error {
	db, err := bolt.Open(path, 0o644, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{"meta", "records", "index"} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	var count uint64
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 16 {
			continue
		}
		count++
		record := workflowTestGeneRecord{
			id:       count,
			taxID:    cleanWorkflowGeneInfoValue(fields[0]),
			geneID:   cleanWorkflowGeneInfoValue(fields[1]),
			symbol:   cleanWorkflowGeneInfoValue(fields[2]),
			locusTag: cleanWorkflowGeneInfoValue(fields[3]),
			synonyms: cleanWorkflowGeneInfoValue(fields[4]),
			values: []string{
				fields[2], fields[10], fields[1], fields[3], fields[4], fields[5],
				fields[13], fields[11], fields[8], fields[9], fields[15], fields[6], fields[7],
			},
		}
		if err := db.Update(func(tx *bolt.Tx) error {
			if err := tx.Bucket([]byte("records")).Put(u64WorkflowKey(record.id), encodeWorkflowGeneRecord(record)); err != nil {
				return err
			}
			for _, term := range workflowGeneInfoTerms(record) {
				if err := putWorkflowIndexTerm(tx.Bucket([]byte("index")), term.key, record.id, term.weight); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		values := map[string]string{
			"schema_version": "3",
			"url":            "https://example.test/GENE_INFO/",
			"last_modified":  time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat),
			"downloaded_at":  time.Now().UTC().Format(time.RFC3339Nano),
			"content_length": strconv.Itoa(len(content)),
			"record_count":   fmt.Sprintf("%d", count),
		}
		for key, value := range values {
			if err := meta.Put([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	})
}

type workflowTestGeneRecord struct {
	id       uint64
	taxID    string
	geneID   string
	symbol   string
	locusTag string
	synonyms string
	values   []string
}

type workflowGeneTerm struct {
	key    string
	weight int
}

func encodeWorkflowGeneRecord(record workflowTestGeneRecord) []byte {
	fields := [...]string{record.taxID, record.geneID, record.symbol, record.locusTag, record.synonyms}
	out := []byte{2}
	for _, field := range fields {
		out = binary.AppendUvarint(out, uint64(len(field)))
		out = append(out, field...)
	}
	return out
}

func workflowGeneInfoTerms(record workflowTestGeneRecord) []workflowGeneTerm {
	weights := []int{100, 100, 92, 88, 82, 72, 62, 45, 45, 25, 25, 25, 25}
	best := make(map[string]int)
	for i, value := range record.values {
		weight := 25
		if i < len(weights) {
			weight = weights[i]
		}
		for _, candidate := range workflowGeneInfoCandidates(value) {
			key := normalizeWorkflowGeneInfoKey(candidate)
			if key == "" {
				continue
			}
			if current, ok := best[key]; !ok || weight > current {
				best[key] = weight
			}
		}
	}
	out := make([]workflowGeneTerm, 0, len(best))
	for key, weight := range best {
		out = append(out, workflowGeneTerm{key: key, weight: weight})
	}
	return out
}

func workflowGeneInfoCandidates(value string) []string {
	value = cleanWorkflowGeneInfoValue(value)
	if value == "" {
		return nil
	}
	out := []string{value}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '|' || r == ';' || r == ',' || r == '\t' || r == '\n' || r == '\r'
	}) {
		part = cleanWorkflowGeneInfoValue(part)
		if part != "" {
			out = append(out, part)
			if i := strings.LastIndex(part, ":"); i >= 0 && i+1 < len(part) {
				out = append(out, strings.TrimSpace(part[i+1:]))
			}
		}
	}
	out = append(out, workflowSymbolTokenRx.FindAllString(value, -1)...)
	return out
}

func putWorkflowIndexTerm(bucket *bolt.Bucket, key string, id uint64, weight int) error {
	if weight > 255 {
		weight = 255
	}
	indexKey := append([]byte(key), 0)
	indexKey = append(indexKey, u64WorkflowKey(id)...)
	return bucket.Put(indexKey, []byte{byte(weight)})
}

func u64WorkflowKey(id uint64) []byte {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], id)
	return key[:]
}

func cleanWorkflowGeneInfoValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}

func normalizeWorkflowGeneInfoKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, `"'()[]{}<>`)
	value = strings.Join(strings.Fields(value), " ")
	return value
}
