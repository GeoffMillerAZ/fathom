package benchprep

import (
	"context"
	"fmt"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// FunctionRef identifies a gold function by its base-tree relative path
// and name (bare or qualified, e.g. Worker.Run). Symbol may instead name
// the exact opaque SCIP symbol, avoiding ambiguity for overloads.
type FunctionRef struct {
	Path   string `json:"path"`
	Name   string `json:"name,omitempty"`
	Symbol string `json:"symbol,omitempty"`
}

// HopRequest supplies independent truth. SCIP takes precedence over Root;
// otherwise Root names an already-prepared Go module at the task's base.
// Test variants are excluded. No index, store or network is consulted.
type HopRequest struct {
	Root          string
	Issue         string
	GoldFunctions []FunctionRef
	SCIP          *scippb.Index
}

// HopResult is a covariate, never a stratum. Distance is nonnegative when
// Bucket is empty; otherwise it is -1 and Bucket is "no-graph-path" (an
// oracle exists, but no named source reaches gold) or "no-oracle" (truth
// is unavailable or incomplete). Oracle discloses the graph semantics.
type HopResult struct {
	Distance int    `json:"distance"`
	Bucket   string `json:"bucket,omitempty"`
	Oracle   string `json:"oracle,omitempty"`
}

type hopNode struct {
	id, path string
	symbol   string
	aliases  []string
}

type hopGraph struct {
	nodes map[string]hopNode
	edges map[string][]string
}

// HopDistance finds the shortest directed path from ANY function named
// in the issue to ANY gold function. Whole identifierToken tokens are
// matched exactly, case-sensitively, against oracle symbols, including
// plain prose names. The stratum's shape/length/stopword filters do not
// apply. Qualified names are never reduced to their leaf (bare names
// intentionally select all matching subject functions).
//
// Go truth uses go/packages, go/types-backed SSA and CHA. CHA is chosen
// because subjects include libraries without main: unlike reachability
// analyses it needs no chosen entry point. It conservatively includes
// interface and function-value targets, so this is static distance, not
// proof of a runtime path. Dependencies must already be available locally;
// module, checksum and toolchain downloads are disabled, as are workspaces
// and module updates. No subject code or test is executed.
//
// SCIP truth uses reference occurrences within the innermost definition's
// half-open EnclosingRange, not relationships or fathom-extracted nodes.
// It measures a reference graph, which may contain non-call references.
// Missing definition extents are refused rather than invented from token
// ranges. A missing oracle yields no-oracle; malformed or incomplete truth
// additionally returns an error, never a misleading no-graph-path result.
func HopDistance(ctx context.Context, req HopRequest) (HopResult, error) {
	result := HopResult{Distance: -1, Bucket: "no-oracle"}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if req.SCIP == nil && req.Root == "" {
		return result, nil
	}
	if len(req.GoldFunctions) == 0 {
		return result, fmt.Errorf("hop distance: no gold functions")
	}
	for _, gold := range req.GoldFunctions {
		if !fs.ValidPath(gold.Path) || (gold.Name == "" && gold.Symbol == "") {
			return result, fmt.Errorf("hop distance: gold needs a relative path and function name or symbol")
		}
	}
	var graph hopGraph
	var err error
	if req.SCIP != nil {
		result.Oracle = "scip-references"
		graph, err = scipHopGraph(req.SCIP)
	} else {
		result.Oracle = "go/types+cha"
		graph, err = goHopGraph(ctx, req.Root)
	}
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	ids := make([]string, 0, len(graph.nodes))
	for id := range graph.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	goldIDs := map[string]bool{}
	for _, gold := range req.GoldFunctions {
		found := false
		for _, id := range ids {
			node := graph.nodes[id]
			if node.path == gold.Path && ((gold.Symbol != "" && node.symbol == gold.Symbol) || (gold.Symbol == "" && hasName(node.aliases, gold.Name))) {
				goldIDs[id] = true
				found = true
			}
		}
		if !found {
			return result, fmt.Errorf("hop distance: gold function absent from oracle: %s (%s)", gold.Name, gold.Path)
		}
	}
	names := identifierToken.FindAllString(req.Issue, -1)
	type step struct {
		id       string
		distance int
	}
	queue := []step{}
	seen := map[string]bool{}
	for _, id := range ids {
		node := graph.nodes[id]
		if node.path == "" { // dependencies are intermediates, not issue subjects
			continue
		}
		for _, name := range names {
			if hasName(node.aliases, name) {
				queue = append(queue, step{id: id})
				seen[id] = true
				break
			}
		}
	}
	for head := 0; head < len(queue); head++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		current := queue[head]
		if goldIDs[current.id] {
			result.Distance, result.Bucket = current.distance, ""
			return result, nil
		}
		for _, to := range graph.edges[current.id] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, step{id: to, distance: current.distance + 1})
			}
		}
	}
	result.Bucket = "no-graph-path"
	return result, nil
}

func hasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func goHopGraph(ctx context.Context, root string) (hopGraph, error) {
	graph := hopGraph{nodes: map[string]hopNode{}, edges: map[string][]string{}}
	root, err := filepath.Abs(root)
	if err != nil {
		return graph, fmt.Errorf("hop distance: resolve Go root: %w", err)
	}
	// Explicit overrides are appended last, as required by packages.Config.Env.
	env := append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly", "GOTELEMETRY=off", "GOPACKAGESDRIVER=off")
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: root, Mode: packages.LoadAllSyntax, Env: env}, "./...")
	if err != nil {
		return graph, fmt.Errorf("hop distance: load Go oracle: %w", err)
	}
	var problems []string
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, problem := range pkg.Errors {
			problems = append(problems, problem.Msg)
		}
	})
	if len(problems) != 0 {
		sort.Strings(problems)
		return graph, fmt.Errorf("hop distance: incomplete Go oracle: %s", strings.Join(problems, "; "))
	}
	if len(pkgs) == 0 {
		return graph, fmt.Errorf("hop distance: no Go packages")
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	calls := cha.CallGraph(prog)
	for fn, node := range calls.Nodes {
		if fn == nil { // CHA's distinguished root is not a source function.
			continue
		}
		id := fn.String()
		item := hopNode{id: id}
		if pos := prog.Fset.Position(fn.Pos()); pos.IsValid() {
			if path, err := filepath.Rel(root, pos.Filename); err == nil && path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator)) {
				item.path = filepath.ToSlash(path)
			}
		}
		if obj, ok := fn.Object().(*types.Func); ok && obj.Pkg() != nil {
			name := obj.Name()
			item.aliases = append(item.aliases, name)
			if recv := obj.Type().(*types.Signature).Recv(); recv != nil {
				typ := recv.Type()
				if pointer, ok := typ.(*types.Pointer); ok {
					typ = pointer.Elem()
				}
				if named, ok := typ.(*types.Named); ok {
					name = named.Obj().Name() + "." + name
					item.aliases = append(item.aliases, name)
				}
			}
			item.aliases = append(item.aliases, obj.Pkg().Name()+"."+name, obj.Pkg().Path()+"."+name)
		}
		sort.Strings(item.aliases)
		graph.nodes[id] = item
		for _, edge := range node.Out {
			graph.edges[id] = append(graph.edges[id], edge.Callee.Func.String())
		}
		sort.Strings(graph.edges[id])
	}
	return graph, nil
}
