package fusion

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// terraformXrefEntity identifies one labelled Terraform definition.
type terraformXrefEntity struct{ file, name string }

type terraformOutputReason uint8

const (
	terraformOutputResolved terraformOutputReason = iota
	terraformOutputMissingCall
	terraformOutputAmbiguousCall
	terraformOutputNonlocalSource
	terraformOutputMissingOutput
	terraformOutputAmbiguousOutput
)

// Each attempted intent enters exactly one outcome bucket.
type terraformOutputStats struct {
	Intents, Resolved, MissingCall, AmbiguousCall, NonlocalSource, MissingOutput, AmbiguousOutput int
}

func (s *terraformOutputStats) record(reason terraformOutputReason) {
	s.Intents++
	switch reason {
	case terraformOutputResolved:
		s.Resolved++
	case terraformOutputMissingCall:
		s.MissingCall++
	case terraformOutputAmbiguousCall:
		s.AmbiguousCall++
	case terraformOutputNonlocalSource:
		s.NonlocalSource++
	case terraformOutputMissingOutput:
		s.MissingOutput++
	case terraformOutputAmbiguousOutput:
		s.AmbiguousOutput++
	}
}

func (s terraformOutputStats) unresolved() int { return s.Intents - s.Resolved }

// resolveTerraformModuleOutputs builds a bounded two-hop join. One call in the
// caller's directory must name one local child directory, and one output in
// that child directory must match. Duplicate definitions stay unresolved.
func resolveTerraformModuleOutputs(files []DocFile) func(extractor.DocCrossFileRef) (terraformXrefEntity, terraformOutputReason) {
	// Keep file provenance beside every label without relying on map iteration.
	type candidate struct {
		file string
		set  extractor.DocLabelSet
	}
	type nodeKey struct{ file, name string }
	type exactNodeKey struct {
		file, name string
		line       int
		kind       core.SourceKind
	}
	nodeCount := map[nodeKey]int{}
	nodes := map[exactNodeKey]bool{}
	for _, f := range files {
		if path.Ext(f.Path) != ".tf" {
			continue
		}
		for _, n := range f.Result.Nodes {
			k := nodeKey{f.Path, n.Name}
			nodeCount[k]++
			nodes[exactNodeKey{f.Path, n.Name, n.Line, n.Kind}] = true
		}
	}
	index := map[string]map[string][]candidate{}
	type labelKey struct {
		scope, address, file string
		line                 int
	}
	seenLabel := map[labelKey]bool{}
	// A same-file duplicate address cannot be represented by doc-xref's
	// file/name symbol key. Refuse it even when only one node has the right kind.
	collisions := map[string]map[string]bool{}
	for _, f := range files {
		if path.Ext(f.Path) != ".tf" {
			continue
		}
		for _, ls := range f.Result.Labels {
			address := ls.Labels["terraform.address"]
			if address == "" || ls.Entity != address {
				continue
			}
			var kind core.SourceKind
			switch {
			case strings.HasPrefix(address, "module."):
				kind = core.KindTerraformModule
			case strings.HasPrefix(address, "output."):
				kind = core.KindTerraformOutput
			default:
				continue
			}
			if !nodes[exactNodeKey{f.Path, ls.Entity, ls.Line, kind}] {
				continue
			}
			if nodeCount[nodeKey{f.Path, ls.Entity}] > 1 {
				if collisions[ls.Scope] == nil {
					collisions[ls.Scope] = map[string]bool{}
				}
				collisions[ls.Scope][address] = true
				continue
			}
			if index[ls.Scope] == nil {
				index[ls.Scope] = map[string][]candidate{}
			}
			// Repeated identical labels describe one declaration, not two calls.
			k := labelKey{ls.Scope, address, f.Path, ls.Line}
			if seenLabel[k] {
				continue
			}
			seenLabel[k] = true
			index[ls.Scope][address] = append(index[ls.Scope][address], candidate{f.Path, ls})
		}
	}
	return func(ref extractor.DocCrossFileRef) (terraformXrefEntity, terraformOutputReason) {
		if ref.Strategy != "terraform-module-output" || ref.Edge != "module-output" || !strings.HasPrefix(ref.Scope, "terraform:") {
			return terraformXrefEntity{}, terraformOutputMissingCall
		}
		call := ref.Labels["terraform.module_call"]
		output := ref.Labels["terraform.output"]
		if !strings.HasPrefix(call, "module.") || !strings.HasPrefix(output, "output.") {
			return terraformXrefEntity{}, terraformOutputMissingCall
		}
		calls := index[ref.Scope][call]
		if collisions[ref.Scope][call] {
			return terraformXrefEntity{}, terraformOutputAmbiguousCall
		}
		if len(calls) == 0 {
			return terraformXrefEntity{}, terraformOutputMissingCall
		}
		if len(calls) > 1 {
			return terraformXrefEntity{}, terraformOutputAmbiguousCall
		}
		child := calls[0].set.Labels["terraform.local_source_dir"]
		if child == "" {
			return terraformXrefEntity{}, terraformOutputNonlocalSource
		}
		outputScope := "terraform:" + child
		outputs := index[outputScope][output]
		if collisions[outputScope][output] {
			return terraformXrefEntity{}, terraformOutputAmbiguousOutput
		}
		if len(outputs) == 0 {
			return terraformXrefEntity{}, terraformOutputMissingOutput
		}
		if len(outputs) > 1 {
			return terraformXrefEntity{}, terraformOutputAmbiguousOutput
		}
		return terraformXrefEntity{outputs[0].file, outputs[0].set.Entity}, terraformOutputResolved
	}
}
