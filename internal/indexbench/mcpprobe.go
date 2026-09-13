package indexbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
		return json.Marshal(map[string]string{
			"name_path":     sym.Name,
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

// serenaReference is one entry of find_referencing_symbols' JSON array
// response (docs/bench/competitors.md).
type serenaReference struct {
	NamePath     string `json:"name_path"`
	RelativePath string `json:"relative_path"`
	Kind         string `json:"kind"`
	Line         int    `json:"line"`
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
		var refs []serenaReference
		if err := json.Unmarshal([]byte(text), &refs); err != nil {
			return nil, fmt.Errorf("indexbench: parse %s JSON response: %w", tool, err)
		}
		out := make([]refSite, 0, len(refs))
		for _, r := range refs {
			out = append(out, refSite{file: r.RelativePath, line: r.Line})
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
			from := s.file
			if s.line > 0 {
				from = fmt.Sprintf("%s:%d", s.file, s.line)
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
