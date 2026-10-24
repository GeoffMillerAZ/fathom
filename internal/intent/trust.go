package intent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// trust.go — EP-37, the trust boundary.
//
// An annotation is REPOSITORY CONTENT: it arrives with a clone, a pull
// request, or a sibling committer. Treating it as a trusted instruction to
// this process would be the mistake — so every oracle argument is confined
// before anything reads it:
//
//   - `paths` may not leave the repository root. "../" and absolute paths
//     lint loud rather than reading a developer's home directory.
//   - `service` must be a service rooted in THIS repository. Cross-repo
//     querying would be federation (F18) through the back door, and
//     federation is refused.
//
// v1 oracles are read-only, so the blast radius of a hostile annotation is
// a count. The threat model itself is still an open owner question (Q5);
// this is the floor that holds regardless of its answer.

// ConfineError explains why an annotation argument was refused. The verdict
// it produces is `unevaluable` with this reason — never `fired`, and never
// a silent skip.
type ConfineError struct {
	Key    string
	Value  string
	Reason string
}

func (e *ConfineError) Error() string {
	return fmt.Sprintf("annotation %s=%q refused: %s", e.Key, e.Value, e.Reason)
}

// confineAnnotation validates one condition's oracle arguments against the
// repository they were read from. It returns nil when every argument is
// safe, and a *ConfineError naming the first refusal otherwise.
func confineAnnotation(root string, ann map[string]string, localServices map[string]bool) error {
	if len(ann) == 0 {
		return nil
	}
	for _, key := range []string{"paths", "path", "plan"} {
		raw, ok := ann[key]
		if !ok {
			continue
		}
		for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' }) {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if err := confinePath(root, key, p); err != nil {
				return err
			}
		}
	}
	if svc, ok := ann["service"]; ok {
		svc = strings.TrimSpace(svc)
		if svc == "" {
			return &ConfineError{Key: "service", Value: svc, Reason: "empty service"}
		}
		local := false
		for name, allowed := range localServices {
			if allowed && strings.EqualFold(name, svc) {
				local = true
				break
			}
		}
		if localServices != nil && !local {
			return &ConfineError{
				Key: "service", Value: svc,
				Reason: "cross-repo refused — a condition may only query a service rooted in this repository (F18 federation is not built)",
			}
		}
	}
	return nil
}

// confinePath refuses anything that leaves the repository.
func confinePath(root, key, p string) error {
	if filepath.IsAbs(p) || strings.HasPrefix(p, "~") {
		return &ConfineError{
			Key: key, Value: p,
			Reason: "absolute or home-relative path — an oracle reads only inside the repository",
		}
	}
	clean := filepath.Clean(filepath.Join(root, p))
	relPath, err := filepath.Rel(root, clean)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return &ConfineError{
			Key: key, Value: p,
			Reason: "escapes the repository root — an oracle reads only inside the repository",
		}
	}
	return nil
}

// ConfineAnnotation is confineAnnotation's exported face, so a caller can
// validate an annotation before an evaluator exists (the runtime is gated
// on K0's demand signal, but the boundary is not).
func ConfineAnnotation(root string, ann map[string]string, localServices map[string]bool) error {
	return confineAnnotation(root, ann, localServices)
}

// ResolveReferences is the exported entry point for U11 — the check that a
// decision or living doc does not cite code that moved. Exported because
// the audit is not the only sensible caller: a doc-lint, a pre-commit hook,
// or another repo's tooling can ask the same question.
func ResolveReferences(root string, files []string) []Claim {
	a := &Audit{}
	c := Corpus{Root: root}
	for _, f := range files {
		c.Conditions = append(c.Conditions, Condition{File: f})
	}
	a.auditReferences(AuditInput{Corpus: c})
	return a.Unproven
}
