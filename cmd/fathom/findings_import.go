package main

// findings_import.go — `fathom findings import`, the CLI half of the
// findings-loop's governed external import (docs/vision/30-findings-loop.md
// §3.3, plan FL-10). CLI and pkg/client share ONE code path
// (Client.ImportFindings, pkg/client/findings.go) — this file is flag
// parsing, config-default resolution, and receipt rendering only.
//
// v0 is store-direct only (--store required, mirroring `fathom claim` /
// `fathom compile`'s own discipline): a governed write is a deliberate,
// single operation best made directly against the store an operator can
// inspect, not routed through the read-serving daemon.

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/findings/external"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdFindingsImport implements:
//
//	fathom findings import --store PATH --service S --subject ORG/REPO/COMPONENT --producer NAME --file PATH [--dry-run] [--json]
//
// Exit codes: 0 clean, 1 the receipt names any Rejected event, 2 usage/
// operational failure (bad flags, unreadable file, unopenable store, a
// batch schema failure, or a refused import — e.g. no subject provisioned).
func cmdFindingsImport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("findings import", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	subject := fs.String("subject", "", "provisioned ORG/REPO/COMPONENT subject (default: [findings] subject in .fathom/config.toml)")
	producer := fs.String("producer", "", "restrict import to this producer id (default: any catalogued producer)")
	file := fs.String("file", "", "path to the review kit's projection JSON (required)")
	dryRun := fs.Bool("dry-run", false, "compute the receipt without writing anything")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *storePath == "" || *service == "" || *file == "" {
		fmt.Fprintln(stderr, "error: findings import requires --store, --service, and --file")
		fs.Usage()
		return 2
	}

	subj := *subject
	if subj == "" {
		if loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath()); err == nil {
			subj = loaded.Config.Findings.Subject
		}
		// A config load failure here is not fatal to the flag-resolution
		// step: an explicit --subject still works, and an empty subject
		// either way is Importer's own named refusal ("no [findings]
		// subject provisioned"), never a silent default.
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %s: %v\n", *file, err)
		return 2
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 2
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	receipt, err := c.ImportFindings(context.Background(), client.ImportFindingsRequest{
		Service: *service, Subject: subj, Producer: *producer, Projection: raw, DryRun: *dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if *jsonOut {
		jsonPrint(stdout, receipt)
	} else {
		renderFindingsImportReceipt(stdout, receipt)
	}
	if len(receipt.Rejected) > 0 {
		return 1
	}
	return 0
}

// renderFindingsImportReceipt is the text-mode render: one line per receipt
// count, then each rejection, then each cites skew (a finding imported with
// no cites link because its lines could not be read at its own revision).
func renderFindingsImportReceipt(w io.Writer, r external.Receipt) {
	fmt.Fprintf(w, "delivered:      %d\n", r.Delivered)
	fmt.Fprintf(w, "duplicate:      %d\n", r.Duplicate)
	fmt.Fprintf(w, "stale:          %d\n", r.Stale)
	fmt.Fprintf(w, "unclassified:   %d\n", r.Unclassified)
	fmt.Fprintf(w, "no_fingerprint: %d\n", r.NoFingerprint)
	fmt.Fprintf(w, "truncated:      %v\n", r.Truncated)
	fmt.Fprintf(w, "dry_run:        %v\n", r.DryRun)
	fmt.Fprintf(w, "rejected:       %d\n", len(r.Rejected))
	for _, rej := range r.Rejected {
		fmt.Fprintf(w, "  [%d] %s: %s\n", rej.Index, rej.FindingID, rej.Reason)
	}
	if len(r.CitesSkew) > 0 {
		fmt.Fprintf(w, "cites_skew:     %d (no cites link: lines not readable at the event's revision)\n", len(r.CitesSkew))
		for _, sk := range r.CitesSkew {
			fmt.Fprintf(w, "  [%d] %s: %s at %q: %s\n", sk.Index, sk.FindingID, sk.Path, sk.Revision, sk.Reason)
		}
	}
	for _, cur := range r.Cursors {
		fmt.Fprintf(w, "cursor: %s @ %d\n", cur.Producer, cur.LatestSourceVersion)
	}
}
