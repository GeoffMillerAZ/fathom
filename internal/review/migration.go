package review

// migration.go — CX-9 (design §4, U9 the migration tracker): a migration
// declaration names FROM and TO for one of five kinds — a symbol, an
// import path, a route, a config key, or a `pkg:` version — the shape
// design §4 says can be "declared in a plan (the refactor-conservation
// grammar already carries gone:, retires: and surface_change:), or
// written by `fathom refactor plan`". Both sources hand ParseMigration the
// SAME one-line text (a plan row's own annotation value, or a bare line a
// generator writes) — this file parses that text and nothing about where
// it came from:
//
//	symbol OldName -> NewName
//	import "internal/old/pkg" -> "internal/new/pkg"     (quotes optional)
//	route GET /v1/things -> GET /v2/things
//	config old.setting.key -> new.setting.key
//	pkg example.com/mod@v1.2.0 -> example.com/mod@v2.0.0
//
// Pure and deterministic like the rest of this package (receipt.go's own
// doc comment): no I/O, no clock, identical input always parses to an
// identical Migration.
import (
	"fmt"
	"strconv"
	"strings"
)

// MigrationKind is the fixed vocabulary CX-9 supports — never open-ended,
// so an unrecognized word is a named parse error rather than a silently
// accepted new kind (design §4 names exactly these five).
type MigrationKind string

const (
	MigrationSymbol MigrationKind = "symbol"
	MigrationImport MigrationKind = "import"
	MigrationRoute  MigrationKind = "route"
	MigrationConfig MigrationKind = "config"
	MigrationPkg    MigrationKind = "pkg"
)

// Migration is one declared from/to pair — CX-10's MigrationSites and
// CX-12's MigrationBurnDown both key off Kind+From+To, so this type is
// intentionally the smallest fact both later rows need: the declaration
// itself carries no site count, no repository and no proof — those are
// computed, not declared (design §4: "Fathom computes... per repository").
type Migration struct {
	Kind MigrationKind
	From string
	To   string
}

// ParseMigration parses one migration declaration's text — the value that
// follows a plan row's "migration:" annotation key, or a bare generated
// line — in the fixed "<kind> <from> -> <to>" form. from/to may each be a
// double-quoted Go string literal (matching the quote-aware plan grammar
// internal/verify/markdown.go already uses for other annotation values) or
// bare text; either way surrounding whitespace is trimmed. Every rejection
// names what was wrong — never a zero-value Migration returned silently.
func ParseMigration(raw string) (Migration, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Migration{}, fmt.Errorf("empty migration declaration")
	}

	kindWord, rest, ok := strings.Cut(s, " ")
	if !ok {
		return Migration{}, fmt.Errorf("migration declaration %q: missing from/to (want \"<kind> <from> -> <to>\")", raw)
	}
	kind, kerr := parseMigrationKind(kindWord)
	if kerr != nil {
		return Migration{}, fmt.Errorf("migration declaration %q: %w", raw, kerr)
	}

	from, to, ok := strings.Cut(rest, "->")
	if !ok {
		return Migration{}, fmt.Errorf("migration declaration %q: missing \"->\" between from and to", raw)
	}
	from = unquoteMigrationValue(strings.TrimSpace(from))
	to = unquoteMigrationValue(strings.TrimSpace(to))
	if from == "" || to == "" {
		return Migration{}, fmt.Errorf("migration declaration %q: from and to must both be non-empty", raw)
	}
	if from == to {
		return Migration{}, fmt.Errorf("migration declaration %q: from and to are identical", raw)
	}

	return Migration{Kind: kind, From: from, To: to}, nil
}

// parseMigrationKind validates word against the fixed MigrationKind
// vocabulary.
func parseMigrationKind(word string) (MigrationKind, error) {
	switch MigrationKind(word) {
	case MigrationSymbol, MigrationImport, MigrationRoute, MigrationConfig, MigrationPkg:
		return MigrationKind(word), nil
	default:
		return "", fmt.Errorf("unknown migration kind %q (want symbol|import|route|config|pkg)", word)
	}
}

// unquoteMigrationValue strips a wrapping double-quoted Go string literal,
// when s is one; anything else (a bare word, an import path with no
// quotes, a route's "METHOD /path") passes through unchanged.
func unquoteMigrationValue(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unq, err := strconv.Unquote(s); err == nil {
			return unq
		}
	}
	return s
}
