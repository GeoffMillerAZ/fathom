package semantic

// rulesscope.go — the scope-injection path GL-2..GL-6 share
// (docs/plans/governance-learning.md, design doc §3 "Rules readers"). A
// client's rules file often states its OWN scope — Cursor's `globs`,
// Copilot's `applyTo`, Kiro's `fileMatchPattern` — and that declared scope
// is the human's word on where the rule applies. The compiler must not let
// the model restate it: verifyDraft asks declaredDocScope for the document
// and, when the document declares globs, those globs REPLACE the model's
// proposed scope before the ordinary I27 checks (every glob matches an
// indexed file, none over-broad) run on them exactly as they do on any
// other claim. A declared audience (Kiro `inclusion: manual`, BUGBOT.md's
// review-only default) becomes a RelAudience link the serve path reads.
//
// The declaration comes from the RulesSource that owns the file (an
// optional ScopeDeclarer interface), not from a second table here — one
// reader, one file shape, one place its frontmatter is interpreted. The
// function is pure over (path, content): same bytes, same scope, on every
// machine and every recompile, and it never touches the filesystem.

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DocScope is a rules file's own declared scope and audience.
type DocScope struct {
	// Globs, when non-empty, replace the model's proposed scope for every
	// claim compiled from the document. Empty means "no declared globs":
	// the model's scope stands, verified as for any governing document
	// (Cursor's alwaysApply, Copilot's repo-wide instructions file, Kiro's
	// `inclusion: always` — repo scope, the same shape a root CLAUDE.md
	// has).
	Globs []string
	// Audience is "" (served everywhere), core.AudienceManual or
	// core.AudienceReview; non-empty writes a RelAudience link.
	Audience string
}

// ScopeDeclarer is the optional half of a RulesSource whose file shape can
// declare a scope. DeclaredScope reports ok=false for a path the source
// does not own, so declaredDocScope can ask every registered source.
type ScopeDeclarer interface {
	DeclaredScope(path string, content []byte) (DocScope, bool)
}

// declaredDocScope asks every registered source, in sorted-ID order, for
// the document's declared scope; the first owner wins. A document no
// source owns (CLAUDE.md, ADR.md, any plain governing doc) returns
// ok=false and compiles exactly as before this file existed.
func declaredDocScope(path string, content []byte) (DocScope, bool) {
	for _, id := range RegisteredRulesSources() {
		d, ok := rulesSourceRegistry[id].(ScopeDeclarer)
		if !ok {
			continue
		}
		if ds, owns := d.DeclaredScope(path, content); owns {
			return ds, true
		}
	}
	return DocScope{}, false
}

// applyDeclaredScope is verifyDraft's hook: the model's proposed scope, or
// the document's declared globs when it has any, plus the audience to
// record. Globs are trimmed of a leading "./" and a leading "/" (Copilot
// and Kiro both accept repo-root-relative "/src/**"), deduplicated and
// sorted.
func applyDeclaredScope(docPath string, content []byte, proposed []string) (scope []string, audience string) {
	ds, ok := declaredDocScope(docPath, content)
	if !ok {
		return proposed, ""
	}
	if len(ds.Globs) == 0 {
		return proposed, ds.Audience
	}
	return normalizeGlobs(ds.Globs), ds.Audience
}

func normalizeGlobs(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range in {
		g = strings.TrimSpace(g)
		g = strings.TrimPrefix(g, "./")
		g = strings.TrimPrefix(g, "/")
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// ─── frontmatter ────────────────────────────────────────────────────────────

// frontmatter is a rules file's leading `---` block as key → values. The
// three clients' frontmatter is YAML in name only — Cursor writes
// `globs: *.ts, src/**` unquoted (a YAML alias indicator), so a real YAML
// decoder rejects files the client itself accepts. This parser takes the
// shapes the clients document and nothing more: `key: value`, a quoted
// value, a comma-separated value, a flow list `[a, b]`, and a block list of
// `- item` lines under an empty `key:`. Keys are case-sensitive, as the
// clients read them.
type frontmatter map[string][]string

// parseFrontmatter returns the document's frontmatter, or ok=false when the
// document does not open with a `---` fence closed by another.
func parseFrontmatter(content []byte) (frontmatter, bool) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, false
	}
	lines := strings.Split(text, "\n")
	fm := frontmatter{}
	lastKey := ""
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			return fm, true
		}
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && lastKey != "" {
			fm[lastKey] = append(fm[lastKey], unquote(strings.TrimSpace(trimmed[2:])))
			continue
		}
		key, val, found := strings.Cut(l, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		lastKey = key
		fm[key] = splitFrontmatterValue(strings.TrimSpace(val))
	}
	return nil, false
}

// splitFrontmatterValue turns one value into its list of items: a flow list
// or a comma-separated scalar split on commas outside brace groups (a glob
// like "**/*.{ts,tsx}" is one item), each item unquoted.
func splitFrontmatterValue(v string) []string {
	if v == "" {
		return nil
	}
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		v = v[1 : len(v)-1]
	} else if uq := unquote(v); uq != v {
		v = uq
	}
	var out []string
	depth, start := 0, 0
	for i, r := range v {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if item := unquote(strings.TrimSpace(v[start:i])); item != "" {
					out = append(out, item)
				}
				start = i + 1
			}
		}
	}
	if item := unquote(strings.TrimSpace(v[start:])); item != "" {
		out = append(out, item)
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// first returns the key's first value, or "".
func (fm frontmatter) first(key string) string {
	if v := fm[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// isTrue reports whether the key's value is YAML true.
func (fm frontmatter) isTrue(key string) bool {
	return strings.EqualFold(fm.first(key), "true")
}

// audienceOf reads a claim's RelAudience from its links, "" when it has
// none — the serve path's filter input.
func audienceOf(links []core.ClaimLink) map[string]string {
	out := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelAudience {
			out[l.ClaimID] = l.TargetRef
		}
	}
	return out
}
