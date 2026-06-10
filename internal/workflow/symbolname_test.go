package workflow

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
)

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

	var gz bytes.Buffer
	writer := gzip.NewWriter(&gz)
	_, _ = writer.Write([]byte(workflowTestGeneInfo()))
	if err := writer.Close(); err != nil {
		return 1
	}
	payload := gz.Bytes()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dbPath := filepath.Join(dir, labelname.DefaultGeneInfoPGD)
	lastModified := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if err := labelname.DownloadAndBuildGeneInfoDatabase(context.Background(), dbPath, labelname.GeneInfoMetadata{
		URL:             server.URL,
		LastModified:    lastModified,
		LastModifiedRaw: lastModified.Format(http.TimeFormat),
		ContentLength:   int64(len(payload)),
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
