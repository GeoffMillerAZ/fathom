package syslink

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fleetDialect maps a fixture file to its ADR-048 dialect id by extension.
func fleetDialect(path string) string {
	switch filepath.Ext(path) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".js":
		return "javascript"
	case ".py":
		return "python"
	case ".proto":
		return "proto"
	case ".yaml", ".yml":
		return "yaml"
	}
	return ""
}

type answerKey struct {
	Links []struct {
		Kind     string `json:"kind"`
		Key      string `json:"key"`
		Consumer struct {
			Service string `json:"service"`
			File    string `json:"file"`
			Line    int    `json:"line"`
		} `json:"consumer"`
		Provider struct {
			Service string `json:"service"`
		} `json:"provider"`
		Tier string `json:"tier"`
	} `json:"links"`
}

// TestFauxtownFleetAnswerKey runs the wave-2 sources and the link join over
// the fauxtown fixture fleet and reads the result against the hand-written
// answer key (SG-14): no link outside the key (precision), and every key
// link found at its tier except the ones a named, not-yet-built extractor
// owns (the TypeScript gRPC-web stub consumer).
func TestFauxtownFleetAnswerKey(t *testing.T) {
	root := filepath.Join("..", "connector", "testdata", "fauxtown")
	raw, err := os.ReadFile(filepath.Join(root, "links.answer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ak answerKey
	if err := json.Unmarshal(raw, &ak); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	reg := DefaultRegistry()
	var rows []KeyRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		svc := e.Name()
		var files []SourceFile
		_ = filepath.WalkDir(filepath.Join(root, svc), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			dialect := fleetDialect(p)
			if dialect == "" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			files = append(files, SourceFile{Path: rel, Dialect: dialect, Content: b})
			for _, r := range reg.Keys(rel, b, dialect) {
				r.Service = svc
				rows = append(rows, r)
			}
			return nil
		})
		for _, r := range protoRPCKeys(files) {
			if r.Role == RoleConsumes {
				r.Service = svc
				rows = append(rows, r)
			}
		}
	}
	type id struct {
		cs, file string
		line     int
		key, ps  string
	}
	got := map[id]Tier{}
	for _, l := range MatchLinks(rows) {
		if l.ConsumerService == l.ProviderService {
			continue
		}
		k := id{l.ConsumerService, l.Evidence.Consumer.File, l.Evidence.Consumer.Line, l.Key.String(), l.ProviderService}
		if prev, ok := got[k]; !ok || tierRank(l.Tier) < tierRank(prev) {
			got[k] = l.Tier
		}
	}
	want := map[id]Tier{}
	for _, w := range ak.Links {
		want[id{w.Consumer.Service, w.Consumer.File, w.Consumer.Line, w.Key, w.Provider.Service}] = Tier(w.Tier)
	}
	notYetBuilt := func(k id) bool { return strings.HasSuffix(k.file, ".ts") && strings.HasPrefix(k.key, "rpc:") }

	var fp, fn []string
	for k, tier := range got {
		wt, ok := want[k]
		if !ok {
			fp = append(fp, k.cs+" "+k.file+" "+k.key+" -> "+k.ps)
			continue
		}
		if wt != tier {
			t.Errorf("link %v: tier %s, answer key says %s", k, tier, wt)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok && !notYetBuilt(k) {
			fn = append(fn, k.cs+" "+k.file+" "+k.key+" -> "+k.ps)
		}
	}
	sort.Strings(fp)
	sort.Strings(fn)
	t.Logf("fleet: %d cross-service links found, %d in the answer key", len(got), len(want))
	if len(got) < len(want)-1 {
		t.Errorf("found %d links for a %d-link answer key", len(got), len(want))
	}
	if len(fp) > 0 {
		t.Errorf("links outside the answer key (precision): %v", fp)
	}
	if len(fn) > 0 {
		t.Errorf("answer-key links not found (recall): %v", fn)
	}
}
