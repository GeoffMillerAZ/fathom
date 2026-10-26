package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixtures(t *testing.T) (CatalogInput, Scope, Subjects) {
	t.Helper()
	var c CatalogInput
	var s Scope
	var b Subjects
	for _, v := range []struct {
		name string
		out  any
	}{{"catalog.json", &c}, {"scope.json", &s}, {"subjects.json", &b}} {
		data, err := os.ReadFile(filepath.Join("fixtures", v.name))
		if err != nil {
			t.Fatal(err)
		}
		if err = strictJSON(data, v.out); err != nil {
			t.Fatal(err)
		}
	}
	return c, s, b
}
func clone[T any](t *testing.T, x T) T {
	t.Helper()
	b, e := json.Marshal(x)
	if e != nil {
		t.Fatal(e)
	}
	var y T
	if e = json.Unmarshal(b, &y); e != nil {
		t.Fatal(e)
	}
	return y
}
func at() time.Time { return time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC) }
func bindingFor(t *testing.T, r Receipt, id, key string) BindingReceipt {
	t.Helper()
	for _, b := range r.Bindings {
		if b.SubjectID == id && b.StableKey == key {
			return b
		}
	}
	t.Fatalf("missing binding %s/%s", id, key)
	return BindingReceipt{}
}
func assess(t *testing.T, c CatalogInput, s Scope, b Subjects) (Receipt, int, error) {
	t.Helper()
	return evaluate(c, s, b, at(), map[string]string{"catalog": "fixture", "scope": "fixture", "subjects": "fixture"})
}

func TestSyntheticPositiveAndOrderInvariance(t *testing.T) {
	c, s, b := fixtures(t)
	a, code, err := assess(t, c, s, b)
	if err != nil || code != 0 || a.Status != "pass" || len(a.Bindings) != 6 || len(a.Subjects) != 2 {
		t.Fatalf("positive: %+v %d %v", a, code, err)
	}
	c.Spec.Layers[0].Nodes[0].Children[0], c.Spec.Layers[0].Nodes[0].Children[2] = c.Spec.Layers[0].Nodes[0].Children[2], c.Spec.Layers[0].Nodes[0].Children[0]
	c.Practices[0], c.Practices[2] = c.Practices[2], c.Practices[0]
	s.Criteria[0], s.Criteria[2] = s.Criteria[2], s.Criteria[0]
	s.Subjects[0], s.Subjects[1] = s.Subjects[1], s.Subjects[0]
	b.Subjects[0], b.Subjects[1] = b.Subjects[1], b.Subjects[0]
	for i := range b.Subjects {
		x := b.Subjects[i].Checks
		x[0], x[2] = x[2], x[0]
	}
	z, code, err := assess(t, c, s, b)
	if err != nil || code != 0 {
		t.Fatalf("reorder: %d %v", code, err)
	}
	if !reflect.DeepEqual(a, z) {
		aa, _ := json.MarshalIndent(a, "", "  ")
		zz, _ := json.MarshalIndent(z, "", "  ")
		t.Fatalf("equivalent input order changed receipt\n%s\n%s", aa, zz)
	}
}

func TestPathMovePreservesDefinitionPin(t *testing.T) {
	c, s, b := fixtures(t)
	before, _, _ := assess(t, c, s, b)
	for i := range c.Spec.Layers[0].Nodes[0].Children {
		if c.Spec.Layers[0].Nodes[0].Children[i].StableKey == "review.evidence-integrity" {
			c.Spec.Layers[0].Nodes[0].Children[i].Slug = "integrity-moved"
		}
	}
	c.Practices[0].Path = "review/integrity-moved"
	a, code, err := assess(t, c, s, b)
	if err != nil || code != 0 {
		t.Fatalf("move: %d %v", code, err)
	}
	if a.Bindings[1].DefinitionDigest != before.Bindings[1].DefinitionDigest {
		t.Fatal("definition pin changed on path move")
	}
}

func TestSubjectAndScopeChangeSealedIdentities(t *testing.T) {
	c, s, b := fixtures(t)
	base, code, err := assess(t, c, s, b)
	if err != nil || code != 0 {
		t.Fatalf("baseline: %d %v", code, err)
	}
	x := 3
	b.Subjects[0].Checks[1].Work.Steps = &x
	changed, code, err := assess(t, c, s, b)
	if err != nil || code != 0 {
		t.Fatalf("changed steps: %d %v", code, err)
	}
	if base.Subjects[0].Assessment.SubjectDigest == changed.Subjects[0].Assessment.SubjectDigest || bindingFor(t, base, "review-alpha", "review.bounded-work").PreparedDigest == bindingFor(t, changed, "review-alpha", "review.bounded-work").PreparedDigest {
		t.Fatal("subject check change left identity unchanged")
	}
	s.Subjects[0].AllowedActions = append(s.Subjects[0].AllowedActions, "archive-local-report")
	scoped, code, err := assess(t, c, s, b)
	if err != nil || code != 0 {
		t.Fatalf("changed scope: %d %v", code, err)
	}
	if bindingFor(t, changed, "review-alpha", "review.bounded-work").PreparedDigest == bindingFor(t, scoped, "review-alpha", "review.bounded-work").PreparedDigest {
		t.Fatal("scope action change left prepared packet unchanged")
	}
}

func TestPlantedNegativeProperties(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*CatalogInput, *Scope, *Subjects)
		want   int
	}{
		{"omitted subject", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects = b.Subjects[:1] }, 2},
		{"omitted criterion", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks = b.Subjects[0].Checks[:2] }, 2},
		{"changed same version", func(c *CatalogInput, _ *Scope, _ *Subjects) { c.Practices[0].Definition += " changed" }, 2},
		{"duplicate subject", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects = append(b.Subjects, b.Subjects[0]) }, 2},
		{"foreign subject", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].ID = "foreign" }, 2},
		{"invalid key shared by scope and subject", func(_ *CatalogInput, s *Scope, b *Subjects) {
			s.Subjects[0].ID = "review alpha"
			b.Subjects[0].ID = "review alpha"
		}, 2},
		{"duplicate check", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			b.Subjects[0].Checks = append(b.Subjects[0].Checks, b.Subjects[0].Checks[0])
		}, 2},
		{"foreign criterion", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks[0].StableKey = "foreign" }, 2},
		{"unsupported kind", func(c *CatalogInput, _ *Scope, _ *Subjects) { c.Practices[0].Kind = "other" }, 2},
		{"hash mismatch", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks[0].Evidence.Text += "corrupt" }, 1},
		{"foreign subject evidence", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			b.Subjects[1].Checks[0].Evidence = b.Subjects[0].Checks[0].Evidence
		}, 1},
		{"unbounded work", func(_ *CatalogInput, _ *Scope, b *Subjects) { x := 4; b.Subjects[0].Checks[1].Work.Steps = &x }, 1},
		{"changed trusted floor", func(c *CatalogInput, _ *Scope, _ *Subjects) { x := 4; c.Practices[1].MaxSteps = &x }, 2},
		{"changed remedy", func(c *CatalogInput, _ *Scope, _ *Subjects) { c.Practices[1].Remedy += " changed" }, 2},
		{"changed check kind", func(c *CatalogInput, _ *Scope, _ *Subjects) {
			c.Practices[1].Kind = "effect-scope"
			c.Practices[1].MaxSteps = nil
		}, 2},
		{"undeclared effect", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			a := []string{"network-write"}
			b.Subjects[0].Checks[2].RequestedActions = &a
		}, 1},
		{"expired", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks[0].Evidence.ValidUntil = at() }, 2},
		{"revision mismatch", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Revision = "rev-2" }, 2},
		{"extraneous work", func(_ *CatalogInput, _ *Scope, b *Subjects) { x := 0; b.Subjects[0].Checks[0].Work = &Work{Steps: &x} }, 2},
		{"extraneous requested actions", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			a := []string{"write-local-report"}
			b.Subjects[0].Checks[0].RequestedActions = &a
		}, 2},
		{"missing definition", func(c *CatalogInput, _ *Scope, _ *Subjects) { c.Practices = c.Practices[:2] }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s, b := fixtures(t)
			tc.mutate(&c, &s, &b)
			r, code, err := assess(t, c, s, b)
			if code != tc.want {
				t.Fatalf("want %d got %d (%s, %v)", tc.want, code, r.Status, err)
			}
		})
	}
}

func writeJSON(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(dir, name)
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func cliArgs(t *testing.T) ([]string, string, string, string) {
	t.Helper()
	c, s, b := fixtures(t)
	d := t.TempDir()
	cp := writeJSON(t, d, "catalog.json", c)
	sp := writeJSON(t, d, "scope.json", s)
	bp := writeJSON(t, d, "subjects.json", b)
	return []string{"--catalog", cp, "--scope", sp, "--subjects", bp, "--at", "2026-10-23T00:00:00Z", "--format", "json"}, cp, sp, bp
}
func TestCLINegatives(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, args []string, cp, sp, bp string)
	}{
		{"missing file", func(_ *testing.T, a []string, _, _, bp string) { _ = os.Remove(bp) }},
		{"oversized file", func(t *testing.T, _ []string, cp, _, _ string) {
			if e := os.WriteFile(cp, bytes.Repeat([]byte("x"), maxFile+1), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"malformed", func(t *testing.T, _ []string, cp, _, _ string) {
			if e := os.WriteFile(cp, []byte("{"), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"duplicate key", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(`{"version":1,"version":1}`), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"unknown field", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(`{"version":1,"criteria":[],"subjects":[],"other":1}`), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"null", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(`{"version":1,"criteria":null,"subjects":[]}`), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"trailing", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(`{} {}`), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"invalid surrogate", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(`{"version":1,"criteria":[],"subjects":[],"name":"\ud800"}`), 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"subject-supplied limit", func(t *testing.T, _ []string, _, _, bp string) {
			b, e := os.ReadFile(bp)
			if e != nil {
				t.Fatal(e)
			}
			b = bytes.Replace(b, []byte(`"steps":2`), []byte(`"steps":2,"max_steps":9999`), 1)
			if e = os.WriteFile(bp, b, 0600); e != nil {
				t.Fatal(e)
			}
		}},
		{"depth", func(t *testing.T, _ []string, _, sp, _ string) {
			if e := os.WriteFile(sp, []byte(strings.Repeat("[", 30)+strings.Repeat("]", 30)), 0600); e != nil {
				t.Fatal(e)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, cp, sp, bp := cliArgs(t)
			tc.change(t, a, cp, sp, bp)
			var out bytes.Buffer
			if code := run(a, &out); code != 2 {
				t.Fatalf("want 2, got %d: %s", code, out.String())
			}
		})
	}
}

func TestCLIPlantedNegativeCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*CatalogInput, *Scope, *Subjects)
		want   int
	}{
		{"missing whole subject", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects = b.Subjects[:1] }, 2},
		{"missing criterion", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks = b.Subjects[0].Checks[:2] }, 2},
		{"changed definition", func(c *CatalogInput, _ *Scope, _ *Subjects) { c.Practices[0].Definition += " altered" }, 2},
		{"hash mismatch", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks[0].Evidence.Text += " altered" }, 1},
		{"foreign subject evidence", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			b.Subjects[1].Checks[0].Evidence = b.Subjects[0].Checks[0].Evidence
		}, 1},
		{"unbounded work", func(_ *CatalogInput, _ *Scope, b *Subjects) { x := 4; b.Subjects[0].Checks[1].Work.Steps = &x }, 1},
		{"undeclared effect", func(_ *CatalogInput, _ *Scope, b *Subjects) {
			a := []string{"publish"}
			b.Subjects[0].Checks[2].RequestedActions = &a
		}, 1},
		{"expired", func(_ *CatalogInput, _ *Scope, b *Subjects) { b.Subjects[0].Checks[0].Evidence.ValidUntil = at() }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s, b := fixtures(t)
			tc.mutate(&c, &s, &b)
			d := t.TempDir()
			a := []string{"--catalog", writeJSON(t, d, "catalog.json", c), "--scope", writeJSON(t, d, "scope.json", s), "--subjects", writeJSON(t, d, "subjects.json", b), "--at", "2026-10-23T00:00:00Z", "--format", "json"}
			var out bytes.Buffer
			code := run(a, &out)
			if code != tc.want {
				t.Fatalf("want %d got %d: %s", tc.want, code, out.String())
			}
			var receipt Receipt
			if err := json.Unmarshal(out.Bytes(), &receipt); err != nil {
				t.Fatalf("missing JSON receipt: %v: %s", err, out.String())
			}
			if tc.want == 2 && len(receipt.CoverageGaps) == 0 {
				t.Fatal("unknown case lacks coverage gap")
			}
		})
	}
}

type brokenWriter struct{}

func TestCLIRejectsCueInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Subjects)
	}{
		{"fractional observation time", func(b *Subjects) {
			b.Subjects[0].Checks[0].Evidence.ObservedAt = b.Subjects[0].Checks[0].Evidence.ObservedAt.Add(time.Millisecond)
		}},
		{"negative work steps", func(b *Subjects) {
			steps := -1
			b.Subjects[0].Checks[1].Work.Steps = &steps
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s, b := fixtures(t)
			tc.mutate(&b)
			dir := t.TempDir()
			args := []string{"--catalog", writeJSON(t, dir, "catalog.json", c), "--scope", writeJSON(t, dir, "scope.json", s), "--subjects", writeJSON(t, dir, "subjects.json", b), "--at", "2026-10-23T00:00:00Z", "--format", "json"}
			var out bytes.Buffer
			if code := run(args, &out); code != 2 {
				t.Fatalf("CUE-invalid input must be invalid: exit %d: %s", code, out.String())
			}
		})
	}
}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }
func TestCLIWriterFailure(t *testing.T) {
	a, _, _, _ := cliArgs(t)
	if code := run(a, brokenWriter{}); code != 2 {
		t.Fatalf("writer failure exit %d", code)
	}
}
