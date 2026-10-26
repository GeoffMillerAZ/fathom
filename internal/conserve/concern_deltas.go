package conserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"gopkg.in/yaml.v3"
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

// ConcernSkipped names a file excluded because its YAML could not decode.
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
	obs, err := in.Store.QueryObservations(ctx, in.Service, "")
	if err != nil {
		return ConcernDeltaReport{}, err
	}
	byFile := map[string][]core.SymbolNode{}
	for _, o := range obs {
		if o.SourceKind == core.KindK8sFact {
			byFile[o.SourceRef] = append(byFile[o.SourceRef], core.SymbolNode{Name: o.Symbol, Kind: o.SourceKind, FilePath: o.SourceRef, Line: o.Line, Signature: o.Claim})
		}
	}
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
	x := extractor.NewK8sExtractor()
	for _, p := range sorted {
		if err := ctx.Err(); err != nil {
			return ConcernDeltaReport{}, err
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".json":
		default:
			continue
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
		// Status is scoped to files we read, so unrelated dirty files do not
		// downgrade an otherwise exact manifest comparison. Includes staged edits.
		dirty, err := gitro.Output(ctx, in.Root, "status", "--porcelain", "--untracked-files=all", "--", p)
		if err != nil {
			return ConcernDeltaReport{}, err
		}
		if len(dirty) > 0 {
			rep.Basis.State = "uncommitted"
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
		var before []core.SymbolNode
		if baseOK {
			before, err = x.Extract(in.Service, p, base)
			if err != nil {
				return ConcernDeltaReport{}, err
			}
		}
		baseFacts, headFacts := concernFacts(before), concernFacts(byFile[p])
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
				d.BaseAnchor = fmt.Sprintf("%s:%d", p, b.Line)
			} else {
				d.Change = "added"
			}
			if hok {
				v := h.Signature
				d.After = &v
				d.HeadAnchor = fmt.Sprintf("%s:%d", p, h.Line)
			} else {
				d.Change = "removed"
			}
			rep.Deltas = append(rep.Deltas, d)
		}
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

func concernFacts(nodes []core.SymbolNode) map[string]core.SymbolNode {
	out := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == core.KindK8sFact {
			out[n.Name] = n
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
