package custodian

// stanza.go — CC-14 (docs/plans/custodian.md): the stanza-current
// concern. The fathom stanza in CLAUDE.md / AGENTS.md is fathom's OWN
// instruction to agents — "resolve via `find_symbol`, `references`, …" —
// so "instructed means true" binds it first: every tool it names must be
// one the running daemon serves. A stanza that names a pruned or
// undeployed tool sends every session at a call that fails.
//
// This is the one check kind that leaves the tree: it asks the daemon
// for its tools/list. Unreachable is unable-to-check naming the URL —
// never a violation, because a daemon that is down says nothing about
// whether the stanza is true (law 3). The same law covers an EMPTY or
// malformed registry: a 200 whose body carries no tools (result:null, a
// `{}` from some other service on the port, a daemon mid-start) is not a
// registry the stanza can be held to — reporting every name as violated
// would be eight false findings, and a false violation teaches people to
// ignore the custodian.
//
// What counts as a tool name in the stanza:
//
//   - any backticked snake_case identifier (`find_symbol`, `ingest`,
//     `references`) — the stanza's canonical citation form;
//   - any BARE token that contains an underscore (find_callers,
//     pack_context, verify_plan) — the v3 template's "Opening moves"
//     lines name tools this way, and an underscore-joined lowercase word
//     is a tool name and nothing else in this prose.
//
// Single-word tool names (obligations, references, outline, ingest,
// capabilities) are policed ONLY when backticked. Bare, they are ordinary
// English: "the obligations hook", "references on changed symbols",
// "outline the change" — policing them would flag prose as a pruned tool
// on every audit, and the false positives would drown the one real
// finding this concern exists to raise.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Stanza markers, as `fathom init` writes them (cmd/fathom/init.go) and
// the wiring engine's HTMLMarkers read them: the begin line may carry
// v3 attributes (version, body digest) after the prefix.
const (
	stanzaBeginPrefix = "<!-- fathom:begin"
	stanzaEnd         = "<!-- fathom:end -->"
)

// toolNameRe is what a backticked token must look like to be read as a
// tool name: a snake_case identifier. Multi-word commands (`fathom thread
// note`), paths, JSON fragments, and globs all fail it, so the stanza's
// prose cites cannot masquerade as tools.
var toolNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// bareToolNameRe finds tool names cited WITHOUT backticks: a lowercase
// identifier with at least one underscore (find_callers, pack_context).
// The underscore is the whole discriminator — see the file comment for
// why single words stay unpoliced when bare. Backticked spans are blanked
// before this runs, so a name cited both ways on one line is one ref.
var bareToolNameRe = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)

// stanzaToolRef is one tool name at a place in a file.
type stanzaToolRef struct {
	File string
	Line int // 1-based
	Name string
}

// stanzaLineToolNames returns the tool names one stanza line cites —
// backticked snake_case identifiers plus bare underscore-bearing tokens —
// de-duplicated, in first-seen order.
func stanzaLineToolNames(ln string) []string {
	seen := map[string]struct{}{}
	var names []string
	add := func(name string) {
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for _, m := range backtickTokenRe.FindAllStringSubmatch(ln, -1) {
		if toolNameRe.MatchString(m[1]) {
			add(m[1])
		}
	}
	bare := backtickTokenRe.ReplaceAllString(ln, " ")
	for _, m := range bareToolNameRe.FindAllString(bare, -1) {
		add(m)
	}
	return names
}

// checkStanzaToolsServed collects the tool names every agent-config
// file's stanza cites, then compares them against the daemon's served
// registry — fetched once, and only when there is something to compare.
func checkStanzaToolsServed(root string, files []string, c Concern, env auditEnv) Finding {
	f := Finding{Concern: c}

	var refs []stanzaToolRef
	stanzas := 0
	for _, rel := range files {
		lines, err := readFileLines(root, rel)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s unreadable: %v", rel, err)
			return f
		}
		body, present, err := stanzaRegion(lines)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s: %v — fix the markers by hand (fathom reconcile reports the same)", rel, err)
			return f
		}
		if !present {
			continue
		}
		stanzas++
		for i, ln := range body.lines {
			for _, name := range stanzaLineToolNames(ln) {
				refs = append(refs, stanzaToolRef{File: rel, Line: body.first + i, Name: name})
			}
		}
	}
	if stanzas == 0 {
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("no fathom stanza across %s — nothing names a tool", countNoun(len(files), "agent-config file"))
		return f
	}
	if len(refs) == 0 {
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("%d stanza(s) name no tools — nothing to hold to the registry", stanzas)
		return f
	}
	names := uniqueNames(refs)

	served, err := env.toolsFetch(env.daemonURL)
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("daemon at %s gave no usable registry (%v) — %d tool name(s) across %d stanza(s) could not be compared with the served registry", env.daemonURL, err, len(names), stanzas)
		return f
	}
	if len(served) == 0 {
		// A fetcher that hands back nothing (an injected seam, or a
		// registry whose every entry was nameless) is the same absence of
		// evidence as a daemon that is down — never a violation.
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("daemon at %s served an empty registry — %d tool name(s) across %d stanza(s) could not be compared (no tools at all is a wrong surface or a daemon mid-start, not proof the stanza is false)", env.daemonURL, len(names), stanzas)
		return f
	}
	have := make(map[string]struct{}, len(served))
	for _, name := range served {
		have[name] = struct{}{}
	}

	var missing []stanzaToolRef
	for _, r := range refs {
		if _, ok := have[r.Name]; !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		// refs are already in (sorted file, ascending line) order; make the
		// contract explicit rather than incidental.
		sort.SliceStable(missing, func(i, j int) bool {
			if missing[i].File != missing[j].File {
				return missing[i].File < missing[j].File
			}
			if missing[i].Line != missing[j].Line {
				return missing[i].Line < missing[j].Line
			}
			return missing[i].Name < missing[j].Name
		})
		for _, r := range missing {
			f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d → %s", r.File, r.Line, r.Name))
		}
		gone := uniqueNames(missing)
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d of %d stanza tool name(s) not served by the daemon at %s: %s (registry serves %d tool(s)) — the instruction outlived the tool",
			len(gone), len(names), env.daemonURL, strings.Join(gone, ", "), len(have))
		return f
	}
	f.Status = StatusHolds
	f.Detail = fmt.Sprintf("%d tool name(s) across %d stanza(s) all served by the daemon at %s (registry serves %d tool(s))",
		len(names), stanzas, env.daemonURL, len(have))
	return f
}

func uniqueNames(refs []stanzaToolRef) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range refs {
		if _, dup := seen[r.Name]; dup {
			continue
		}
		seen[r.Name] = struct{}{}
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

// stanzaBody is the lines between the markers and the 1-based line
// number of the first of them.
type stanzaBody struct {
	lines []string
	first int
}

// stanzaRegion finds the marker-owned region in a file's lines. A file
// with neither marker is simply not wired (present=false); one marker of
// the pair, a duplicate, or an END before the BEGIN is corruption the
// audit refuses to guess at — the wiring engine's law-15 stance.
func stanzaRegion(lines []string) (stanzaBody, bool, error) {
	begin, end := -1, -1
	begins, ends := 0, 0
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(t, stanzaBeginPrefix):
			begins++
			if begin < 0 {
				begin = i
			}
		case t == stanzaEnd:
			ends++
			if end < 0 {
				end = i
			}
		}
	}
	switch {
	case begins == 0 && ends == 0:
		return stanzaBody{}, false, nil
	case begins > 1 || ends > 1:
		return stanzaBody{}, false, fmt.Errorf("found %d begin / %d end markers — fathom owns at most one region per file", begins, ends)
	case begins == 0:
		return stanzaBody{}, false, fmt.Errorf("found %q with no matching %q", stanzaEnd, stanzaBeginPrefix)
	case ends == 0:
		return stanzaBody{}, false, fmt.Errorf("found %q with no matching %q", stanzaBeginPrefix, stanzaEnd)
	case end < begin:
		return stanzaBody{}, false, fmt.Errorf("%q appears before %q", stanzaEnd, stanzaBeginPrefix)
	}
	return stanzaBody{lines: lines[begin+1 : end], first: begin + 2}, true, nil
}

// toolsListEnvelope is the slice of a tools/list response this check
// reads. Pointers, not values, so an absent `result` or `result.tools`
// is told apart from an empty one — the shape is part of the error.
type toolsListEnvelope struct {
	Result *struct {
		Tools *[]struct {
			Name string `json:"name"`
		} `json:"tools"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// FetchServedTools is the default registry lookup: POST tools/list to the
// daemon's agent surface (/rpc/agent — the surface the stanza's readers
// are wired to), falling back to the bare /rpc for a daemon that
// predates scoped surfaces. Two-second timeout; sorted, de-duplicated
// names. Any failure is an error for the caller to report as
// unable-to-check — this function never invents an empty registry, and
// never RETURNS one: a 200 whose body has no result, no tools array, an
// empty one, or only nameless entries is an error naming the URL and the
// shape seen, because such a body is some other service (or a daemon
// mid-start), not a registry the stanza can be held to.
func FetchServedTools(baseURL string) ([]string, error) {
	hc := &http.Client{Timeout: 2 * time.Second}
	base := strings.TrimRight(baseURL, "/")
	const body = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

	var lastErr error
	for _, path := range []string{"/rpc/agent", "/rpc"} {
		url := base + path
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err // unreachable: the second path would fail the same way
		}
		raw, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
		resp.Body.Close()
		if rerr != nil {
			return nil, fmt.Errorf("%s: %w", url, rerr)
		}
		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("%s: HTTP 404", url)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
		}
		var env toolsListEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("%s: tools/list body is not JSON-RPC (%s): %w", url, bodyShape(raw), err)
		}
		if env.Error != nil {
			return nil, fmt.Errorf("%s: tools/list: %s", url, env.Error.Message)
		}
		switch {
		case env.Result == nil:
			return nil, fmt.Errorf("%s: tools/list returned no result (%s) — not a fathom agent surface?", url, bodyShape(raw))
		case env.Result.Tools == nil:
			return nil, fmt.Errorf("%s: tools/list result has no tools array (%s) — not a fathom agent surface?", url, bodyShape(raw))
		case len(*env.Result.Tools) == 0:
			return nil, fmt.Errorf("%s: tools/list returned an empty tools array — a registry with no tools cannot be compared against", url)
		}
		seen := map[string]struct{}{}
		var out []string
		for _, t := range *env.Result.Tools {
			if t.Name == "" {
				continue
			}
			if _, dup := seen[t.Name]; dup {
				continue
			}
			seen[t.Name] = struct{}{}
			out = append(out, t.Name)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%s: tools/list returned %d tool entr(y|ies) with no name — malformed registry", url, len(*env.Result.Tools))
		}
		sort.Strings(out)
		return out, nil
	}
	return nil, lastErr
}

// bodyShape renders the head of a response body for an error message:
// whitespace-collapsed, capped, so the operator sees WHAT answered on the
// port (an HTML login page, a `{}`, a null result) without a dump.
func bodyShape(raw []byte) string {
	s := strings.Join(strings.Fields(string(raw)), " ")
	if s == "" {
		return "empty body"
	}
	const cap = 96
	r := []rune(s)
	if len(r) > cap {
		s = string(r[:cap-1]) + "…"
	}
	return fmt.Sprintf("body %q", s)
}
