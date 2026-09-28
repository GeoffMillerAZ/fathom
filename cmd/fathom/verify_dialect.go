package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/specdialect"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// dialectForPlan picks the spec dialect `fathom verify --plan <path>` reads
// path with (ADR-049, spec-interop design §3): the --dialect override when
// set, else the dialect that claims the path by its path and content shape
// (foreign dialects first, plan grammar v2 as the Markdown fallback). An
// unknown override, or a file nothing claims, is an error the caller turns
// into exit 2 — operational, never a gate verdict.
func dialectForPlan(reg *specdialect.Registry, path string, content []byte, override string) (specdialect.SpecDialect, error) {
	if override != "" {
		d, ok := reg.Lookup(override)
		if !ok {
			return nil, fmt.Errorf("--dialect %q: unknown dialect (want one of %s)", override, strings.Join(reg.IDs(), ", "))
		}
		return d, nil
	}
	d, ok := reg.ForFile(path, content)
	if !ok {
		return nil, fmt.Errorf("plan %q: no spec dialect claims it — fathom verify reads Markdown checklists (plan grammar v2, Spec Kit, OpenSpec); pass --dialect <%s> to choose one", path, strings.Join(reg.IDs(), "|"))
	}
	return d, nil
}

// parsePlanWith parses one plan file's bytes with d for service: the
// native grammar exactly as before, a foreign dialect through the port
// (with the sibling files it names). pf is the path the bytes came from.
func parsePlanWith(d specdialect.SpecDialect, service, pf string, raw []byte) (verify.Plan, error) {
	if d.ID() == specdialect.NativeID {
		return verify.ParseMarkdownPlan(service, raw), nil
	}
	file := pf
	if abs, err := filepath.Abs(pf); err == nil {
		file = abs
	}
	p, _, err := specdialect.ParseContent(d, file, raw)
	if err != nil {
		return verify.Plan{}, fmt.Errorf("plan %q (%s): %w", pf, d.ID(), err)
	}
	p.Service = service
	return p, nil
}

// planParserFor is parsePlanWith as the (service, bytes) parser the
// self-skip and --since comparisons use to read a prior version of the
// same file: the same dialect, never the native grammar over foreign
// bytes. A prior version that fails to parse reads as an empty plan.
func planParserFor(d specdialect.SpecDialect, pf string) func(string, []byte) verify.Plan {
	return func(service string, md []byte) verify.Plan {
		p, err := parsePlanWith(d, service, pf, md)
		if err != nil {
			return verify.Plan{Service: service}
		}
		return p
	}
}
