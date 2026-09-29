package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/internal/jsonwire"
	governedprosekit "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit"
)

const (
	inputVersion   = "kit-workflow/prose/v1"
	maxInputBytes  = 32 << 20
	maxJSONNesting = 64
	proseUsage     = "usage: kit-workflow prose check|export < request.json"
)

// input is deliberately self-contained. Sources and current are declarations
// supplied by the caller; localSources checks their internal consistency but
// cannot authenticate their origin, bytes, access rights, or principal.
type input struct {
	Version    string                            `json:"version"`
	Sources    []governedprosekit.SourceSnapshot `json:"sources"`
	Current    []currentSource                   `json:"current"`
	Request    governedprosekit.SourcedRequest   `json:"request"`
	Draft      governedprosekit.ProseDraft       `json:"draft"`
	Config     governedprosekit.SourcedConfig    `json:"config"`
	NonFactual []string                          `json:"non_factual"`
	Audience   string                            `json:"audience,omitempty"`
}

type currentSource struct {
	ID  string                     `json:"id"`
	Ref governedprosekit.SourceRef `json:"ref"`
}

type assertions struct {
	SourceBytes string `json:"source_bytes"`
	SourceLinks string `json:"source_links"`
	Access      string `json:"access"`
	Publication string `json:"publication"`
}

type output struct {
	Version    string                            `json:"version"`
	Operation  string                            `json:"operation"`
	Assertions assertions                        `json:"assertions"`
	Artifact   *governedprosekit.SourcedArtifact `json:"artifact,omitempty"`
	Export     *governedprosekit.ProseExport     `json:"export,omitempty"`
}

type errorOutput struct {
	Version string `json:"version"`
	Error   struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"error"`
}

// localAssertions describes the boundary precisely: this command verifies
// consistency of caller-provided data, not an external authority claim.
func localAssertions() assertions {
	return assertions{
		SourceBytes: "caller-supplied bytes were hashed and checked against declared immutable references; origin is not independently authenticated",
		SourceLinks: "declared current references and exact UTF-8 byte evidence ranges were checked locally",
		Access:      "caller-declared principal, access, acquisition, retention, and revocation assertions were checked for consistency only; no identity or source authorization occurred",
		Publication: "local bytes only; this result neither writes nor sends anything and is not authenticated publication or durable approval",
	}
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, func() time.Time { return time.Now().UTC() }))
}

// run is the testable command boundary. now is injected so receipts can be
// reproduced in tests; production supplies UTC wall time.
func run(ctx context.Context, args []string, in io.Reader, stdout, stderr io.Writer, now func() time.Time) int {
	if len(args) == 1 && isHelp(args[0]) {
		return writeHelp(stdout, stderr, proseUsage+"\n"+lessonUsage)
	}
	if len(args) > 0 && args[0] == "lesson" {
		return runLesson(ctx, args[1:], stdout, stderr, now)
	}
	if len(args) == 2 && args[0] == "prose" && isHelp(args[1]) {
		return writeHelp(stdout, stderr, proseUsage)
	}
	if len(args) == 3 && args[0] == "prose" && (args[1] == "check" || args[1] == "export") && isHelp(args[2]) {
		return writeHelp(stdout, stderr, "usage: kit-workflow prose "+args[1]+" < request.json")
	}
	if len(args) != 2 || args[0] != "prose" || (args[1] != "check" && args[1] != "export") {
		return writeFailure(stdout, stderr, 2, "usage", proseUsage+"\n"+lessonUsage)
	}
	if ctx == nil {
		return writeFailure(stdout, stderr, 1, "invalid", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return writeFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	request, err := decodeInput(in)
	if err != nil {
		return writeFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	if request.Version != inputVersion {
		return writeFailure(stdout, stderr, 1, "invalid", "unsupported or missing input version")
	}
	if args[1] == "check" && request.Audience != "" {
		return writeFailure(stdout, stderr, 1, "invalid", "audience is only valid for prose export")
	}
	if args[1] == "export" && request.Audience == "" {
		return writeFailure(stdout, stderr, 1, "invalid", "export audience is required")
	}
	sources, err := newLocalSources(request.Sources, request.Current)
	if err != nil {
		return writeFailure(stdout, stderr, 1, "evidence", err.Error())
	}
	pipeline, err := governedprosekit.NewSourcedPipeline(noModelClient{}, sources, governedprosekit.ExactSupportReviewer{NonFactual: append([]string(nil), request.NonFactual...)}, request.Config, now)
	if err != nil {
		return writeFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	artifact, err := pipeline.CheckDraft(ctx, request.Request, request.Draft)
	if err != nil {
		return writeFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	result := output{Version: inputVersion, Operation: "prose/check", Assertions: localAssertions(), Artifact: &artifact}
	if args[1] == "export" {
		exported, err := pipeline.Export(ctx, artifact, request.Audience, localOutputOnly)
		if err != nil {
			return writeFailure(stdout, stderr, 1, errorKind(err), err.Error())
		}
		result.Operation, result.Artifact, result.Export = "prose/export", nil, &exported
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintf(stderr, "kit-workflow: write result: %v\n", err)
		return 1
	}
	return 0
}

func isHelp(arg string) bool { return arg == "-h" || arg == "--help" }

func writeHelp(stdout, stderr io.Writer, usage string) int {
	if _, err := fmt.Fprintln(stdout, usage); err != nil {
		fmt.Fprintf(stderr, "kit-workflow: write help: %v\n", err)
		return 1
	}
	return 0
}

// localOutputOnly intentionally grants no publication authority. Export needs
// an explicit callback to make byte-returning behavior visible; this callback
// only permits returning the freshly checked result to this process's stdout.
func localOutputOnly(ctx context.Context, principal string, receipt governedprosekit.ProseReceipt, audience string) error {
	if strings.TrimSpace(principal) == "" || receipt.Integrity != "pass" || receipt.ArtifactDigest == "" || receipt.EvidenceDigest == "" || (audience != "public" && audience != "restricted") {
		return governedprosekit.ErrArtifact
	}
	return ctx.Err()
}

func writeFailure(stdout, stderr io.Writer, code int, kind, message string) int {
	result := errorOutput{Version: inputVersion}
	result.Error.Kind, result.Error.Message = kind, message
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintf(stderr, "kit-workflow: write error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "kit-workflow: %s\n", message)
	return code
}

func errorKind(err error) string {
	switch {
	case errors.Is(err, governedprosekit.ErrEvidence):
		return "evidence"
	case errors.Is(err, governedprosekit.ErrUnsupported):
		return "unsupported"
	case errors.Is(err, governedprosekit.ErrArtifact):
		return "artifact"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "invalid"
	}
}

func decodeInput(in io.Reader) (input, error) {
	if in == nil {
		return input{}, fmt.Errorf("input is required")
	}
	b, err := io.ReadAll(io.LimitReader(in, maxInputBytes+1))
	if err != nil {
		return input{}, fmt.Errorf("read input: %w", err)
	}
	if len(b) > maxInputBytes {
		return input{}, fmt.Errorf("input exceeds %d byte limit", maxInputBytes)
	}
	if !utf8.Valid(b) {
		return input{}, fmt.Errorf("input is not valid UTF-8")
	}
	if err := validateJSON(b, maxJSONNesting); err != nil {
		return input{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	var out input
	if err := decoder.Decode(&out); err != nil {
		return input{}, fmt.Errorf("invalid input: %w", err)
	}
	if decoder.More() {
		return input{}, fmt.Errorf("trailing JSON value")
	}
	return out, nil
}

// validateJSON rejects duplicate object keys at every nesting level, excessive
// nesting, malformed Unicode and non-single JSON values before typed decoding.
// encoding/json alone replaces unpaired surrogates and accepts duplicate keys
// with last-write-wins, which is unsuitable for policy input.
func validateJSON(b []byte, maxDepth int) error {
	if !jsonwire.ValidUnicode(b) {
		return fmt.Errorf("invalid input JSON: invalid Unicode")
	}
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.UseNumber()
	if err := validateJSONValue(decoder, 0, maxDepth); err != nil {
		return fmt.Errorf("invalid input JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}

func validateJSONValue(decoder *json.Decoder, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("JSON nesting exceeds %d", maxDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			folded := strings.ToLower(name)
			if _, duplicate := seen[folded]; duplicate {
				return fmt.Errorf("duplicate JSON key %q", name)
			}
			seen[folded] = struct{}{}
			if err := validateJSONValue(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := validateJSONValue(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

type localSources struct {
	items   map[governedprosekit.SourceRef]governedprosekit.SourceSnapshot
	current map[string]governedprosekit.SourceRef
}

func newLocalSources(snapshots []governedprosekit.SourceSnapshot, current []currentSource) (*localSources, error) {
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("at least one source snapshot is required")
	}
	s := &localSources{items: make(map[governedprosekit.SourceRef]governedprosekit.SourceSnapshot, len(snapshots)), current: make(map[string]governedprosekit.SourceRef, len(current))}
	for _, snapshot := range snapshots {
		if _, exists := s.items[snapshot.Ref]; exists {
			return nil, fmt.Errorf("duplicate source snapshot reference")
		}
		s.items[snapshot.Ref] = snapshot
	}
	for _, item := range current {
		if strings.TrimSpace(item.ID) == "" || item.Ref.ID != item.ID {
			return nil, fmt.Errorf("invalid current source declaration")
		}
		if _, exists := s.current[item.ID]; exists {
			return nil, fmt.Errorf("duplicate current source declaration")
		}
		s.current[item.ID] = item.Ref
	}
	return s, nil
}

func (s *localSources) Resolve(ctx context.Context, principal string, ref governedprosekit.SourceRef) (governedprosekit.SourceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return governedprosekit.SourceSnapshot{}, err
	}
	if strings.TrimSpace(principal) == "" {
		return governedprosekit.SourceSnapshot{}, governedprosekit.ErrEvidence
	}
	item, ok := s.items[ref]
	if !ok {
		return governedprosekit.SourceSnapshot{}, governedprosekit.ErrEvidence
	}
	return item, nil
}

func (s *localSources) Current(ctx context.Context, principal, id string) (governedprosekit.SourceRef, error) {
	if err := ctx.Err(); err != nil {
		return governedprosekit.SourceRef{}, err
	}
	if strings.TrimSpace(principal) == "" {
		return governedprosekit.SourceRef{}, governedprosekit.ErrEvidence
	}
	ref, ok := s.current[id]
	if !ok {
		return governedprosekit.SourceRef{}, governedprosekit.ErrEvidence
	}
	return ref, nil
}

// noModelClient makes accidental Generate wiring fail closed. check and export
// never call a model, and this command intentionally exposes neither operation.
type noModelClient struct{}

func (noModelClient) Complete(context.Context, llmclient.Request) (llmclient.Response, error) {
	return llmclient.Response{}, fmt.Errorf("kit-workflow: model calls are disabled")
}

func (noModelClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, fmt.Errorf("kit-workflow: model calls are disabled")
}
