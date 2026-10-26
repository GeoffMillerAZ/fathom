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
// that child directory must match. Override files merge in native load order;
// duplicate primary definitions stay unresolved.
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
	// Materialize once so repeated references never rescan a declaration set.
	type effective struct {
		candidate candidate
		child     string
		count     int
	}
	effectiveIndex := map[string]map[string]effective{}
	for scope, addresses := range index {
		effectiveIndex[scope] = map[string]effective{}
		for address, candidates := range addresses {
			var primary candidate
			var primaryCount int
			var firstOverride, lastSourceOverride candidate
			for _, c := range candidates {
				if !terraformOverrideFile(c.file) {
					primaryCount++
					primary = c
					continue
				}
				if firstOverride.file == "" || terraformOverrideLess(c.file, firstOverride.file) {
					firstOverride = c
				}
				if c.set.Labels["terraform.source_present"] == "true" && (lastSourceOverride.file == "" || terraformOverrideLess(lastSourceOverride.file, c.file)) {
					lastSourceOverride = c
				}
			}
			e := effective{candidate: primary, count: primaryCount}
			if primaryCount == 0 && firstOverride.file != "" {
				e.candidate = firstOverride
				e.count = 1
			}
			if strings.HasPrefix(address, "module.") {
				e.child = e.candidate.set.Labels["terraform.local_source_dir"]
				if lastSourceOverride.file != "" {
					e.child = lastSourceOverride.set.Labels["terraform.local_source_dir"]
				}
			}
			effectiveIndex[scope][address] = e
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
		callDef := effectiveIndex[ref.Scope][call]
		if collisions[ref.Scope][call] {
			return terraformXrefEntity{}, terraformOutputAmbiguousCall
		}
		if callDef.count == 0 {
			return terraformXrefEntity{}, terraformOutputMissingCall
		}
		if callDef.count > 1 {
			return terraformXrefEntity{}, terraformOutputAmbiguousCall
		}
		child := callDef.child
		if child == "" {
			return terraformXrefEntity{}, terraformOutputNonlocalSource
		}
		outputScope := "terraform:" + child
		outputDef := effectiveIndex[outputScope][output]
		if collisions[outputScope][output] {
			return terraformXrefEntity{}, terraformOutputAmbiguousOutput
		}
		if outputDef.count == 0 {
			return terraformXrefEntity{}, terraformOutputMissingOutput
		}
		if outputDef.count > 1 {
			return terraformXrefEntity{}, terraformOutputAmbiguousOutput
		}
		return terraformXrefEntity{outputDef.candidate.file, outputDef.candidate.set.Entity}, terraformOutputResolved
	}
}

func terraformOverrideFile(file string) bool {
	base := path.Base(file)
	return base == "override.tf" || strings.HasSuffix(base, "_override.tf")
}

// Terraform applies override.tf before the remaining override files in
// lexical filename order, regardless of the DocFile input order.
func terraformOverrideLess(a, b string) bool {
	aBase, bBase := path.Base(a), path.Base(b)
	if aBase == "override.tf" || bBase == "override.tf" {
		return aBase == "override.tf" && bBase != "override.tf"
	}
	return aBase < bBase
}
