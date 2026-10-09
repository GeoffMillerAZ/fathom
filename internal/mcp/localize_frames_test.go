package mcp_test

// localize_frames_test.go — TL-1 (docs/plans/telemetry-links.md,
// ADR-062): localize over the ONE shared frame fixture
// (internal/stackframe/testdata/frames.json, also read by the prompt-hook
// test in cmd/fathom).

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/stackframe"
)

type frameFixture struct {
	Sentinel string `json:"sentinel"`
	Cases    []struct {
		Name   string             `json:"name"`
		Shape  string             `json:"shape"`
		Text   string             `json:"text"`
		Frames []stackframe.Frame `json:"frames"`
	} `json:"cases"`
}

func loadFrameFixture(t *testing.T) frameFixture {
	t.Helper()
	b, err := os.ReadFile("../stackframe/testdata/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var f frameFixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// frameRepoPath maps a frame's runtime path to the fixture repository's
// path (a deploy root such as /app or /var/task stripped; a JVM frame laid
// out by its package under src/main/java).
func frameRepoPath(f stackframe.Frame) string {
	if f.PathHint != "" {
		return "src/main/java/" + f.PathHint
	}
	p := strings.TrimPrefix(f.File, "/")
	for _, root := range []string{"app/", "var/task/"} {
		p = strings.TrimPrefix(p, root)
	}
	return p
}

// frameBackend indexes one symbol per (file, function) of a case's frames,
// spanning every line the trace names for it, with typed caller edges from
// each frame to the one above it; JVM cases gain a decoy file of the same
// basename (and the same method name) in a package that sorts first.
func frameBackend(t *testing.T, frames []stackframe.Frame) (*localizeFakeBackend, core.SymbolNode) {
	t.Helper()
	type span struct{ lo, hi int }
	spans := map[string]span{}
	var order []string
	symName := func(f stackframe.Frame) string {
		if n := stackframe.ShortName(f.Func); n != "" {
			return n
		}
		return fmt.Sprintf("anon%d", f.Depth)
	}
	for _, f := range frames {
		if f.File == "" {
			continue
		}
		k := frameRepoPath(f) + "\x00" + symName(f)
		s, ok := spans[k]
		if !ok {
			s = span{f.Line - 1, f.Line + 1}
			order = append(order, k)
		}
		if f.Line-1 < s.lo {
			s.lo = f.Line - 1
		}
		if f.Line+1 > s.hi {
			s.hi = f.Line + 1
		}
		spans[k] = s
	}
	var nodes []core.SymbolNode
	byKey := map[string]core.SymbolNode{}
	for _, k := range order {
		file, name, _ := strings.Cut(k, "\x00")
		n := core.SymbolNode{Service: "svc", Package: "pkg" + fmt.Sprint(len(nodes)), Name: name, Kind: core.KindGoFunc, FilePath: file, Line: spans[k].lo, EndLine: spans[k].hi}
		nodes = append(nodes, n)
		byKey[k] = n
		if strings.HasPrefix(file, "src/main/java/") {
			base := file[strings.LastIndex(file, "/")+1:]
			nodes = append(nodes, core.SymbolNode{Service: "svc", Package: "decoy" + fmt.Sprint(len(nodes)), Name: name, Kind: core.KindGoFunc, FilePath: "src/main/java/aaa/" + base, Line: 1, EndLine: 5000})
		}
	}
	// A frame known only by its function (PHP's thrower in a bare
	// getTraceAsString) lives in its owner's file, unlocated by the trace.
	var throw core.SymbolNode
	if len(frames) > 0 && frames[0].File == "" {
		owner := strings.TrimSuffix(frames[0].Func, stackframe.ShortName(frames[0].Func))
		owner = stackframe.ShortName(strings.TrimRight(owner, ":->.#"))
		throw = core.SymbolNode{Service: "svc", Package: "owner", Name: stackframe.ShortName(frames[0].Func), Kind: core.KindGoFunc, FilePath: "lib/" + owner + ".php", Line: 1, EndLine: 5}
		nodes = append(nodes, throw)
		be := &localizeFakeBackend{nodes: nodes, svcPath: t.TempDir(), indexCommit: "c1"}
		return be, throw
	}
	var edges []core.Edge
	for i, f := range frames {
		if f.File == "" {
			continue
		}
		cur := byKey[frameRepoPath(f)+"\x00"+symName(f)]
		if throw.Name == "" {
			throw = cur
		}
		if i+1 < len(frames) && frames[i+1].File != "" {
			caller := byKey[frameRepoPath(frames[i+1])+"\x00"+symName(frames[i+1])]
			if caller.Key() != cur.Key() {
				edges = append(edges, core.Edge{FromService: "svc", FromSymbol: caller.Key(), ToSymbol: cur.Key(), Kind: core.EdgeDirect})
			}
		}
	}
	return &localizeFakeBackend{nodes: nodes, typedEdges: edges, svcPath: t.TempDir(), indexCommit: "c1"}, throw
}

// TestLocalize_ThrowSiteIsTheTopCandidateOnEveryFixtureShape is TL-1's
// end-to-end row: over the shared fixture, every raw grammar and every
// payload shape puts the enclosing symbol of the code that threw first,
// and a JVM frame's package picks its file over a same-named decoy.
func TestLocalize_ThrowSiteIsTheTopCandidateOnEveryFixtureShape(t *testing.T) {
	f := loadFrameFixture(t)
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			be, throw := frameBackend(t, c.Frames)
			srv := mcp.NewServer(be)
			got := localizeCall(t, srv, map[string]any{"service": "svc", "stack_trace": c.Text})
			cands, _ := got["candidates"].([]any)
			if len(cands) == 0 {
				t.Fatalf("no candidates: %v", got)
			}
			top := cands[0].(map[string]any)
			if top["file"] != throw.FilePath || top["symbol"] != throw.Name {
				t.Errorf("top = %v %v, want the throw site %s %s; candidates %v", top["file"], top["symbol"], throw.FilePath, throw.Name, cands)
			}
			if c.Shape != "raw" {
				p, ok := got["payload"].(map[string]any)
				if !ok || p["frames"] == float64(0) {
					t.Errorf("payload disclosure = %v, want shapes and a frame count", got["payload"])
				}
			}
		})
	}
}

// TestLocalize_PayloadMessageFieldsNeverEchoed: frame tuples only (ADR-058
// decision 3). The sentinel sits in message, breadcrumb, user and metaData
// fields (and quoted inside them); no localize answer built from a payload
// carries it, under stack_trace or error, nor one built from a raw trace
// under stack_trace.
func TestLocalize_PayloadMessageFieldsNeverEchoed(t *testing.T) {
	f := loadFrameFixture(t)
	for _, c := range f.Cases {
		kinds := []string{"stack_trace"}
		if c.Shape != "raw" {
			kinds = append(kinds, "error")
		}
		for _, kind := range kinds {
			be, _ := frameBackend(t, c.Frames)
			srv := mcp.NewServer(be)
			got := localizeCall(t, srv, map[string]any{"service": "svc", kind: c.Text})
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), f.Sentinel) {
				t.Errorf("%s (%s): answer echoes message text: %s", c.Name, kind, b)
			}
		}
	}
}

// TestLocalize_UnregisteredJSONIsNeverReformulated: a JSON value no
// registered shape matches seeds nothing, and its fields are not mined by
// reformulation.
func TestLocalize_UnregisteredJSONIsNeverReformulated(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{"service": "svc", "error": `{"msg":"getBalanceForUser failed SENTINEL_c1_TL1"}`})
	if got["reformulated"] == true {
		t.Errorf("a payload was reformulated: %v", got)
	}
	if b, _ := json.Marshal(got); strings.Contains(string(b), "SENTINEL_c1_TL1") || strings.Contains(string(b), "getBalanceForUser") {
		t.Errorf("answer echoes payload text: %s", b)
	}
}
