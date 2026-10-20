package main

// readouthook_daemonjoin.go — RO-30 (docs/plans/readouts.md phase 3; design
// docs/design/readouts.md section 5): the readout hook's daemon joins. A
// rich compile line (RO-24) places an identifier root only at a symbol the
// working tree changed; the joins here ask the index for the rest, the
// symbol at an identifier in a compile error, and name the tests that reach
// the changed symbols (the shared reach, reach.go). Each runs under its own
// deadline inside the hook's 1 s ceiling; a miss reads
// "index: not consulted (deadline)" in the line, never as nothing found.
// Rich mode only ([readouts] mode = rich, off by default).

import (
	"context"
	"errors"
	"path"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readout"
)

// daemonJoinCap is the daemon joins' own deadline. RO-M13 read the reach
// through a warm daemon at a p95 of 37 ms; a cold one (8 to 10 s building
// its call graph) is the named miss, never a wait.
var daemonJoinCap = 250 * time.Millisecond

// daemonJoinReserve is what the hook keeps under its ceiling after the
// joins: the line, the flush and the row.
const daemonJoinReserve = 100 * time.Millisecond

// daemonJoinMaxIdents bounds the identifiers one call asks about.
const daemonJoinMaxIdents = 3

// The miss reasons a daemon join names.
const (
	daemonJoinMissDeadline   = "deadline"
	daemonJoinMissDaemon     = "daemon"
	daemonJoinMissNotIndexed = "not indexed"
)

// daemonJoinMissLine is the receipt's words for a missed daemon join.
func daemonJoinMissLine(reason string) string {
	return "index: not consulted (" + reason + ")"
}

// daemonJoin runs join under the daemon joins' own deadline: daemonJoinCap,
// cut to what is left of the hook's ceiling after spent, less
// daemonJoinReserve. It returns "" when join answered, else the miss:
// daemonJoinMissDeadline when the deadline passed or no time was left to
// start, daemonJoinMissNotIndexed when the daemon indexes no service at the
// root, daemonJoinMissDaemon when the daemon failed. Synchronous: join's
// calls carry the context, so nothing outlives the call.
func daemonJoin(spent time.Duration, join func(ctx context.Context) error) string {
	d := daemonJoinCap
	if left := readoutHookCeiling - spent - daemonJoinReserve; left < d {
		d = left
	}
	if d <= 0 {
		return daemonJoinMissDeadline
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	err := join(ctx)
	switch {
	case err == nil:
		return ""
	case deadlinePassed(ctx) || errors.Is(err, context.DeadlineExceeded):
		return daemonJoinMissDeadline
	case errors.Is(err, errNoService):
		return daemonJoinMissNotIndexed
	}
	return daemonJoinMissDaemon
}

// readoutDaemonJoins asks the daemon, under one daemonJoin, for the
// definitions of idents (at most daemonJoinMaxIdents) and the tests that
// reach files, for the repository at root. On a miss it returns nothing
// from the daemon and the miss reason; a partial answer is never mixed in.
func readoutDaemonJoins(spent time.Duration, root string, idents, files []string) (defs []readout.CompileDef, tests []string, miss string) {
	if len(idents) == 0 && len(files) == 0 {
		return nil, nil, ""
	}
	if len(idents) > daemonJoinMaxIdents {
		idents = idents[:daemonJoinMaxIdents]
	}
	baseURL := defaultFathomURL()
	var gotDefs []readout.CompileDef
	var gotTests []string
	miss = daemonJoin(spent, func(ctx context.Context) error {
		svc, err := reachService(ctx, baseURL, root)
		if err != nil {
			return err
		}
		for _, id := range idents {
			d, ok, err := indexDefinition(ctx, baseURL, svc, id)
			if err != nil {
				return err
			}
			if ok {
				gotDefs = append(gotDefs, d)
			}
		}
		if len(files) > 0 {
			r, err := fetchBlastReach(ctx, baseURL, svc, files)
			if err != nil {
				return err
			}
			gotTests = r.Tests
		}
		return nil
	})
	if miss != "" {
		return nil, nil, miss
	}
	return gotDefs, gotTests, ""
}

// indexDefinition is the one definition the index holds for a compile
// error's identifier: find_symbol's exact answer when it is one symbol;
// for a qualified name ("money.Round") the one exact hit on its last part
// whose directory is the qualifier. Two candidates place nothing (never a
// guessed root).
func indexDefinition(ctx context.Context, baseURL, service, ident string) (readout.CompileDef, bool, error) {
	hits, err := findSymbolExact(ctx, baseURL, service, ident)
	if err != nil {
		return readout.CompileDef{}, false, err
	}
	if len(hits) == 1 {
		return readout.CompileDef{Name: ident, File: hits[0].FilePath, Line: hits[0].Line}, true, nil
	}
	qual, last, ok := cutLastDot(ident)
	if len(hits) > 0 || !ok {
		return readout.CompileDef{}, false, nil
	}
	hits, err = findSymbolExact(ctx, baseURL, service, last)
	if err != nil {
		return readout.CompileDef{}, false, err
	}
	var in []indexSymbol
	for _, h := range hits {
		if path.Base(path.Dir(h.FilePath)) == qual {
			in = append(in, h)
		}
	}
	if len(in) != 1 {
		return readout.CompileDef{}, false, nil
	}
	return readout.CompileDef{Name: ident, File: in[0].FilePath, Line: in[0].Line}, true, nil
}

// indexSymbol is one find_symbol hit, as the join reads it.
type indexSymbol struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
}

// findSymbolExact is find_symbol's exact hits for name; a case-insensitive
// or substring fallback is no definition.
func findSymbolExact(ctx context.Context, baseURL, service, name string) ([]indexSymbol, error) {
	var p struct {
		Symbols    []indexSymbol `json:"symbols"`
		MatchedVia string        `json:"matched_via"`
	}
	if err := callTestRunTool(ctx, baseURL, "find_symbol", map[string]any{"service": service, "name": name}, &p); err != nil {
		return nil, err
	}
	if p.MatchedVia != "exact" {
		return nil, nil
	}
	return p.Symbols, nil
}

// cutLastDot splits "money.Round" into "money" and "Round".
func cutLastDot(s string) (head, last string, ok bool) {
	for i := len(s) - 1; i > 0; i-- {
		if s[i] == '.' {
			return s[:i], s[i+1:], i < len(s)-1
		}
	}
	return "", "", false
}
