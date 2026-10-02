package main

// loader.go — definitions and references at tier A (LB-Q-kyverno-rest): every
// policy, rule, PolicyException and CLI Test is one Kyverno's own loading
// code accepts, used offline as a Go library (github.com/kyverno/kyverno
// v1.18.1, the release the kyverno CLI 1.18.1 is built from):
// cmd/cli/kubectl-kyverno/policy.Load (kubectl-validate against Kyverno's
// CRD schemas and the built-in API, then the typed conversion `kyverno apply`
// and `kyverno test` run on) and cmd/cli/kubectl-kyverno/test.LoadTest (the
// strict decode `kyverno test` runs on). Names, rules and references are read
// off the typed objects; gopkg.in/yaml.v3 gives only each entity's line.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// testSpec is a loaded Test's entity and the files it names.
type testSpec struct {
	entity     string
	policies   []string
	exceptions []string
	resources  []string
}

// exceptionRef is a loaded exception's policyName, for the cross-file join.
type exceptionRef struct{ path, from, policy string }

type loaded struct {
	defs []def
	refs []ref
	// refused is every claimed file a loader refuses (the error); skipped
	// counts documents a loader skipped without failing the file.
	refused map[string]string
	skipped int
	// first is each file's first attested entity (by line).
	first map[string]string
	// attested is each attested entity's (file, entity) key, rules
	// included as (file, rule name).
	attested   map[[2]string]bool
	byName     map[string][]policyDecl
	exceptions []exceptionRef
	tests      map[string]testSpec
}

type kobject interface {
	GetName() string
	GetNamespace() string
}

// kindOf is a typed object's kind, from its TypeMeta.
func kindOf(o kobject) string {
	switch t := o.(type) {
	case interface{ GetObjectKind() schema.ObjectKind }:
		return t.GetObjectKind().GroupVersionKind().Kind
	case interface{ GetKind() string }:
		return t.GetKind()
	}
	return ""
}

// docLine is the document in f declaring kind/ns/name, and its line.
func docLine(f yamlFile, kind, ns, name string) (*yaml.Node, int) {
	for _, d := range f.docs {
		if scalar(get(d, "kind")) == kind && scalar(get(d, "metadata", "name")) == name &&
			scalar(get(d, "metadata", "namespace")) == ns {
			return d, d.Line
		}
	}
	return nil, 0
}

type celException struct {
	ns, name string
	refs     []string
}

func wrapCEL(in []*policiesv1beta1.PolicyException) []*celException {
	var out []*celException
	for _, e := range in {
		c := &celException{ns: e.GetNamespace(), name: e.GetName()}
		for _, r := range e.Spec.PolicyRefs {
			c.refs = append(c.refs, r.Name)
		}
		out = append(out, c)
	}
	return out
}

// transient reports a test load that failed on a read error.
func transient(tcs test.TestCases) bool {
	for _, tc := range tcs {
		if tc.Err != nil && strings.Contains(tc.Err.Error(), "not permitted") {
			return true
		}
	}
	return false
}

func loadEntities(subject string, files []yamlFile) loaded {
	lr := loaded{refused: map[string]string{}, first: map[string]string{}, attested: map[[2]string]bool{},
		byName: map[string][]policyDecl{}, tests: map[string]testSpec{}}
	refSeen := map[ref]bool{}
	addRef := func(r ref) {
		if r.To != "" && !refSeen[r] {
			refSeen[r] = true
			lr.refs = append(lr.refs, r)
		}
	}
	firstLine := map[string]int{}
	defSeen := map[def]bool{}
	addDef := func(d def) {
		if defSeen[d] {
			return // a CEL exception can sit in both loader fields
		}
		defSeen[d] = true
		lr.defs = append(lr.defs, d)
		lr.attested[[2]string{d.Path, d.Name}] = true
		if d.Kind == "rule" {
			return
		}
		if l, ok := firstLine[d.Path]; !ok || d.Line < l {
			firstLine[d.Path], lr.first[d.Path] = d.Line, d.Name
		}
	}
	for _, f := range files {
		if !claimed(f) {
			continue
		}
		abs := filepath.Join(subject, filepath.FromSlash(f.rel))
		hasTest, hasPolicy := false, false
		for _, d := range f.docs {
			if scalar(get(d, "kind")) == "Test" {
				hasTest = true
			} else {
				hasPolicy = true
			}
		}
		if hasTest {
			var tcs test.TestCases
			for attempt := 0; attempt < 5; attempt++ {
				if tcs = test.LoadTest(nil, abs); !transient(tcs) {
					break
				}
				time.Sleep(time.Second)
			}
			for _, tc := range tcs {
				if tc.Err != nil {
					lr.refused[f.rel] = tc.Err.Error()
					continue
				}
				if tc.Test == nil {
					continue
				}
				t := tc.Test
				name := t.GetName()
				if name == "" {
					name = t.Name
				}
				_, line := docLine(f, "Test", "", name)
				if name == "" || line == 0 {
					continue
				}
				ent := qualify("Test", "", name)
				addDef(def{f.rel, ent, line, "test"})
				for _, r := range t.Results {
					addRef(ref{f.rel, ent, r.Policy})
					addRef(ref{f.rel, ent, r.Rule})
				}
				lr.tests[f.rel] = testSpec{entity: ent, policies: t.Policies, exceptions: t.PolicyExceptions, resources: t.Resources}
			}
		}
		if !hasPolicy {
			continue
		}
		// A read on the external volume can fail transiently ("operation
		// not permitted"): retried, never taken as the loader's refusal.
		var res *policy.LoaderResults
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			if res, err = policy.Load(nil, "", abs); err == nil || !strings.Contains(err.Error(), "not permitted") {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			lr.refused[f.rel] = err.Error()
			continue
		}
		lr.skipped += len(res.NonFatalErrors)
		var objs []kobject
		for _, p := range res.Policies {
			objs = append(objs, p)
		}
		for _, p := range res.ValidatingPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.EnvoyPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.HTTPPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.ImageValidatingPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.GeneratingPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.DeletingPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.MutatingPolicies {
			objs = append(objs, p)
		}
		for _, p := range res.CleanupPolicies {
			objs = append(objs, p)
		}
		for _, o := range objs {
			kind, ns, name := kindOf(o), o.GetNamespace(), o.GetName()
			_, line := docLine(f, kind, ns, name)
			if line == 0 {
				continue
			}
			ent := qualify(kind, ns, name)
			addDef(def{f.rel, ent, line, "policy"})
			lr.byName[ent] = append(lr.byName[ent], policyDecl{f.rel, ent})
		}
		for _, p := range res.Policies {
			kind, ns, name := kindOf(p), p.GetNamespace(), p.GetName()
			d, line := docLine(f, kind, ns, name)
			if line == 0 {
				continue
			}
			ruleLines := map[string]int{}
			for _, r := range items(get(d, "spec", "rules")) {
				if rn := scalar(get(r, "name")); rn != "" && ruleLines[rn] == 0 {
					ruleLines[rn] = r.Line
				}
			}
			for _, r := range p.GetSpec().Rules {
				if l := ruleLines[r.Name]; l > 0 {
					addDef(def{f.rel, r.Name, l, "rule"})
				}
			}
		}
		for _, e := range res.PolicyExceptions {
			_, line := docLine(f, "PolicyException", e.GetNamespace(), e.GetName())
			if line == 0 {
				continue
			}
			ent := qualify("PolicyException", e.GetNamespace(), e.GetName())
			addDef(def{f.rel, ent, line, "policy"})
			for _, ex := range e.Spec.Exceptions {
				addRef(ref{f.rel, ent, ex.PolicyName})
				for _, rn := range ex.RuleNames {
					addRef(ref{f.rel, ent, rn})
				}
				if ex.PolicyName != "" {
					lr.exceptions = append(lr.exceptions, exceptionRef{f.rel, ent, ex.PolicyName})
				}
			}
		}
		for _, e := range append(wrapCEL(res.PolicyCELExceptions), wrapCEL(res.PolicyCelExceptions)...) {
			_, line := docLine(f, "PolicyException", e.ns, e.name)
			if line == 0 {
				continue
			}
			ent := qualify("PolicyException", e.ns, e.name)
			addDef(def{f.rel, ent, line, "policy"})
			for _, n := range e.refs {
				addRef(ref{f.rel, ent, n})
			}
		}
	}
	if len(lr.refused) > 0 || lr.skipped > 0 {
		var paths []string
		for p := range lr.refused {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		fmt.Fprintf(os.Stderr, "kyverno-oracle: Kyverno's loaders refuse %d claimed files (no entity from them) and skip %d documents\n", len(paths), lr.skipped)
		for _, p := range paths {
			fmt.Fprintf(os.Stderr, "  refused %s: %.240s\n", p, lr.refused[p])
		}
	}
	return lr
}
