package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const unityReferencesWireLimit = 16 << 10

// cmdUnityReferences is an explicit receipt reader. Producer run directories
// retain history; this command creates no family table or current-fact cache.
// Even an unchanged receipt must pass the public client's current input and
// active-generation checks on every invocation.
func cmdUnityReferences(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("unity-references", stderr)
	service := fs.String("service", "", "registered service identity (required)")
	project := fs.String("project", "", "service-relative Unity project (required)")
	family := fs.String("family", "", "explicit receipt family identity (required)")
	receipt := fs.String("receipt", "", "service-relative immutable producer receipt JSON (required)")
	active := fs.String("active", "", "service-relative current family active marker JSON (required)")
	editorVersion := fs.String("editor-version", "", "trusted exact Editor version, never accepted from the receipt (required)")
	editorSHA := fs.String("editor-sha256", "", "trusted exact Editor executable SHA256 (required)")
	target := fs.String("target", "", "trusted selected native build target (required)")
	guid := fs.String("guid", "", "exact persistent target/source GUID (required)")
	fileID := fs.String("file-id", "", "canonical signed64 decimal fileID text, without rounding (required)")
	direction := fs.String("direction", "inbound", "inbound or outbound; one hop only")
	limit := fs.Int("limit", 25, "maximum groups, 0 selects 25, at most 100; complete response at most 16KiB")
	jsonOut := fs.Bool("json", false, "emit bounded compact JSON instead of text")
	storePath := fs.String("store", defaultStorePath(), "existing registered-service store, mounted read-only")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unity-references accepts named flags only")
		return 1
	}
	for _, required := range []struct{ name, value string }{
		{"service", *service}, {"project", *project}, {"family", *family},
		{"receipt", *receipt}, {"active", *active}, {"editor-version", *editorVersion},
		{"editor-sha256", *editorSHA}, {"target", *target}, {"guid", *guid}, {"file-id", *fileID},
	} {
		if required.value == "" {
			fmt.Fprintf(stderr, "error: unity-references requires --%s\n", required.name)
			return 1
		}
	}
	if *limit < 0 || *limit > 100 || (*direction != "inbound" && *direction != "outbound") {
		fmt.Fprintln(stderr, "error: unity-references requires inbound/outbound and limit 0..100")
		return 1
	}
	mounted, err := store.MountReadOnly(*storePath, store.LevelL0)
	if err != nil {
		fmt.Fprintf(stderr, "error: unity-references read-only store: %v\n", err)
		return 1
	}
	c := client.New(nil, mounted)
	defer func() { _ = c.Close() }() // Client owns the one mount and pipeline.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := c.UnityAssetReferences(ctx, *service, client.UnityAssetReferenceOptions{
		Project: *project, FamilyID: *family, ReceiptPath: *receipt, ActivePath: *active,
		ExpectedEditorVersion: *editorVersion, ExpectedEditorSHA256: *editorSHA, ExpectedBuildTarget: *target,
	}, client.AssetReferenceQuery{Object: client.AssetObjectID{GUID: *guid, FileID: *fileID}, Direction: *direction, Limit: *limit})
	if err != nil {
		fmt.Fprintf(stderr, "error: unity-references: %v\n", err)
		return 1
	}
	if err := renderUnityReferences(stdout, result, *jsonOut); err != nil {
		fmt.Fprintf(stderr, "error: unity-references output: %v\n", err)
		return 1
	}
	return 0
}

// Buffer the whole projection before writing: a refusal never prints a partial
// positive response. Compact JSON has no extra newline beyond its byte budget.
func renderUnityReferences(w io.Writer, result client.AssetReferenceResult, jsonOut bool) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if len(encoded) > unityReferencesWireLimit {
		return fmt.Errorf("complete result exceeds 16KiB")
	}
	if jsonOut {
		_, err = w.Write(encoded)
		return err
	}
	var text bytes.Buffer
	fmt.Fprintf(&text, "Unity references: %s; service=%q project=%q family=%q\n", result.Status, result.Service, result.Project, result.FamilyID)
	fmt.Fprintf(&text, "%s %s#%s: %d/%d groups; %d elided\n", result.Direction, result.Object.GUID, result.Object.FileID, len(result.References), result.Total, result.Truncated)
	for _, reference := range result.References {
		fmt.Fprintf(&text, "%q [%s#%s] -> %q [%s#%s]; %d matched occurrences (locations/properties unpaired)\n", reference.SourcePath, reference.Source.GUID, reference.Source.FileID, reference.TargetPath, reference.Target.GUID, reference.Target.FileID, reference.MatchedOccurrences)
	}
	fmt.Fprintf(&text, "Cohort: matched=%d admitted=%d package-unbound=%d native-only=%d raw-only=%d\n", result.Counts.Matched, result.Counts.AdmittedMatched, result.Counts.NativeObservedUnbound, result.Counts.NativeOnly, result.Counts.RawOnly)
	fmt.Fprintf(&text, "Receipt=%s source=%s context=%s\n", result.ReceiptSHA256, result.SourceSHA256, result.ContextSHA256)
	for _, limitation := range result.Limitations {
		fmt.Fprintf(&text, "Partial: %s\n", limitation)
	}
	if text.Len() > unityReferencesWireLimit {
		return fmt.Errorf("complete text result exceeds 16KiB; use --json or a lower --limit")
	}
	_, err = w.Write(text.Bytes())
	return err
}

type unityReferencesRunner struct{}

func (unityReferencesRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdUnityReferences(args, stdout, stderr)
}
func init() {
	declareVerb(verb{Name: "unity-references", Group: "index", Order: 95, Run: unityReferencesRunner{}, Summary: "Read one-hop persistent Unity asset references from an explicit current partial receipt; registered service root, read-only store, no code-call evidence"})
}
