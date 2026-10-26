package conserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// ConcernBasis states whether the index and the requested commit agree.
// Uncommitted changes to a read file take precedence over commit skew.
type ConcernBasis struct {
	State       string `json:"state"`
	IndexCommit string `json:"index_commit"`
	HeadCommit  string `json:"head_commit"`
}

// ConcernDelta is information about one K8sFact, never a conservation loss.
// Key is the extractor's fact key (including the container-list prefix).
// FactVersion v1 names the current K8sFact key/value contract, not apiVersion.
// A missing side has a null value and an empty anchor, never an invented line.
type ConcernDelta struct {
	Workload    string          `json:"workload"`
	Namespace   string          `json:"namespace"`
	Container   string          `json:"container"`
	Key         string          `json:"key"`
	FactKind    core.SourceKind `json:"fact_kind"`
	FactVersion string          `json:"fact_version"`
	Change      string          `json:"change"`
	Before      *string         `json:"before"`
	After       *string         `json:"after"`
	BaseAnchor  string          `json:"base_anchor"`
	HeadAnchor  string          `json:"head_anchor"`
	Basis       ConcernBasis    `json:"basis"`
	HeadCommit  string          `json:"head_commit"`
}

// ConcernSkipped names a file excluded by ingest dispatch or YAML decoding.
// Commit identifies which side was unreadable, even when both sides failed.
type ConcernSkipped struct {
	Path   string `json:"path"`
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}

// ConcernDeltaReport is the deterministic CLI-only concern-delta file.
// Nothing in this report participates in a plane verdict or strict exit.
type ConcernDeltaReport struct {
	Schema  string           `json:"schema"`
	Service string           `json:"service"`
	Base    string           `json:"base"`
	Head    string           `json:"head"`
	Basis   ConcernBasis     `json:"basis"`
	Deltas  []ConcernDelta   `json:"deltas"`
	Skipped []ConcernSkipped `json:"skipped"`
}

// ConcernDeltas compares changed manifests' stored head facts with facts
// extracted from BaseFile by the same Kubernetes extractor used at ingest.
// It runs only when the CLI requests a file. It never writes to the store,
// registers a plane, or re-extracts head values to substitute for the index.
func ConcernDeltas(ctx context.Context, in Input) (ConcernDeltaReport, error) {
	if in.Store == nil {
		return ConcernDeltaReport{}, fmt.Errorf("concern deltas require a store")
	}
	head, err := resolveRef(ctx, in.Root, "HEAD")
	if err != nil {
		return ConcernDeltaReport{}, fmt.Errorf("concern deltas HEAD: %w", err)
	}
	meta, found, err := in.Store.GetServiceMeta(ctx, in.Service)
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	if !found || meta.IndexedCommit == "" {
		return ConcernDeltaReport{}, fmt.Errorf("no index metadata for service %q; ingest before computing concern deltas", in.Service)
	}
	basis := ConcernBasis{State: "differs", IndexCommit: meta.IndexedCommit, HeadCommit: head}
	if meta.IndexedCommit == head {
		basis.State = "exact"
	}
	rep := ConcernDeltaReport{Schema: "fathom.concern-deltas/v1", Service: in.Service, Base: in.Base, Head: head, Basis: basis, Deltas: []ConcernDelta{}, Skipped: []ConcernSkipped{}}
	// Include the committed range even if a dirty edit happens to restore Base.
	changed, err := gitro.Output(ctx, in.Root, "diff", "--no-renames", "--relative", "--name-only", "-z", in.Base, head, "--")
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	paths := map[string]bool{}
	for _, p := range strings.Split(string(changed), "\x00") {
		if p != "" {
			paths[p] = true
		}
	}
	for _, p := range in.ChangedFiles {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	// K8sExtractor labels packages by file stem. Query just the touched
	// packages, then retain only the requested sources (RT-81/RT-85).
	packages := map[string]bool{}
	for _, p := range sorted {
		switch filepath.Ext(p) {
		case ".yaml", ".yml":
			packages[strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))] = true
		}
	}
	pkgs := make([]string, 0, len(packages))
	for pkg := range packages {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	obs, err := in.Store.PackageObservations(ctx, in.Service, pkgs)
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	byFile := map[string][]core.SymbolNode{}
	headK8s := map[string]bool{}
	for _, o := range obs {
		if !paths[o.SourceRef] {
			continue
		}
		if o.ToolKind == string(fusion.ToolK8sManifest) || o.ToolKind == string(fusion.ToolK8sCRD) {
			headK8s[o.SourceRef] = true
			if o.SourceKind == core.KindK8sFact {
				byFile[o.SourceRef] = append(byFile[o.SourceRef], core.SymbolNode{Name: o.Symbol, Kind: o.SourceKind, FilePath: o.SourceRef, Line: o.Line, Signature: o.Claim})
			}
		}
	}
	// Sibling classification needs only the base tree's path metadata, never
	// unrelated file content (notably Chart.yaml ancestors for Helm templates).
	basePaths, err := gitro.Output(ctx, in.Root, "ls-tree", "-r", "--name-only", "-z", in.Base)
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	projectIgnore, _, err := in.BaseFile(connector.IgnoreFileName)
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	projectConfig, _, err := in.BaseFile(projectdir.Config.Rel())
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	scopedPatterns, err := connector.ScopedIgnorePatterns(in.Service)
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	basePolicy := connector.NewBlobCorpusPolicy(projectIgnore, projectConfig, scopedPatterns)
	var refs []core.SourceRef
	for _, p := range strings.Split(string(basePaths), "\x00") {
		if p != "" && basePolicy.Exclusion(p) == "" {
			refs = append(refs, core.SourceRef{Service: in.Service, Path: p})
		}
	}
	x := fusion.New(nil, nil, fusion.Config{})
	defer x.Close()
	var allBase, allHead []core.SymbolNode
	for _, p := range sorted {
		if err := ctx.Err(); err != nil {
			return ConcernDeltaReport{}, err
		}
		base, baseOK, err := in.BaseFile(p)
		if err != nil {
			return ConcernDeltaReport{}, fmt.Errorf("concern deltas base %s: %w", p, err)
		}
		committed, headOK, err := gitShowFile(ctx, in.Root, head, p)
		if err != nil {
			return ConcernDeltaReport{}, err
		}
		// An untracked output JSON file has no input on either side. Do not
		// let it downgrade the next run's basis merely by being present.
		if !baseOK && !headOK && len(byFile[p]) == 0 {
			continue
		}
		skip := false
		for _, side := range []struct {
			data   []byte
			ok     bool
			commit string
		}{{base, baseOK, in.Base}, {committed, headOK, head}} {
			if side.ok && !concernYAMLDecodes(side.data) {
				rep.Skipped = append(rep.Skipped, ConcernSkipped{Path: p, Commit: side.commit, Reason: "YAML does not decode; no deltas computed for this file"})
				skip = true
			}
		}
		if skip {
			continue
		}
		reason, excluded, err := connector.ExcludedFromCorpus(in.Root, in.Service, p)
		if err != nil {
			return ConcernDeltaReport{}, err
		}
		if !excluded {
			reason = ""
		}
		var before []core.SymbolNode
		if baseOK {
			extracted, err := x.ExtractKubernetesBlob(in.Service, refs, p, base, basePolicy)
			if err != nil {
				return ConcernDeltaReport{}, err
			}
			if extracted.Skipped != "" {
				rep.Skipped = append(rep.Skipped, ConcernSkipped{Path: p, Commit: in.Base, Reason: extracted.Skipped})
				skip = true
			}
			before = extracted.Nodes
		}
		if headOK && (reason != "" || !headK8s[p]) {
			if reason == "" {
				reason = "stored observations do not identify Kubernetes (tool_kind k8s or k8s-crd required)"
			}
			rep.Skipped = append(rep.Skipped, ConcernSkipped{Path: p, Commit: head, Reason: reason})
			skip = true
		}
		if skip {
			continue
		}
		// Status is scoped to files we read, so unrelated dirty files do not
		// downgrade an otherwise exact manifest comparison. Includes staged edits.
		dirty, err := gitro.Output(ctx, in.Root, "status", "--porcelain", "--untracked-files=all", "--", p)
		if err != nil {
			return ConcernDeltaReport{}, err
		}
		if len(dirty) > 0 {
			rep.Basis.State = "uncommitted"
		}
		// Metadata alone cannot prove an index represents a commit: ingest may
		// have read a dirty worktree that was subsequently restored. The
		// epoch-salted digest also refuses values from an older extractor.
		fingerprint, stored, err := in.Store.Fingerprint(ctx, in.Service, p)
		if err != nil {
			return ConcernDeltaReport{}, err
		}
		if (headOK && (!stored || fingerprint != fusion.FingerprintHash(committed))) || (!headOK && stored) {
			rep.Basis.State = "uncommitted"
		}
		allBase = append(allBase, before...)
		allHead = append(allHead, byFile[p]...)
	}
	duplicates := concernDuplicateFacts(allBase, allHead)
	baseFacts, headFacts := concernFacts(allBase, duplicates), concernFacts(allHead, duplicates)
	keys := map[string]bool{}
	for k := range baseFacts {
		keys[k] = true
	}
	for k := range headFacts {
		keys[k] = true
	}
	for k := range keys {
		b, bok := baseFacts[k]
		h, hok := headFacts[k]
		if bok && hok && b.Signature == h.Signature {
			continue
		}
		n := h
		if !hok {
			n = b
		}
		subject, key, _ := strings.Cut(n.Name, "#")
		parts := strings.Split(subject, "/")
		if len(parts) < 2 {
			return ConcernDeltaReport{}, fmt.Errorf("invalid K8sFact subject %q", subject)
		}
		namespace := ""
		if len(parts) > 2 {
			namespace = parts[1]
		}
		d := ConcernDelta{Workload: parts[0] + "/" + parts[len(parts)-1], Namespace: namespace, Key: key, FactKind: core.KindK8sFact, FactVersion: "v1", Change: "changed", HeadCommit: head}
		fields := strings.Split(key, ".")
		if len(fields) > 2 && (fields[0] == "containers" || fields[0] == "initContainers") {
			d.Container = fields[1]
		}
		if bok {
			v := b.Signature
			d.Before = &v
			d.BaseAnchor = fmt.Sprintf("%s:%d", b.FilePath, b.Line)
		} else {
			d.Change = "added"
		}
		if hok {
			v := h.Signature
			d.After = &v
			d.HeadAnchor = fmt.Sprintf("%s:%d", h.FilePath, h.Line)
		} else {
			d.Change = "removed"
		}
		rep.Deltas = append(rep.Deltas, d)
	}
	for i := range rep.Deltas {
		rep.Deltas[i].Basis = rep.Basis
	}
	// Canonical byte order also resolves ties across files and namespaces.
	sort.Slice(rep.Deltas, func(i, j int) bool {
		a, b := rep.Deltas[i], rep.Deltas[j]
		ak, bk := []string{a.Workload, a.Namespace, a.Key, a.BaseAnchor, a.HeadAnchor, a.Change}, []string{b.Workload, b.Namespace, b.Key, b.BaseAnchor, b.HeadAnchor, b.Change}
		for n := range ak {
			if ak[n] != bk[n] {
				return ak[n] < bk[n]
			}
		}
		return false
	})
	sort.Slice(rep.Skipped, func(i, j int) bool {
		a, b := rep.Skipped[i], rep.Skipped[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Commit < b.Commit
	})
	return rep, nil
}

// Duplicate names use paths on BOTH sides if either side has more than one
// file for that name. Otherwise moves and document splits keep fact identity.
func concernDuplicateFacts(sides ...[]core.SymbolNode) map[string]bool {
	duplicates := map[string]bool{}
	for _, nodes := range sides {
		files := map[string]string{}
		for _, n := range nodes {
			if n.Kind != core.KindK8sFact {
				continue
			}
			if p, ok := files[n.Name]; ok && p != n.FilePath {
				duplicates[n.Name] = true
			}
			files[n.Name] = n.FilePath
		}
	}
	return duplicates
}

func concernFacts(nodes []core.SymbolNode, duplicates map[string]bool) map[string]core.SymbolNode {
	out := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == core.KindK8sFact {
			key := n.Name
			if duplicates[key] {
				key = n.FilePath + "\x00" + key
			}
			out[key] = n
		}
	}
	return out
}

// Conservative file-level decoding: a partially undecodable multi-document
// file is explicitly skipped rather than claiming its missing facts changed.
func concernYAMLDecodes(src []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var n yaml.Node
		err := dec.Decode(&n)
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		if !concernYAMLMapKeys(&n) {
			return false
		}
	}
}

// YAML nodes accept complex map keys (including unrendered Helm expressions)
// that cannot decode as Kubernetes' JSON maps. Repeated scalar keys remain
// valid here, matching the extractor's last-value-wins fallback.
func concernYAMLMapKeys(n *yaml.Node) bool {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Kind != yaml.ScalarNode {
				return false
			}
		}
	}
	for _, child := range n.Content {
		if !concernYAMLMapKeys(child) {
			return false
		}
	}
	return true
}
