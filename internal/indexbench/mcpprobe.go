package indexbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	mcpclient "github.com/geoffmilleraz/agentic-kit/foundations/agent/mcp-client"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// MCPServerConfig names a stdio MCP server ProbeReferencesViaMCP spawns:
// Command run with Args, from Dir (empty = the caller's working directory),
// with Env appended to the child's inherited environment. docs/bench/
// competitors.md names the exact command for each competitor (Serena,
// gopls).
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
}

// TruthSymbol is one query ProbeReferencesViaMCP asks the server about — a
// symbol the go/types truth declares (oracle.go's declaredNames), named and
// located the way a tool's own argument mapping needs.
type TruthSymbol struct {
	// Name is the qualified name (Name, or Recv.Method).
	Name string
	// File is the repo-relative file the symbol is declared in.
	File string
}

// TruthSymbolsForMCP resolves names (typically a sample of
// declaredNames(truth), the same names oracle.go asks fathom about) to the
// file each is declared in — the shape ProbeReferencesViaMCP's argument
// mapping needs.
func TruthSymbolsForMCP(truth publicloc.GoTruth, names []string) []TruthSymbol {
	fileOf := map[string]string{}
	for f, decls := range truth.DeclsByFile {
		for _, n := range decls {
			fileOf[n] = f
		}
	}
	out := make([]TruthSymbol, 0, len(names))
	for _, n := range names {
		out = append(out, TruthSymbol{Name: n, File: fileOf[n]})
	}
	return out
}

// mcpReferenceArgs builds tools/call arguments for tool, per docs/bench/
// competitors.md's per-tool argument mapping (SB-30): Serena's
// find_referencing_symbols wants {name_path, relative_path}; gopls's
// go_symbol_references wants {symbol, file}.
func mcpReferenceArgs(tool string, sym TruthSymbol) (json.RawMessage, error) {
	switch tool {
	case "find_referencing_symbols":
		// Serena resolves a method by its bare name within the file
		// ("HooksWired"); "Descriptor.HooksWired" and "Descriptor/HooksWired"
		// both answer "No symbol matching" (probed 2026-09-13, v1.7.0).
		name := sym.Name
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		return json.Marshal(map[string]string{
			"name_path":     name,
			"relative_path": sym.File,
		})
	case "go_symbol_references":
		return json.Marshal(map[string]string{
			"symbol": sym.Name,
			"file":   sym.File,
		})
	default:
		return nil, fmt.Errorf("indexbench: no MCP argument mapping registered for tool %q", tool)
	}
}

// refSite is one reference occurrence parsed out of a tool's response,
// before it is turned into an "file:line" Edge.From.
type refSite struct {
	file string
	line int
}

// serenaReference is one referencing symbol in find_referencing_symbols'
// response. The live shape (v1.7.0, probed 2026-09-13) is an object keyed
// by relative file path → symbol kind → entries; the reference LINE is not
// a field but the `>  N:` marker inside content_around_reference, and
// body_location is the span of the ENCLOSING symbol, never the reference.
type serenaReference struct {
	NamePath               string `json:"name_path"`
	ContentAroundReference string `json:"content_around_reference"`
	BodyLocation           struct {
		StartLine int `json:"start_line"`
		EndLine   int `json:"end_line"`
	} `json:"body_location"`
}

var reSerenaRefLine = regexp.MustCompile(`(?m)^\s*>\s*(\d+):`)

// parseSerenaReferences turns the file → kind → entries object into
// reference sites, one per `>  N:` marker; an entry with no marker falls
// back to the enclosing symbol's start line so the reference is still
// attributed to the right file.
func parseSerenaReferences(text string) ([]refSite, error) {
	var byFile map[string]map[string][]serenaReference
	if err := json.Unmarshal([]byte(text), &byFile); err != nil {
		return nil, err
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	var out []refSite
	for _, f := range files {
		kinds := byFile[f]
		kindNames := make([]string, 0, len(kinds))
		for k := range kinds {
			kindNames = append(kindNames, k)
		}
		sort.Strings(kindNames)
		for _, k := range kindNames {
			for _, r := range kinds[k] {
				marks := reSerenaRefLine.FindAllStringSubmatch(r.ContentAroundReference, -1)
				if len(marks) == 0 {
					out = append(out, refSite{file: f, line: r.BodyLocation.StartLine})
					continue
				}
				for _, m := range marks {
					line, _ := strconv.Atoi(m[1])
					out = append(out, refSite{file: f, line: line})
				}
			}
		}
	}
	return out, nil
}

var (
	reGoplsFile = regexp.MustCompile(`(?i)Located in the file:\s*([^\s,;]+)`)
	reGoplsLine = regexp.MustCompile(`(?i)located on line\s*(\d+)`)
)

// parseGoplsProse extracts (file, line) pairs from gopls's prose response
// ("Located in the file: <path>" … "located on line N", 1-indexed),
// pairing the i-th file mention with the i-th line mention — the order
// both regexes find them in the response text.
func parseGoplsProse(text string) []refSite {
	files := reGoplsFile.FindAllStringSubmatch(text, -1)
	lines := reGoplsLine.FindAllStringSubmatch(text, -1)
	n := len(files)
	if len(lines) < n {
		n = len(lines)
	}
	out := make([]refSite, 0, n)
	for i := 0; i < n; i++ {
		file := strings.TrimRight(files[i][1], ".,;:")
		line, _ := strconv.Atoi(lines[i][1])
		out = append(out, refSite{file: file, line: line})
	}
	return out
}

// parseMCPReferenceSites dispatches on tool: find_referencing_symbols
// returns JSON, go_symbol_references returns prose (SB-30).
func parseMCPReferenceSites(tool, text string) ([]refSite, error) {
	switch tool {
	case "find_referencing_symbols":
		out, err := parseSerenaReferences(text)
		if err != nil {
			return nil, fmt.Errorf("indexbench: parse %s JSON response: %w", tool, err)
		}
		return out, nil
	case "go_symbol_references":
		return parseGoplsProse(text), nil
	default:
		return nil, fmt.Errorf("indexbench: no response parser registered for tool %q", tool)
	}
}

// ProbeReferencesViaMCP starts the stdio MCP server cfg names, calls tool
// once per symbol with the per-tool argument mapping documented in
// docs/bench/competitors.md, and turns every reference the server reports
// into a comparable EdgeSet: Edge.From is "file:line" of the reference site
// (or the bare file when no line was parsed), Edge.To is the queried
// symbol's name, Edge.Kind is EdgeKindReference. A response that fails to
// parse is counted under EdgeSet.DroppedKinds["unparsed_response"] rather
// than aborting the whole probe — one competitor's malformed answer for one
// symbol must not erase every other symbol's real answer.
func ProbeReferencesViaMCP(ctx context.Context, cfg MCPServerConfig, tool string, symbols []TruthSymbol) (EdgeSet, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = cfg.Dir
	if len(cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: mcp server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: mcp server stdout: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: start mcp server %s: %w", cfg.Command, err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	tr := mcpclient.NewIOTransport(stdout, stdin, mcpclient.DefaultMaxMessageBytes)
	client, err := mcpclient.New(ctx, tr, mcpclient.Options{ClientName: "fathom-indexbench", ClientVersion: "0"})
	if err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: mcp handshake with %s: %w", cfg.Command, err)
	}
	defer func() { _ = client.Close() }()

	set := EdgeSet{DroppedKinds: map[string]int{}}
	for _, sym := range symbols {
		args, err := mcpReferenceArgs(tool, sym)
		if err != nil {
			return EdgeSet{}, err
		}
		text, err := client.CallTool(ctx, tool, args)
		if err != nil {
			return EdgeSet{}, fmt.Errorf("indexbench: %s(%s): %w", tool, sym.Name, err)
		}
		sites, err := parseMCPReferenceSites(tool, text)
		if err != nil {
			set.DroppedKinds["unparsed_response"]++
			continue
		}
		for _, s := range sites {
			file := s.file
			if cfg.Dir != "" && filepath.IsAbs(file) {
				// gopls answers absolute paths; the truth is repo-relative.
				if rel, err := filepath.Rel(cfg.Dir, file); err == nil && !strings.HasPrefix(rel, "..") {
					file = rel
				}
			}
			from := file
			if s.line > 0 {
				from = fmt.Sprintf("%s:%d", file, s.line)
			}
			set.Edges = append(set.Edges, Edge{From: from, To: sym.Name, Kind: EdgeKindReference})
		}
	}
	sort.Slice(set.Edges, func(i, j int) bool {
		if set.Edges[i].To != set.Edges[j].To {
			return set.Edges[i].To < set.Edges[j].To
		}
		return set.Edges[i].From < set.Edges[j].From
	})
	if len(set.DroppedKinds) == 0 {
		set.DroppedKinds = nil
	}
	return set, nil
}
