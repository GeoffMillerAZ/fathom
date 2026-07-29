package modspec

// template.go — the module.cue scaffold `fathom module init` writes. Plain
// data, no package clause, no "#Module &" wrapper — the exact convention
// internal/semantic's decodePack doc comment states for a project-local
// layer pack's layer.cue, reused here for a module manifest.

import (
	"fmt"
	"sort"
	"strings"
)

// TemplateInput is the scaffolding input for Template — everything `fathom
// module init`'s flags collect before a watermark has been computed.
type TemplateInput struct {
	Name        string
	Description string
	Include     []string
	Exclude     []string
	Watermark   string
	TierProfile string // "" -> field omitted, schema default "engineering" applies
	License     string // "" -> field omitted, schema default "unspecified" applies
	CompiledBy  string // "" -> "deterministic" (module init never calls an LLM)
	CompiledAt  string // RFC3339, caller-injected clock (repo convention: never time.Now() inline)
}

// Template renders a well-formed module.cue instance from in — every field
// ValidateStrict's authoring gate requires is present and concrete, so the
// emitted file passes ValidateStrict unmodified (the `fathom module init`
// contract). recompile.allowed is always false and eval_evidence.evaled is
// always false: a freshly scaffolded module has compiled nothing yet, so
// naming a recompile prompt identity or eval evidence path would be a lie —
// `fathom module compile`/`eval` are what earn those fields, not init.
func Template(in TemplateInput) string {
	tierProfile := in.TierProfile
	if tierProfile == "" {
		tierProfile = "engineering"
	}
	license := in.License
	if license == "" {
		license = "unspecified"
	}
	compiledBy := in.CompiledBy
	if compiledBy == "" {
		compiledBy = "deterministic"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// module.cue — scaffolded by `fathom module init` (docs/spec/expert-module-v0.md §3).\n")
	fmt.Fprintf(&b, "// Validate with `fathom module doctor` or modspec.ValidateStrict; edit freely —\n")
	fmt.Fprintf(&b, "// every field below is a plain value, not a template placeholder.\n")
	fmt.Fprintf(&b, "name:        %q\n", in.Name)
	fmt.Fprintf(&b, "version:     \"v1\"\n")
	fmt.Fprintf(&b, "description: %q\n", in.Description)
	fmt.Fprintf(&b, "corpus: {\n")
	fmt.Fprintf(&b, "\tinclude: [%s]\n", quotedList(in.Include))
	if len(in.Exclude) > 0 {
		fmt.Fprintf(&b, "\texclude: [%s]\n", quotedList(in.Exclude))
	}
	fmt.Fprintf(&b, "}\n")
	fmt.Fprintf(&b, "watermark:    %q\n", in.Watermark)
	fmt.Fprintf(&b, "tier_profile: %q\n", tierProfile)
	fmt.Fprintf(&b, "eval_evidence: evaled: false\n")
	fmt.Fprintf(&b, "recompile: allowed: false\n")
	fmt.Fprintf(&b, "license: %q\n", license)
	fmt.Fprintf(&b, "provenance: {\n")
	fmt.Fprintf(&b, "\tcompiled_by: %q\n", compiledBy)
	fmt.Fprintf(&b, "\tcompiled_at: %q\n", in.CompiledAt)
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

func quotedList(ss []string) string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	quoted := make([]string, len(out))
	for i, s := range out {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
