package mcp

// localize_identity.go — TL-5 (docs/plans/telemetry-links.md, ADR-062,
// docs/design/telemetry-links.md section 4): a telemetry identity maps to
// a service. An alert, a span or an APM error names the service by its
// telemetry identity (an OpenTelemetry service.name, a New Relic app name,
// a Lambda function name, a Prometheus job), which is often not the
// repository's service id. The dialects record each identity a repository
// declares as a doc entity (source kind TelemetryIdentity, name the literal
// value, signature `<scheme>=<value>`); localize resolves through them,
// additively: no new tool, no schema or surface change.

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// telemetryIdentityKind is the TL-5 contract's source kind: items prom and
// mesh emit to the same name.
const telemetryIdentityKind = "TelemetryIdentity"

// telemetryIdentityMatch is one indexed service declaring an identity.
type telemetryIdentityMatch struct {
	Service string `json:"service"`
	Scheme  string `json:"scheme,omitempty"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// telemetryIdentityLiteral reports whether name is a literal identity: a
// value carrying interpolation (`${`, `{{`, `$(`) or a CloudFormation
// intrinsic (`!Ref`, `!Sub`) is never matched, because what it resolves
// to is decided at deploy time and fathom never guesses it.
func telemetryIdentityLiteral(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, m := range []string{"${", "{{", "$("} {
		if strings.Contains(name, m) {
			return false
		}
	}
	for _, p := range []string{"!Ref", "!Sub"} {
		if strings.HasPrefix(strings.TrimSpace(name), p) {
			return false
		}
	}
	return true
}

// telemetryIdentityScheme is the scheme of an identity's signature
// (`service.name=checkout` is service.name).
func telemetryIdentityScheme(sig string) string {
	if i := strings.IndexByte(sig, '='); i > 0 {
		return sig[:i]
	}
	return ""
}

// resolveTelemetryIdentity is TL-5's additive resolution. When service
// names an indexed service it is returned unchanged. Otherwise every
// indexed service is asked for a TelemetryIdentity symbol of that exact
// literal name: exactly one service answers, and that service is
// returned with the matches that named it; two or more, and the caller
// gets them all back to choose from (never a guess); none, and service is
// returned unchanged so localize's own not-indexed note answers.
func resolveTelemetryIdentity(ctx context.Context, backend QueryInterface, service string) (resolved string, matches []telemetryIdentityMatch, ambiguous bool) {
	if _, _, found, _ := backend.ServiceMeta(ctx, service); found {
		return service, nil, false
	}
	if !telemetryIdentityLiteral(service) {
		return service, nil, false
	}
	ids, err := backend.Services(ctx)
	if err != nil {
		return service, nil, false
	}
	sort.Strings(ids)
	byService := map[string][]telemetryIdentityMatch{}
	for _, id := range ids {
		syms, err := backend.FindSymbol(ctx, id, service)
		if err != nil {
			continue
		}
		for _, n := range syms {
			if string(n.Kind) != telemetryIdentityKind || n.Name != service {
				continue
			}
			byService[id] = append(byService[id], telemetryIdentityMatch{Service: id, Scheme: telemetryIdentityScheme(n.Signature), File: n.FilePath, Line: n.Line})
		}
	}
	var all []telemetryIdentityMatch
	for _, id := range ids {
		ms := byService[id]
		sort.Slice(ms, func(i, j int) bool {
			if ms[i].File != ms[j].File {
				return ms[i].File < ms[j].File
			}
			return ms[i].Line < ms[j].Line
		})
		all = append(all, ms...)
	}
	switch len(byService) {
	case 0:
		return service, nil, false
	case 1:
		return all[0].Service, all, false
	default:
		return service, all, true
	}
}

// telemetryIdentityRefusal is the message for an identity two or more
// indexed services declare.
func telemetryIdentityRefusal(identity string, matches []telemetryIdentityMatch) string {
	var names []string
	seen := map[string]bool{}
	for _, m := range matches {
		if !seen[m.Service] {
			seen[m.Service] = true
			names = append(names, m.Service)
		}
	}
	return fmt.Sprintf("localize: %q is no indexed service, and %d indexed services declare it as a telemetry identity (%s); pass one of them as service", identity, len(names), strings.Join(names, ", "))
}
