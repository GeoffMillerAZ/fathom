package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/claudecli"
)

// fsEntry is one path's state in a snapshot: its kind and, for a file, a
// digest of its bytes (for a link, its target).
type fsEntry struct{ kind, sum string }

// snapshotTree records every directory, file and link under base.
func snapshotTree(t *testing.T, base string) map[string]fsEntry {
	t.Helper()
	m := map[string]fsEntry{}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, rerr := os.Readlink(p)
			if rerr != nil {
				return rerr
			}
			m[p] = fsEntry{"link", target}
		case d.IsDir():
			m[p] = fsEntry{"dir", ""}
		default:
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			sum := sha256.Sum256(data)
			m[p] = fsEntry{"file", hex.EncodeToString(sum[:])}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", base, err)
	}
	return m
}

// touchedFiles is every non-directory path whose state differs between two
// snapshots: created, changed or removed. Sorted.
func touchedFiles(before, after map[string]fsEntry) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for p, a := range after {
		if a.kind == "dir" {
			continue
		}
		if b, ok := before[p]; !ok || b != a {
			add(p)
		}
	}
	for p, b := range before {
		if b.kind == "dir" {
			continue
		}
		if _, ok := after[p]; !ok {
			add(p)
		}
	}
	sort.Strings(out)
	return out
}

var dryRunFileLine = regexp.MustCompile(`^would (create|update|link|remove) (.+)$`)

// dryRunList parses the files a dry run names into absolute paths: a
// repo-relative path is joined to root, a "~/" path to home.
func dryRunList(out, root, home string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		m := dryRunFileLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		p := m[2]
		switch {
		case strings.HasPrefix(p, "~/"):
			p = filepath.Join(home, filepath.FromSlash(p[2:]))
		case !filepath.IsAbs(p):
			p = filepath.Join(root, filepath.FromSlash(p))
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// countingInitDaemon is fakeInitDaemon's shape with a count of the ingest
// calls, so a dry run can be held to asking the daemon for none.
func countingInitDaemon(t *testing.T, service string, ingests *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "ingest":
			ingests.Add(1)
			inner = fmt.Sprintf(`{"service":%q,"ok":true}`, service)
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":7,"languages":["go"]}]}`, service)
		default:
			http.Error(w, "unknown tool", http.StatusBadRequest)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestInitDryRunListsExactlyTheRealRunsTouchedSet holds `fathom init
// --dry-run` to two promises: it writes nothing (the tree, the home and the
// state directory are byte-identical after it, directories included, and
// the daemon is asked for no ingest), and the files it lists are exactly
// the files the real run then creates, changes or removes — the touched set
// read off the disk, not off init's own account of itself.
func TestInitDryRunListsExactlyTheRealRunsTouchedSet(t *testing.T) {
	cases := []struct {
		name  string
		flags []string
		seed  func(t *testing.T, root string, run func([]string) (int, string))
	}{
		{name: "a fresh repository, default flags"},
		{
			name:  "skills and the user ignore seed, over an existing AGENTS.md and CLAUDE.md",
			flags: []string{"--skills", "--seed-user-ignore"},
			seed: func(t *testing.T, root string, _ func([]string) (int, string)) {
				for f, body := range map[string]string{"AGENTS.md": "# Project rules\n", "CLAUDE.md": "# Claude\n"} {
					if err := os.WriteFile(filepath.Join(root, f), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name:  "a second run over a repository init already wired",
			flags: []string{"--skills"},
			seed: func(t *testing.T, _ string, run func([]string) (int, string)) {
				if code, out := run(nil); code != 0 {
					t.Fatalf("first real run exit %d:\n%s", code, out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			root := filepath.Join(base, "repo")
			for _, d := range []string{home, root} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", "")
			t.Setenv("FATHOM_STATE_DIR", "")
			t.Setenv("FATHOM_HOOKS_SCOPE", "")

			var ingests atomic.Int32
			srv := countingInitDaemon(t, "demo-svc", &ingests)
			args := []string{"--path", root, "--service", "demo-svc", "--url", srv.URL, "--bin", testBinPath, "--no-scip"}
			args = append(args, tc.flags...)
			run := func(extra []string) (int, string) {
				var out, errb bytes.Buffer
				code := cmdInit(append(append([]string{}, args...), extra...), &out, &errb)
				return code, out.String() + errb.String()
			}
			if tc.seed != nil {
				tc.seed(t, root, run)
			}

			before := snapshotTree(t, base)
			ingests.Store(0)
			code, dryOut := run([]string{"--dry-run"})
			if code != 0 {
				t.Fatalf("dry run exit %d, want 0:\n%s", code, dryOut)
			}
			if after := snapshotTree(t, base); len(touchedFiles(before, after)) != 0 || len(after) != len(before) {
				t.Fatalf("the dry run wrote: %v (entries %d -> %d)\n%s", touchedFiles(before, after), len(before), len(after), dryOut)
			}
			if n := ingests.Load(); n != 0 {
				t.Fatalf("the dry run asked the daemon for %d ingest(s), want none", n)
			}
			if !strings.Contains(dryOut, "would ingest") {
				t.Errorf("the dry run does not name the daemon ingest it skipped:\n%s", dryOut)
			}
			listed := dryRunList(dryOut, root, home)

			code, realOut := run(nil)
			if code != 0 {
				t.Fatalf("real run exit %d, want 0:\n%s", code, realOut)
			}
			touched := touchedFiles(before, snapshotTree(t, base))
			if strings.Join(listed, "\n") != strings.Join(touched, "\n") {
				t.Fatalf("the dry run's list is not the real run's touched set\nlisted:\n  %s\ntouched:\n  %s\ndry run output:\n%s",
					strings.Join(listed, "\n  "), strings.Join(touched, "\n  "), dryOut)
			}
			if tc.seed == nil && len(listed) == 0 {
				t.Fatalf("a fresh repository listed nothing:\n%s", dryOut)
			}
		})
	}
}

// rejectingInitProvider is both provider ports and the HTTP transport. Any
// attempt to spend or contact a daemon/provider is an immediate test failure.
type rejectingInitProvider struct{ t *testing.T }

func (p rejectingInitProvider) Complete(context.Context, llmclient.Request) (llmclient.Response, error) {
	p.t.Error("dry run called the compile provider")
	return llmclient.Response{}, errors.New("unexpected provider call")
}
func (p rejectingInitProvider) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	p.t.Error("dry run streamed from the compile provider")
	return nil, errors.New("unexpected provider call")
}
func (p rejectingInitProvider) Name() string { return "rejecting" }
func (p rejectingInitProvider) Decide(context.Context, decision.Request) (decision.Response, error) {
	p.t.Error("dry run called the decision provider")
	return decision.Response{}, errors.New("unexpected provider call")
}
func (p rejectingInitProvider) RoundTrip(*http.Request) (*http.Response, error) {
	p.t.Error("dry run made a network call")
	return nil, errors.New("unexpected network call")
}

// Each advertised facet is tied to its shipped implementation, rather than
// the planned vocabulary (role was refused; configures has no implementation).
func TestInitDryRunFacetsMatchImplementedSymbols(t *testing.T) {
	main := core.SymbolNode{Service: "fixture", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "main.go"}
	implemented := map[string]bool{
		"entry-point": callgraph.ProductEntryReach(callgraph.Build([]core.SymbolNode{main})).HasEntries(),
		"test-entry":  repofacts.IsTestEntry("TestExample", "example_test.go", ""),
	}
	// Read the actual doc-dialect registration for the image reference facet.
	// Its emitted edge supplies the name, rather than a second literal list.
	x, err := extractor.NewDocDialectExtractor("docker-compose")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := x.ExtractDoc("fixture", "compose.yaml", []byte("services:\n  app:\n    image: example/app:1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range doc.CrossFile {
		if ref.Strategy == "image-ref" {
			implemented[ref.Edge] = true
		}
	}
	if !implemented["entry-point"] || !implemented["test-entry"] || len(implemented) != 3 {
		t.Fatalf("fixture did not exercise every implemented facet: %v", implemented)
	}
	isolateConfigDirs(t)
	var out bytes.Buffer
	printInitCost(&out, ".", "fixture", semantic.Compiler{})
	got := regexp.MustCompile(`(?m)^  ([^:]+): \$0;`).FindAllStringSubmatch(out.String(), -1)
	if len(got) != len(implemented) {
		t.Errorf("facets = %v, want exactly the implemented symbols", got)
	}
	for _, m := range got {
		if !implemented[m[1]] {
			t.Errorf("facet %s has no implementation", m[1])
		}
	}
}

func TestInitDryRunDecisionSiteCommands(t *testing.T) {
	isolateConfigDirs(t)
	var out bytes.Buffer
	printInitCost(&out, ".", "fixture", semantic.Compiler{})
	commands := map[string]string{
		decide.SiteRoute:             "fathom serve-http",
		decide.SiteCompilePrefilter:  "fathom compile --service fixture --dry-run=false",
		decide.SiteObligationsRerank: "fathom serve-http",
		decide.SiteCertifySuggest:    "fathom certify suggest --service fixture",
		decide.SiteAssess:            "fathom assess --project .",
		decide.SitePreflight:         "fathom plans context <plan.md> --row <id> --service fixture --store <db> --preflight --score",
	}
	if len(commands) != len(decide.AllSites) {
		t.Fatal("site command coverage differs from AllSites")
	}
	for _, site := range decide.AllSites {
		command, ok := commands[site]
		if !ok {
			t.Errorf("no command for %s", site)
			continue
		}
		prefix := "  " + site + ": unpriced (no measurement yet); command: " + command + " (requires [decide] provider configured"
		if !strings.Contains(out.String(), prefix) {
			t.Errorf("missing %s:\n%s", prefix, &out)
		}
		var line string
		for _, l := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(l, "  "+site+":") {
				line = l
			}
		}
		if site == decide.SiteRoute {
			if !strings.Contains(line, "sites including route") {
				t.Error("route needs an explicit site opt-in")
			}
		} else if !strings.Contains(line, "site allowed by default") {
			t.Errorf("%s misstates default site membership: %s", site, line)
		}
		if site == decide.SiteObligationsRerank && !strings.Contains(line, "; [decide] rank_within_tier = true") {
			t.Error("rerank rank_within_tier gate omitted")
		}
		if !strings.Contains(line, "provider none: no call") {
			t.Errorf("%s omits provider none fallback: %s", site, line)
		}
		if site == decide.SiteAssess && !strings.Contains(line, "fathom capabilities set assess on") {
			t.Error("assess capability gate omitted")
		}
	}
}

func TestInitDryRunHelpMentionsCost(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"init", "--help"}, &out, &errOut); code != 0 {
		t.Fatalf("help exit %d: %s", code, &errOut)
	}
	if !strings.Contains(out.String(), "Cost") {
		t.Fatalf("help omits Cost section: %s", &out)
	}
}

func TestInitDryRunPricedSectionSpendsNothing(t *testing.T) {
	isolateConfigDirs(t)
	clearProfileEnv(t)
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":                   "module example.com/fixture\n\ngo 1.22\n",
		"main.go":                  "package main\nfunc main() {}\n",
		"AGENTS.md":                "# Rules\nKeep output sorted.\n",
		"ADR.md":                   "# Decisions\nUse pure Go.\n",
		"docs/plans/example.md":    "# Work\nKeep the public contract.\n",
		".claude/AGENTS.md":        "# Additional root guide\nPreserve the public contract.\n",
		".cursor/rules/sorted.mdc": "---\ndescription: sorted\nglobs: **/*.go\n---\nKeep all results sorted.\n",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	provider := rejectingInitProvider{t}
	oldTransport, oldConstructor := http.DefaultTransport, newClaudeCLIClient
	http.DefaultTransport = provider
	constructed := 0
	newClaudeCLIClient = func(claudecli.Config) (llmclient.LLMClient, error) { constructed++; return provider, nil }
	t.Cleanup(func() { http.DefaultTransport = oldTransport; newClaudeCLIClient = oldConstructor })
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_PRICE_TABLE", filepath.Join(root, "prices.toml"))
	t.Setenv("FATHOM_STATE_DIR", filepath.Join(root, "state"))
	cases := []struct {
		name, model, table string
		known              bool
	}{
		{"fresh default", defaultCompileModel, "", true},
		{"irrelevant user prices", defaultCompileModel, "[models.other]\ninput_per_mtok=99\noutput_per_mtok=99\n", true},
		{"invalid user prices", defaultCompileModel, "not valid toml [", true},
		{"unknown model with user price", "fixture-model", "[models.fixture-model]\ninput_per_mtok=1\noutput_per_mtok=5\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FATHOM_COMPILE_MODEL", tc.model)
			if tc.table != "" {
				if err := os.WriteFile("prices.toml", []byte(tc.table), 0644); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotTree(t, root)
			var out, errOut bytes.Buffer
			if code := run([]string{"init", "--dry-run", "--path", root, "--service", "fixture", "--no-scip", "--bin", testBinPath}, &out, &errOut); code != 0 {
				t.Fatalf("init exit %d: %s %s", code, &out, &errOut)
			}
			text := out.String()
			start := strings.Index(text, "\nCost\n")
			if start < 0 || start < strings.Index(text, "nothing was written") {
				t.Fatalf("Cost must follow preview: %s", text)
			}
			section := text[start:]
			names := []string{"entry-point", "runs", "test-entry"}
			names = append(names, semantic.DefaultLayersFor(root)...)
			sites := append([]string(nil), decide.AllSites...)
			sort.Strings(sites)
			names = append(names, sites...)
			last := -1
			for _, name := range names {
				at := strings.Index(section, "  "+name+":")
				if at <= last {
					t.Fatalf("missing/out-of-order %s: %s", name, section)
				}
				last = at
			}
			st := store.NewMemoryStore()
			defer st.Close()
			ctx := semantic.WithSizingEstimate(context.Background())
			if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "fixture", Path: root}); err != nil {
				t.Fatal(err)
			}
			inRate, outRate, _ := rateForModel(tc.model)
			c := client.New(connector.NewLocalConnector(), st).WithLLM(provider, tc.model, inRate, outRate)
			c.WithDecision(provider, config.DefaultDecide, nil)
			for _, name := range semantic.DefaultLayersFor(root) {
				compiled, err := c.CompileLayer(ctx, "fixture", name, true)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(section, "command: fathom compile --service fixture --layer "+name+" --dry-run=false") {
					t.Errorf("missing spending command for %s", name)
				}
				if compiled.TokensIn <= 0 {
					t.Fatalf("%s fixture produced no tokens", name)
				}
				price := "unknown"
				if tc.known {
					price = fmt.Sprintf("$%.6f-$%.6f", compiled.EstCostLowUSD, compiled.EstCostHighUSD)
				}
				want := fmt.Sprintf("  %s: input %d tokens; output %d-%d tokens; price: %s (model %s);", name, compiled.TokensIn, compiled.EstTokensOutLow, compiled.EstTokensOutHigh, price, tc.model)
				if !strings.Contains(section, want) {
					t.Errorf("init differs from compile, want %s:\n%s", want, section)
				}
			}
			for _, line := range strings.Split(strings.TrimSpace(section), "\n")[1:] {
				if !strings.Contains(line, "command: fathom ") {
					t.Errorf("missing command: %s", line)
				}
			}
			if after := snapshotTree(t, root); len(touchedFiles(before, after)) != 0 || len(after) != len(before) {
				t.Fatal("dry run changed repository")
			}
		})
	}
	if constructed != 0 {
		t.Fatalf("CLI dry run constructed %d subprocess providers", constructed)
	}
	// Also supply both rejecting ports directly to the sizing engine.
	var priced bytes.Buffer
	printInitCost(&priced, root, "fixture", semantic.Compiler{LLM: provider, Decider: provider, Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	if !strings.Contains(priced.String(), "obligations: input ") {
		t.Fatal("no estimate")
	}
}
