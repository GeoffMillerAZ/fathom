// Command catalogctl resolves, explains and compares catalogs and prepares decisions.
// Concurrency: each invocation owns its state; no shared mutable state is used.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	internalcatalog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/cmd/internal/catalog"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

type usageError struct{ error }
type outputError struct{ error }
type outputWriter struct{ io.Writer }

func (w outputWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err != nil {
		return n, outputError{err}
	}
	if n != len(p) {
		return n, outputError{io.ErrShortWrite}
	}
	return n, nil
}

func usage(message string) error { return usageError{errors.New("catalogctl: " + message)} }

func execute(args []string, out, diagnostic io.Writer) int {
	err := run(args, outputWriter{out})
	if err == nil {
		return 0
	}
	fmt.Fprintln(diagnostic, err)
	var invocation usageError
	var pathError *os.PathError
	var outputFailure outputError
	if errors.As(err, &invocation) || errors.As(err, &pathError) || errors.As(err, &outputFailure) {
		return 2
	}
	return 1
}

func help(out io.Writer) error {
	_, err := fmt.Fprint(out, `Usage: catalogctl <command> [flags]

Commands:
  resolve   -spec spec.json [-policy policy.json] [-profile profile.json]
  scan-go   -root . [-revision REV] [-format bundle|rows|ndjson|digest]
  doctor    -bundle bundle.json OR -spec spec.json [-policy ...] [-profile ...]
  explain   -bundle bundle.json -id path/to/item
  query     -bundle bundle.json [-prefix path] [-facet axis=value]
  search    -bundle bundle.json -text words [-limit 20] [-include-hidden]
  diff      -before old.json -after new.json
  prepare   -bundle bundle.json -binding binding.json -definition definition.txt -evidence evidence.json [-identity identity.json] [-max-bytes 65536]

resolve output: -format bundle (default), rows, ndjson, digest.
Inputs are strict JSON exported from validated CUE. prepare makes no model call.
Flags are command-specific; unused or conflicting flags are usage errors.
Exit: 0 success/help, 1 checked-invalid input, 2 usage or I/O failure.
`)
	return err
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cap := catalogkit.DefaultLimits().MaxBytes
	b, err := io.ReadAll(io.LimitReader(f, int64(cap)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > cap {
		return nil, fmt.Errorf("catalogctl: file exceeds %d bytes", cap)
	}
	return b, nil
}
func readBundle(path string) (catalogkit.Bundle, error) {
	b, err := readBounded(path)
	if err != nil {
		return catalogkit.Bundle{}, err
	}
	return catalogkit.DecodeBundle(b)
}
func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

func writeBundle(w io.Writer, b catalogkit.Bundle, format string) error {
	switch format {
	case "bundle":
		return writeJSON(w, b)
	case "rows":
		return writeJSON(w, catalogkit.Rows(b))
	case "digest":
		_, err := fmt.Fprintln(w, b.Digest)
		return err
	case "ndjson":
		data, err := catalogkit.ExportNDJSON(b)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	default:
		return usage("unknown format " + format)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return usage("command required; use catalogctl help")
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		return help(out)
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	specPath := f.String("spec", "", "CUE-exported spec JSON")
	policyPath := f.String("policy", "", "receiver-owned policy JSON; defaults deny overrides")
	profilePath := f.String("profile", "", "receiver-owned named layer selection JSON")
	bundlePath := f.String("bundle", "", "verified bundle JSON")
	format := f.String("format", "bundle", "resolve output: bundle, rows, ndjson or digest")
	id := f.String("id", "", "node ID for explain")
	prefix := f.String("prefix", "", "query subtree ID")
	facet := f.String("facet", "", "query axis=value")
	before := f.String("before", "", "old bundle for diff")
	after := f.String("after", "", "new bundle for diff")
	bindingPath := f.String("binding", "", "CUE-exported decision binding JSON")
	evidencePath := f.String("evidence", "", "captured evidence JSON")
	definitionPath := f.String("definition", "", "exact current domain definition bytes pinned by binding")
	identityPath := f.String("identity", "", "optional provider/model/adapter/policy identity JSON for a reuse key")
	text := f.String("text", "", "lexical search terms")
	limit := f.Int("limit", 20, "maximum search results (1..1000)")
	includeHidden := f.Bool("include-hidden", false, "include hidden search entries")
	maxBytes := f.Int("max-bytes", 0, "prepared request byte cap; zero uses 64 KiB")
	root := f.String("root", ".", "Go module root for scan-go")
	revision := f.String("revision", "", "caller-supplied source revision for scan-go")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return help(out)
		}
		return usageError{err}
	}
	if f.NArg() != 0 {
		return usage("unexpected arguments")
	}
	if err := validateCommandFlags(args[0], f); err != nil {
		return err
	}
	switch args[0] {
	case "scan-go":
		entries, err := internalcatalog.Collect(*root)
		if err != nil {
			return err
		}
		b, err := internalcatalog.ResolveBundle(entries, *revision)
		if err != nil {
			return err
		}
		return writeBundle(out, b, *format)
	case "prepare":
		if *bundlePath == "" || *bindingPath == "" || *evidencePath == "" || *definitionPath == "" {
			return usage("-bundle, -binding, -definition and -evidence required")
		}
		b, err := readBundle(*bundlePath)
		if err != nil {
			return err
		}
		data, err := readBounded(*bindingPath)
		if err != nil {
			return err
		}
		bind, err := catalogkit.DecodeBinding(data)
		if err != nil {
			return err
		}
		data, err = readBounded(*evidencePath)
		if err != nil {
			return err
		}
		ev, err := catalogkit.DecodeEvidence(data)
		if err != nil {
			return err
		}
		definition, err := readBounded(*definitionPath)
		if err != nil {
			return err
		}
		prepared, err := catalogkit.PrepareDecision(b, bind, definition, ev, *maxBytes)
		if err != nil {
			return err
		}
		if *identityPath != "" {
			data, err := readBounded(*identityPath)
			if err != nil {
				return err
			}
			identity, err := catalogkit.DecodeDecisionIdentity(data)
			if err != nil {
				return err
			}
			key, err := catalogkit.DecisionCacheKey(prepared, identity)
			if err != nil {
				return err
			}
			return writeJSON(out, struct {
				Prepared catalogkit.Prepared `json:"prepared"`
				CacheKey string              `json:"cache_key"`
			}{prepared, key})
		}
		return writeJSON(out, prepared)
	case "resolve", "doctor":
		if args[0] == "doctor" && *bundlePath != "" {
			if *specPath != "" || *policyPath != "" || *profilePath != "" {
				return usage("doctor accepts a bundle OR authored inputs")
			}
			b, err := readBundle(*bundlePath)
			if err != nil {
				return err
			}
			return doctor(out, b)
		}
		if *specPath == "" {
			return usage("-spec required")
		}
		data, err := readBounded(*specPath)
		if err != nil {
			return err
		}
		s, err := catalogkit.DecodeSpec(data)
		if err != nil {
			return err
		}
		p := catalogkit.Policy{}
		if *policyPath != "" {
			data, err = readBounded(*policyPath)
			if err != nil {
				return err
			}
			p, err = catalogkit.DecodePolicy(data)
			if err != nil {
				return err
			}
		}
		var b catalogkit.Bundle
		if *profilePath != "" {
			data, err := readBounded(*profilePath)
			if err != nil {
				return err
			}
			profile, err := catalogkit.DecodeProfile(data)
			if err != nil {
				return err
			}
			b, err = catalogkit.ResolveProfile(s, p, profile)
			if err != nil {
				return err
			}
		} else {
			b, err = catalogkit.Resolve(s, p)
		}
		if err != nil {
			return err
		}
		if args[0] == "doctor" {
			return doctor(out, b)
		}
		return writeBundle(out, b, *format)
	case "explain", "query", "search":
		if *bundlePath == "" {
			return usage("-bundle required")
		}
		b, err := readBundle(*bundlePath)
		if err != nil {
			return err
		}
		if args[0] == "explain" {
			n, ok := b.Catalog.Get(*id)
			if !ok {
				return fmt.Errorf("catalogctl: unknown node %s", *id)
			}
			changes := []catalogkit.Change{}
			for _, c := range b.Changes {
				if c.ID == *id {
					changes = append(changes, c)
				}
			}
			return writeJSON(out, struct {
				BundleDigest string                 `json:"bundle_digest"`
				PolicyDigest string                 `json:"policy_digest"`
				Sources      []catalogkit.SourcePin `json:"sources"`
				NodeDigest   string                 `json:"node_digest"`
				Node         any                    `json:"node"`
				Changes      []catalogkit.Change    `json:"changes"`
			}{b.Digest, b.PolicyDigest, b.Sources, catalogkit.NodeDigest(n), n, changes})
		}
		if args[0] == "search" {
			if strings.TrimSpace(*text) == "" || *limit < 1 || *limit > 1000 {
				return usage("search needs -text and -limit in 1..1000")
			}
			nodes := make([]catalog.Node, 0, len(b.Catalog.Nodes))
			for _, n := range b.Catalog.Nodes {
				if *includeHidden || !n.Hidden {
					nodes = append(nodes, n)
				}
			}
			selected, err := catalog.New(nodes)
			if err != nil {
				return err
			}
			found := catalog.Search(selected, *text, catalog.NewBM25Ranker())
			matched := len(found.Results)
			if len(found.Results) > *limit {
				found.Results = found.Results[:*limit]
			}
			if found.Results == nil {
				found.Results = []catalog.Result{}
			}
			return writeJSON(out, struct {
				BundleDigest string           `json:"bundle_digest"`
				Query        string           `json:"query"`
				Matched      int              `json:"matched"`
				Results      []catalog.Result `json:"results"`
			}{b.Digest, found.Query, matched, found.Results})
		}
		axis, value, hasFacet := strings.Cut(*facet, "=")
		if *facet != "" && (!hasFacet || axis == "" || value == "") {
			return usage("-facet must be axis=value")
		}
		rows := []catalogkit.Row{}
		for _, r := range catalogkit.Rows(b) {
			if *prefix != "" && r.ID != *prefix && !strings.HasPrefix(r.ID, *prefix+"/") {
				continue
			}
			if hasFacet {
				match := false
				for _, v := range r.Facets[axis] {
					match = match || v == value
				}
				if !match {
					continue
				}
			}
			rows = append(rows, r)
		}
		return writeJSON(out, struct {
			BundleDigest string           `json:"bundle_digest"`
			Matched      int              `json:"matched"`
			Rows         []catalogkit.Row `json:"rows"`
		}{b.Digest, len(rows), rows})
	case "diff":
		if *before == "" || *after == "" {
			return usage("-before and -after required")
		}
		a, err := readBundle(*before)
		if err != nil {
			return err
		}
		b, err := readBundle(*after)
		if err != nil {
			return err
		}
		d, err := catalogkit.Compare(a, b)
		if err != nil {
			return err
		}
		return writeJSON(out, d)
	default:
		return usage("unknown command " + args[0])
	}
}

// validateCommandFlags rejects ignored configuration before opening any inputs.
// Visit includes explicitly supplied zero and false values, unlike value checks.
func validateCommandFlags(command string, f *flag.FlagSet) error {
	var names string
	switch command {
	case "resolve":
		names = "spec policy profile format"
	case "scan-go":
		names = "root revision format"
	case "doctor":
		bundleSelected := false
		f.Visit(func(v *flag.Flag) { bundleSelected = bundleSelected || v.Name == "bundle" })
		if bundleSelected {
			names = "bundle"
		} else {
			names = "spec policy profile"
		}
	case "explain":
		names = "bundle id"
	case "query":
		names = "bundle prefix facet"
	case "search":
		names = "bundle text limit include-hidden"
	case "diff":
		names = "before after"
	case "prepare":
		names = "bundle binding definition evidence identity max-bytes"
	default:
		return usage("unknown command " + command)
	}
	allowed := make(map[string]bool)
	for _, name := range strings.Fields(names) {
		allowed[name] = true
	}
	var invalid error
	f.Visit(func(v *flag.Flag) {
		if invalid == nil && !allowed[v.Name] {
			invalid = usage(fmt.Sprintf("%s does not accept -%s", command, v.Name))
		}
	})
	if invalid != nil {
		return invalid
	}
	value := func(name string) string { return f.Lookup(name).Value.String() }
	if command == "doctor" && names == "bundle" && value("bundle") == "" {
		return usage("doctor requires a nonempty -bundle path")
	}
	if command == "resolve" || command == "scan-go" {
		switch value("format") {
		case "bundle", "rows", "ndjson", "digest":
		default:
			return usage("unknown format " + value("format"))
		}
	}
	if command == "explain" && strings.TrimSpace(value("id")) == "" {
		return usage("explain requires -id")
	}
	if command == "search" {
		limit, _ := strconv.Atoi(value("limit")) // flag.Parse already checked the integer.
		if strings.TrimSpace(value("text")) == "" || limit < 1 || limit > 1000 {
			return usage("search needs -text and -limit in 1..1000")
		}
	}
	if command == "prepare" {
		limit, _ := strconv.Atoi(value("max-bytes")) // flag.Parse already checked the integer.
		if limit < 0 {
			return usage("prepare requires nonnegative -max-bytes")
		}
	}
	return nil
}

func doctor(out io.Writer, b catalogkit.Bundle) error {
	return writeJSON(out, struct {
		Status       string `json:"status"`
		BundleDigest string `json:"bundle_digest"`
		Layers       int    `json:"layers_checked"`
		Nodes        int    `json:"nodes_checked"`
		Changes      int    `json:"changes_checked"`
	}{"ok", b.Digest, len(b.Sources), len(b.Catalog.Nodes), len(b.Changes)})
}
