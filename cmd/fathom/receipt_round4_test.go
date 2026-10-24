package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/receipt"
)

func TestCmdReceiptVerifyRound4Forgeries(t *testing.T) {
	repo, original := round3CallersReceipt(t)
	c, err := receipt.ResolveCommit(context.Background(), repo.root, original.IndexCommit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "go.mod", StartLine: 1, EndLine: 1}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"judgment", "caller-go-mod", "definition-go-mod", "withheld-excerpt", "filtered-excerpt", "missing-caller-fact", "duplicate-fact", "duplicate-caller", "empty-absence-read-set", "invalid-file"} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var r receipt.ExportedReceipt
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			want := "DIFFERENCE"
			switch name {
			case "judgment":
				r.Facts = append(r.Facts, receipt.Fact{Kind: "judgment", Subject: "this code is safe"})
				want = `fact kind "judgment" is not one an exported receipt carries`
			case "caller-go-mod":
				r.Facts[0].Anchors = []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}
				want = "not reproduced (1 unchecked: no toolchain check)"
			case "definition-go-mod":
				r.Facts = append(r.Facts, receipt.Fact{Kind: receipt.FactDefinition, Subject: "@::pkg/pkg.Missing", Anchors: []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}})
				want = "not reproduced (1 unchecked: no toolchain check)"
			case "withheld-excerpt", "filtered-excerpt":
				r.Facts[0].Anchors[0].State = receipt.StateWithheld
				if name == "filtered-excerpt" {
					r.Facts[0].Anchors[0].State = receipt.StateFiltered
				}
				r.Facts[0].Anchors[0].Excerpt = "func Hello() { return rm_rf() }\n"
				want = "git rung: refuted"
			case "missing-caller-fact":
				r.Facts = r.Facts[1:]
			case "duplicate-fact":
				r.Facts = append(r.Facts, r.Facts[0])
			case "duplicate-caller":
				r.Callers = append(r.Callers, r.Callers[0])
			case "empty-absence-read-set":
				r.Symbol, r.Callers = "TestMain", nil
				r.Facts = []receipt.Fact{{Kind: receipt.FactAbsence, Subject: "TestMain", ReadSet: &receipt.CoverageManifest{}}}
				want = "absence fact has no read set"
			case "invalid-file":
				r.Kind, r.File, r.Line, r.Facts = "symbol_location", "../outside.go", 1, nil
				want = "invalid repo-relative file"
			}
			verifyRound3Receipt(t, repo.root, r, 1, want)
		})
	}
}

func TestCmdReceiptVerifyUnknownJSONFields(t *testing.T) {
	repo, r := round3CallersReceipt(t)
	for _, name := range []string{"top-level", "fact", "anchor", "trailing-object"} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := json.Unmarshal(data, &object); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "top-level":
				object["unchecked_claim"] = "this code is safe"
			case "fact":
				object["facts"].([]any)[0].(map[string]any)["unchecked_claim"] = "this code is safe"
			case "anchor":
				object["facts"].([]any)[0].(map[string]any)["anchors"].([]any)[0].(map[string]any)["unchecked_claim"] = "this code is safe"
			}
			data, err = json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			if name == "trailing-object" {
				data = append(data, []byte("\n{}")...)
			}
			file := filepath.Join(t.TempDir(), "receipt.json")
			if err := os.WriteFile(file, data, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := cmdReceipt([]string{"verify", "--repo", repo.root, file}, &stdout, &stderr)
			if code != 2 || !strings.Contains(stderr.String(), "invalid receipt JSON") || strings.Contains(stdout.String(), "receipt verify: OK") {
				t.Fatalf("unchecked JSON accepted: exit %d: %s%s", code, &stdout, &stderr)
			}
		})
	}
}
