package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/phylo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	server := phylo.NewViewerServer("127.0.0.1:0")
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}
	const session = "msaexpor-smoke"
	payload := phylo.ViewerPayload{
		SchemaVersion: 1,
		SessionID:     session,
		Title:         "1.1 Browser Smoke",
		UpdatedAt:     time.Now(),
		AlignedFASTA:  ">PHGOT000001\nMSTNPKPQRK---AAALVVVGGGTTT\n>PHGOT000002\nMS-NPKPQRKGGGAAALV-VGGGTTT\n>PHGOT000003\nMSTNPKPQ-KGGGAAA---VGGGTTT\n",
		Metadata: phylo.Metadata{
			SchemaVersion: 1,
			SequenceKind:  phylo.SequenceProtein,
			Records: []phylo.InputRecord{
				{TaxonID: "PHGOT000001", DisplayName: "Alpha", BaseDisplayName: "Alpha", DisplayPrefix: "[1,1]", CanvasItemIndex: 1, CanvasRow: 1},
				{TaxonID: "PHGOT000002", DisplayName: "Beta", BaseDisplayName: "Beta", DisplayPrefix: "[1,2]", CanvasItemIndex: 1, CanvasRow: 2},
				{TaxonID: "PHGOT000003", DisplayName: "Gamma", BaseDisplayName: "Gamma", DisplayPrefix: "[1,3]", CanvasItemIndex: 1, CanvasRow: 3},
			},
		},
	}
	server.SetMSAPayload(session, payload)
	fmt.Println(server.URL() + "/sessions/" + session + "/msa")
	<-ctx.Done()
}
