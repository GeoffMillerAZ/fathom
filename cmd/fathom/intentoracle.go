package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// intentoracle.go — the index half of EP-7, kept out of internal/intent so
// that package stays pure (no daemon, no store, testable with a fake).
//
// ONE round trip for the whole audit: the invariants name ~60 tests, and a
// find_symbol per name would add seconds to a doctor row budgeted at 500ms.
// A single query_symbols call over the service returns the symbol set, and
// prefix names ("TestQuery_StalenessReport_*") are answered from it.

type indexOracle struct {
	baseURL string
	service string
	timeout time.Duration
	// commit caches the answer per HEAD: the index does not change between
	// commits, so a second audit at the same commit costs nothing.
	commit string
	root   string
	// readOnly (`intent audit --read-only`, OC-38): the per-HEAD answer is
	// read from the ledger but never written to it.
	readOnly bool
	// hc, when set, carries the requests (the console passes its own
	// origin-stamped client, so a page's audit is never counted as usage).
	hc *http.Client
}

// httpClient is the oracle's client: the caller's, else a plain one.
func (o *indexOracle) httpClient() *http.Client {
	if o.hc != nil {
		return o.hc
	}
	return &http.Client{Timeout: o.timeout}
}

func newIndexOracle(baseURL, service, root, commit string, readOnly bool, hc *http.Client) intent.SymbolOracle {
	if service == "" {
		return nil
	}
	// 30s, not 5: one verify_plan over ~60 synthesized rows is a real
	// verify run (measured p50 2.8s, p95 6.7s), and the audit verb is
	// unbudgeted. The doctor row never pays this — it reads the cached
	// per-HEAD answer from the ledger.
	return &indexOracle{baseURL: baseURL, service: service, root: root, commit: commit, timeout: 30 * time.Second, readOnly: readOnly, hc: hc}
}

// Known answers every requested name in one round trip. A name ending in
// "*" names a test FAMILY ("TestCmdVerify_*" stands for ten tests); the
// star is trimmed and the family's stem is probed, because a family exists
// exactly when a member does.
func (o *indexOracle) Known(names []string) (map[string]bool, error) {
	out := make(map[string]bool, len(names))
	if len(names) == 0 {
		return out, nil
	}
	if cached, ok := o.cached(names); ok {
		return cached, nil
	}
	// A starred name is a FAMILY ("TestDeterminism_*" stands for four
	// tests). symbol_present cannot prefix-match, so families are answered
	// by a keyword query instead — there are only a handful, and treating a
	// family as absent because its stem is not itself a symbol would be a
	// false finding, the thing this audit exists to avoid.
	var exact, families []string
	for _, n := range names {
		if strings.HasSuffix(n, "*") {
			families = append(families, n)
			continue
		}
		exact = append(exact, n)
	}
	resolved, err := o.resolveNames(exact)
	if err != nil {
		return nil, err
	}
	for _, fam := range families {
		ok, err := o.familyExists(strings.TrimSuffix(fam, "*"))
		if err != nil {
			return nil, err
		}
		resolved[fam] = ok
	}
	o.store(resolved)
	for _, n := range names {
		out[n] = resolved[n]
	}
	return out, nil
}

// resolveNames answers every name in ONE verify_plan call: a synthesized
// in-memory plan with one row per named test (`skip_test: true`, so the
// question asked is exactly "does this symbol exist in the index?" and
// nothing more). Sixty find_symbol round trips would cost seconds; this
// costs one.
func (o *indexOracle) resolveNames(names []string) (map[string]bool, error) {
	var b strings.Builder
	b.WriteString("# synthesized by fathom intent audit\n\n")
	for i, n := range names {
		b.WriteString(fmt.Sprintf("- [ ] enforcement claim %d {id: EN-%d, symbol: %s, skip_test: true, skip: \"presence probe\"}\n",
			i+1, i+1, strings.TrimSuffix(n, "*")))
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service":       o.service,
				"plan_markdown": b.String(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := originHeader(o.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := o.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("daemon unreachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tasks []struct {
			ID       string `json:"id"`
			Criteria []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"criteria"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, err
	}
	if len(payload.Tasks) == 0 {
		return nil, fmt.Errorf("verify_plan returned no tasks for service %q — empty or stale index, not an answer", o.service)
	}
	out := make(map[string]bool, len(names))
	for i, n := range names {
		want := fmt.Sprintf("EN-%d", i+1)
		for _, task := range payload.Tasks {
			if task.ID != want {
				continue
			}
			for _, c := range task.Criteria {
				if c.Name == "symbol_present" {
					out[n] = strings.EqualFold(c.Status, "pass")
				}
			}
		}
	}
	return out, nil
}

// cached returns the per-HEAD answer a previous audit recorded, when one
// covers every requested name. The index cannot change without a commit, so
// this is exact rather than a heuristic — and it is what keeps the doctor
// row off a 6-second verify.
func (o *indexOracle) cached(names []string) (map[string]bool, bool) {
	if o.commit == "" {
		return nil, false
	}
	for _, row := range intent.ReadLedger() {
		if row.Kind != intent.KindOracle || row.ID != enforcementCacheID || row.Commit != o.commit || row.RepoRoot != o.root {
			continue
		}
		var got map[string]bool
		if err := json.Unmarshal([]byte(row.Evidence), &got); err != nil {
			continue
		}
		for _, n := range names {
			if _, ok := got[n]; !ok {
				return nil, false
			}
		}
		return got, true
	}
	return nil, false
}

// store records the answer for this commit — never on a read-only run.
func (o *indexOracle) store(resolved map[string]bool) {
	if o.commit == "" || o.readOnly {
		return
	}
	blob, err := json.Marshal(resolved)
	if err != nil {
		return
	}
	_ = intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindOracle, ID: enforcementCacheID, RepoRoot: o.root,
		Commit: o.commit, Verdict: "resolved", Evidence: string(blob),
	})
}

// enforcementCacheID is the reserved id for the enforcement-probe cache
// row. It is not a condition id — no condition can collide with it because
// every condition id contains a "#".
const enforcementCacheID = "enforcement-probe"

// familyExists reports whether any indexed symbol starts with prefix — how
// a starred test family is answered.
func (o *indexOracle) familyExists(prefix string) (bool, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name": "query_symbols",
			"arguments": map[string]any{
				"service":  o.service,
				"concern":  "tests",
				"keywords": []string{prefix},
				"top_n":    200,
			},
		},
	})
	if err != nil {
		return false, err
	}
	req, err := originHeader(o.baseURL, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	resp, err := o.httpClient().Do(req)
	if err != nil {
		return false, fmt.Errorf("daemon unreachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return false, err
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return false, err
	}
	var payload struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return false, err
	}
	for _, n := range payload.Nodes {
		if strings.HasPrefix(n.Name, prefix) {
			return true, nil
		}
	}
	return false, nil
}
