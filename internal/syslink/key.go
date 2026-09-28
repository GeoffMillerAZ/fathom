// Package syslink is the system graph's link model (ADR-050,
// docs/design/system-graph.md): nodes provide or consume typed contract
// keys, and a link is a key match graded by one of four evidence tiers.
//
// This package holds the pure halves of the model: the key grammar (§3),
// the tiers and key rows (§4), the ContractSource port and its registry by
// dialect id (§5), the wave-2 sources (routes, HTTP clients, OpenAPI,
// proto/gRPC) and the in-memory link join. Everything here is pure and
// deterministic (internal/CLAUDE.md): no I/O, no clock, sorted outputs.
// Persistence (§6's contract_keys and system_links tables) is a separate
// slice gated by ADR-049 decision 2's tenancy lane.
package syslink

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
)

// Kind is a contract key's kind: the part before the first ':'.
type Kind string

// The eleven kinds of §3. ADR-044's four inventory kinds (route, rpc, env,
// cli) keep their exact spelling, so conservation reads the same keys.
const (
	KindRoute  Kind = "route"
	KindRPC    Kind = "rpc"
	KindGQL    Kind = "gql"
	KindTopic  Kind = "topic"
	KindTable  Kind = "table"
	KindEnv    Kind = "env"
	KindConfig Kind = "config"
	KindPkg    Kind = "pkg"
	KindImage  Kind = "image"
	KindDeploy Kind = "deploy"
	KindCLI    Kind = "cli"
)

// Kinds returns the eleven kinds, sorted.
func Kinds() []Kind {
	return []Kind{KindCLI, KindConfig, KindDeploy, KindEnv, KindGQL, KindImage, KindPkg, KindRPC, KindRoute, KindTable, KindTopic}
}

// ContractKey is one parsed, normalized key. Two keys naming the same
// contract compare equal with ==.
type ContractKey struct {
	Kind Kind
	Body string
}

// String renders the key in its stored form, "<kind>:<body>".
func (k ContractKey) String() string { return string(k.Kind) + ":" + k.Body }

var normalizers = map[Kind]func(string) (string, error){
	KindRoute:  normRoute,
	KindRPC:    normRPC,
	KindGQL:    normGQL,
	KindTopic:  normTopic,
	KindTable:  normTable,
	KindEnv:    normEnv,
	KindConfig: normConfig,
	KindPkg:    normPkg,
	KindImage:  normImage,
	KindDeploy: normDeploy,
	KindCLI:    normCLI,
}

// ParseContractKey parses and normalizes s ("<kind>:<body>") by its kind's
// normalizer (§3). It refuses, with an error naming the reason, a key that
// is not ASCII, has no kind separator, names a kind outside the eleven, or
// whose body the kind's grammar rejects. Normalization is idempotent: the
// String of a parsed key parses to the same key.
func ParseContractKey(s string) (ContractKey, error) {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return ContractKey{}, fmt.Errorf("contract key %q: not ASCII", s)
		}
	}
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return ContractKey{}, fmt.Errorf("contract key %q: no kind separator ':'", s)
	}
	kind := Kind(s[:i])
	norm, ok := normalizers[kind]
	if !ok {
		return ContractKey{}, fmt.Errorf("contract key %q: unknown kind %q (want one of %v)", s, string(kind), Kinds())
	}
	body, err := norm(s[i+1:])
	if err != nil {
		return ContractKey{}, fmt.Errorf("contract key %q: %s: %w", s, kind, err)
	}
	return ContractKey{Kind: kind, Body: body}, nil
}

// mustKey is ParseContractKey for keys built from already-validated parts
// inside this package's sources; it panics on a grammar error, which is a
// programming error there, never input-dependent.
func mustKey(s string) ContractKey {
	k, err := ParseContractKey(s)
	if err != nil {
		panic(err)
	}
	return k
}

func hasSpace(s string) bool { return strings.ContainsAny(s, " \t\r\n") }

var (
	methodRe = regexp.MustCompile(`^[A-Za-z]+$`)
	identRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	sqlSegRe = regexp.MustCompile(`^("[^"]+"|[A-Za-z_][A-Za-z0-9_$]*)$`)
)

// errBody is the refusal every normalizer returns for a body its grammar
// does not accept; the kind is added by ParseContractKey.
func errBody(body, want string) error { return fmt.Errorf("body %q is not %s", body, want) }

// route: "METHOD /path/template" — method upper-cased; the path through
// serviceref.NormalizeTemplate (params to {}, trailing slash and query
// dropped). Exactly one space separates the two.
func normRoute(body string) (string, error) {
	sp := strings.IndexByte(body, ' ')
	if sp <= 0 {
		return "", errBody(body, "METHOD /path")
	}
	method, path := body[:sp], body[sp+1:]
	if !methodRe.MatchString(method) || path == "" || hasSpace(path) {
		return "", errBody(body, "METHOD /path")
	}
	return strings.ToUpper(method) + " " + serviceref.NormalizeTemplate(path), nil
}

// rpc: "package.Service/Method" as ADR-044's inventory spells it (a file
// with no proto package yields a bare "Service/Method").
func normRPC(body string) (string, error) {
	svc, method, ok := strings.Cut(body, "/")
	if !ok || svc == "" || !identRe.MatchString(method) || hasSpace(svc) || strings.Contains(svc, "/") {
		return "", errBody(body, "package.Service/Method")
	}
	return body, nil
}

// gql: "Type.field", type names as declared.
func normGQL(body string) (string, error) {
	typ, field, ok := strings.Cut(body, ".")
	if !ok || !identRe.MatchString(typ) || !identRe.MatchString(field) {
		return "", errBody(body, "Type.field")
	}
	return body, nil
}

// topic: the name, exact.
func normTopic(body string) (string, error) {
	if body == "" || hasSpace(body) {
		return "", errBody(body, "a whitespace-free topic name")
	}
	return body, nil
}

// table: "[schema.]table[.column]", lower-cased unless quoted in DDL (a
// quoted segment keeps its quotes and its case, so normalization stays
// idempotent and `"Orders"` never collides with `orders`).
func normTable(body string) (string, error) {
	segs := strings.Split(body, ".")
	if len(segs) > 3 || body == "" {
		return "", errBody(body, "[schema.]table[.column]")
	}
	for i, s := range segs {
		if !sqlSegRe.MatchString(s) {
			return "", errBody(body, "[schema.]table[.column]")
		}
		if !strings.HasPrefix(s, `"`) {
			segs[i] = strings.ToLower(s)
		}
	}
	return strings.Join(segs, "."), nil
}

// env: "NAME", as ADR-044.
func normEnv(body string) (string, error) {
	if body == "" || hasSpace(body) || strings.ContainsAny(body, "=:") {
		return "", errBody(body, "an environment variable name")
	}
	return body, nil
}

// config: "namespace/key", dots for nesting.
func normConfig(body string) (string, error) {
	ns, key, ok := strings.Cut(body, "/")
	if !ok || ns == "" || key == "" || hasSpace(body) {
		return "", errBody(body, "namespace/key")
	}
	return body, nil
}

var pkgEcosystems = map[string]bool{"golang": true, "npm": true, "pypi": true, "maven": true, "cargo": true, "nuget": true}

// pkg: "ecosystem/name[@version][#symbol]", ecosystem lower-cased and one
// of golang, npm, pypi, maven, cargo, nuget; version as the lockfile pins it.
func normPkg(body string) (string, error) {
	eco, rest, ok := strings.Cut(body, "/")
	eco = strings.ToLower(eco)
	if !ok || !pkgEcosystems[eco] || rest == "" || hasSpace(rest) {
		return "", errBody(body, "ecosystem/name@version#symbol with a known ecosystem")
	}
	name := rest
	if h := strings.IndexByte(name, '#'); h >= 0 {
		if h == len(name)-1 {
			return "", errBody(body, "ecosystem/name@version#symbol (empty symbol)")
		}
		name = name[:h]
	}
	if at := strings.LastIndexByte(name, '@'); at == 0 || at == len(name)-1 {
		return "", errBody(body, "ecosystem/name@version#symbol")
	}
	return eco + "/" + rest, nil
}

// image: "registry/repo" with the registry host lower-cased; a tag or
// digest is an attribute, not part of the key, so it is dropped.
func normImage(body string) (string, error) {
	if body == "" || hasSpace(body) {
		return "", errBody(body, "registry/repo")
	}
	if at := strings.IndexByte(body, '@'); at >= 0 {
		body = body[:at]
	}
	segs := strings.Split(body, "/")
	last := segs[len(segs)-1]
	if c := strings.IndexByte(last, ':'); c >= 0 {
		segs[len(segs)-1] = last[:c]
	}
	if len(segs) < 2 {
		return "", errBody(body, "registry/repo")
	}
	for _, s := range segs {
		if s == "" {
			return "", errBody(body, "registry/repo")
		}
	}
	segs[0] = strings.ToLower(segs[0])
	return strings.Join(segs, "/"), nil
}

// deploy: "k8s/<namespace>/<service>" or "host/<hostname>", lower-cased.
func normDeploy(body string) (string, error) {
	b := strings.ToLower(body)
	segs := strings.Split(b, "/")
	switch {
	case len(segs) == 3 && segs[0] == "k8s" && segs[1] != "" && segs[2] != "" && !hasSpace(b):
		return b, nil
	case len(segs) == 2 && segs[0] == "host" && segs[1] != "" && !hasSpace(b):
		return b, nil
	}
	return "", errBody(body, "k8s/<namespace>/<service> or host/<hostname>")
}

// cli: "command path", words separated by single spaces (as ADR-044).
func normCLI(body string) (string, error) {
	f := strings.Fields(body)
	if len(f) == 0 {
		return "", errBody(body, "a command path")
	}
	return strings.Join(f, " "), nil
}
