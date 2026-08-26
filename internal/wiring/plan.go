package wiring

// plan.go — the typed diff (design §4).
//
// `plan` is ALWAYS computed before any mutation, even when the caller asked
// to apply. There is no code path in this package that writes a byte
// without first producing the diff that says what it would write. That is
// what makes the UI's "show me first" and the CLI's --apply the same
// operation with a different number of steps, and it is what makes BLOCKED
// a verdict rather than an exception.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Verdict is one item's outcome. Three values, because "nothing to do" and
// "fathom refuses to do it" demand opposite responses from a human and
// collapsing them into a boolean is how a safety report gets ignored.
type Verdict string

const (
	VerdictApply   Verdict = "apply"   // fathom will write this
	VerdictSkip    Verdict = "skip"    // already current, or not fathom's to touch
	VerdictBlocked Verdict = "blocked" // drift: law 15 stops here
)

// Item kinds.
const (
	KindRegion = "region"
	KindKey    = "key"
)

// Causes of a BLOCKED verdict. Law 15 stops for more than one reason, and
// the reasons demand different responses from a human — a hand-edited
// region is a choice to make (keep yours or take fathom's), an unparsable
// file is a repair to do — so the cause is typed here rather than left for
// a caller to recover by parsing Reason.
const (
	// CauseDrift: a region or entry fathom is TRACKING no longer matches
	// what fathom recorded — a human has been inside fathom's region.
	CauseDrift = "drift"
	// CauseUnreadable: the file could not be read, or could not be parsed
	// as the structure fathom expects (a corrupted marker pair, invalid
	// JSON) — fathom will not guess at a file it cannot read.
	CauseUnreadable = "unreadable"
	// CauseUnproven: the region carries no digest and matches nothing
	// fathom ever shipped — fathom cannot prove it wrote it and, without
	// Region.AdoptUntracked, will not claim it.
	CauseUnproven = "unproven"
)

// Item is one region or one key, with the verdict and — always — the reason
// behind it. An item with an empty Reason is a defect: a verdict a human
// cannot interpret is a verdict they will learn to ignore (law 3).
type Item struct {
	File    string  `json:"file"`
	Target  string  `json:"target"`
	Kind    string  `json:"kind"`
	Verdict Verdict `json:"verdict"`
	Reason  string  `json:"reason"`
	// Cause classifies a BLOCKED verdict (CauseDrift, CauseUnreadable,
	// CauseUnproven); empty for apply and skip.
	Cause string `json:"cause,omitempty"`
	// Present distinguishes "there is nothing there" from "there is an empty
	// value there". Before is meaningless unless Present.
	Present bool   `json:"present"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after"`
}

// Plan is the whole typed diff for one repository.
type Plan struct {
	Root string `json:"root"`
	// Items is sorted by file, then kind, then target — determinism is a
	// contract, and a diff that reorders itself between runs cannot be
	// compared by digest.
	Items []Item `json:"items"`
	// Digest covers the items only (not Root), so the same repository state
	// produces the same digest on any machine. It is the preview→apply
	// freshness token: if the repository changed since the preview, the
	// digest changes and the apply refuses.
	Digest string `json:"digest"`
	// LedgerNote names why fathom is treating itself as the owner of nothing
	// in this repository, when that is what happened. Empty when the ledger
	// read normally.
	LedgerNote string `json:"ledger_note,omitempty"`
}

// Blocked returns the items law 15 stopped on.
func (p Plan) Blocked() []Item { return p.filter(VerdictBlocked) }

// Applicable returns the items apply would write.
func (p Plan) Applicable() []Item { return p.filter(VerdictApply) }

// Skipped returns the items that need nothing, with the reason each.
func (p Plan) Skipped() []Item { return p.filter(VerdictSkip) }

func (p Plan) filter(v Verdict) []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Verdict == v {
			out = append(out, it)
		}
	}
	return out
}

// Region is one marked region fathom owns in a shared text file.
type Region struct {
	File    string       // repo-relative, slash-separated
	Syntax  MarkerSyntax //
	Version string       // the template version that produces Body ("v3")
	Body    string       // what fathom wants between the markers
	// Known are bodies EARLIER fathom versions wrote. A pre-v3 region
	// records no digest, so its provenance is unknown; matching a body
	// fathom is known to have shipped is the only evidence available that
	// fathom, and not a human, wrote it. Anything else stays BLOCKED —
	// unless AdoptUntracked is set.
	Known []string
	// AdoptUntracked is the v1→v3 migration switch (INC-12). A region whose
	// marker carries no digest cannot prove its provenance either way; the
	// caller that sets this asserts that the MARKER PAIR is the ownership
	// claim (that is what `fathom init` wrote for a year, and what every
	// repository in the fleet carries), so the region is fathom's, is
	// upgraded to a digest-bearing marker on the first write, and every
	// edit inside it after that is detectable. Only a TRACKED region that
	// differs from its recorded digest is drift; an untracked one is never
	// reported as drift. The engine default (false) stays strict.
	AdoptUntracked bool
}

// Spec is what fathom wants a repository to carry.
type Spec struct {
	Regions []Region
	Keys    []Key
}

func (s Spec) validate() error {
	seenRegion := map[string]bool{}
	for _, r := range s.Regions {
		if r.File == "" {
			return fmt.Errorf("region with no file")
		}
		if seenRegion[r.File] {
			return fmt.Errorf("two regions target %s — fathom owns at most one region per file", r.File)
		}
		seenRegion[r.File] = true
	}
	seenKey := map[string]bool{}
	for _, k := range s.Keys {
		if k.File == "" {
			return fmt.Errorf("key with no file")
		}
		if _, err := k.desired(); err != nil {
			return fmt.Errorf("key %s in %s: %w", k.ID(), k.File, err)
		}
		id := k.File + "\x00" + k.ID()
		if seenKey[id] {
			return fmt.Errorf("two keys target %s in %s", k.ID(), k.File)
		}
		seenKey[id] = true
	}
	return nil
}

// PlanWiring computes the typed diff for root without touching anything.
//
// Every failure that is a property of the REPOSITORY (a corrupted marker
// pair, a drifted region, an unparsable settings.json) becomes a BLOCKED
// item, not an error: one bad file must not stop fathom reporting on the
// others. An error return is reserved for failures of the CALL — a bad
// spec, an unreadable root.
func PlanWiring(root string, spec Spec) (Plan, error) {
	if err := spec.validate(); err != nil {
		return Plan{}, err
	}
	p := Plan{Root: root}

	ledger, lerr := LoadLedger(root)
	if lerr != nil {
		p.LedgerNote = lerr.Error() + " — fathom will treat itself as the owner of nothing here and add only what is missing"
	}

	for _, r := range spec.Regions {
		p.Items = append(p.Items, planRegion(root, r))
	}
	for _, k := range spec.Keys {
		p.Items = append(p.Items, planKey(root, k, ledger))
	}

	sort.SliceStable(p.Items, func(i, j int) bool {
		a, b := p.Items[i], p.Items[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Target < b.Target
	})
	enc, err := json.Marshal(p.Items)
	if err != nil {
		return Plan{}, err
	}
	p.Digest = digest(string(enc))
	return p, nil
}

func readFileIfPresent(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

func planRegion(root string, r Region) Item {
	it := Item{File: r.File, Target: "fathom:region", Kind: KindRegion, After: r.Body}
	path := filepath.Join(root, filepath.FromSlash(r.File))

	content, existed, err := readFileIfPresent(path)
	if err != nil {
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "cannot be read: "+err.Error()
		return it
	}
	ending := lineEnding(content)
	want := normalizeEndings(r.Body, ending)
	it.After = want

	if !existed {
		it.Verdict = VerdictApply
		it.Reason = r.File + " does not exist — fathom will create it carrying only its own region"
		return it
	}

	reg, perr := parseOwnedRegion(content, r.Syntax)
	if perr != nil {
		it.Verdict, it.Cause = VerdictBlocked, CauseUnreadable
		it.Reason = "the fathom marker pair is corrupted (" + perr.Error() + ") — fathom will not guess at a broken pair"
		return it
	}

	if !reg.Present {
		it.Verdict = VerdictApply
		it.Reason = "no fathom region in " + r.File + " yet — fathom will append one, leaving every existing byte in place"
		return it
	}
	it.Present, it.Before = true, reg.Body

	switch {
	case reg.Drifted():
		it.Verdict, it.Cause = VerdictBlocked, CauseDrift
		it.Reason = "the region was edited by hand since fathom wrote it (recorded " + short(reg.Digest) +
			", body now hashes to " + short(reg.BodyDigest) + ") — fathom will not merge; keep yours or take fathom's, explicitly"
	case !reg.Tracked() && reg.Body == want:
		it.Verdict = VerdictApply
		it.Reason = "the region predates marker format v3 but its body already matches the current template — fathom will re-mark it with a digest, changing no prose"
	case !reg.Tracked() && knownBody(r, reg.Body, ending):
		it.Verdict = VerdictApply
		it.Reason = "the region predates marker format v3 and its body matches a template fathom shipped — fathom will upgrade it to " + r.Version
	case !reg.Tracked() && r.AdoptUntracked:
		it.Verdict = VerdictApply
		it.Reason = "the region predates marker format v3 (marker pair present, no digest recorded) — fathom owns it by its markers and will upgrade it to " + r.Version +
			", recording a digest so any later hand edit is detected; this is the one-time migration, not drift"
	case !reg.Tracked():
		it.Verdict, it.Cause = VerdictBlocked, CauseUnproven
		it.Reason = "the region predates marker format v3 and its body matches no template fathom shipped — fathom cannot prove it wrote this and will not overwrite it"
	case reg.Body == want && reg.Version == r.Version:
		it.Verdict = VerdictSkip
		it.Reason = "already current (template " + r.Version + ")"
	default:
		from := reg.Version
		if from == "" {
			from = "an unrecorded version"
		}
		it.Verdict = VerdictApply
		it.Reason = "fathom's own region is out of date — upgrading from template " + from + " to " + r.Version
	}
	return it
}

func knownBody(r Region, body, ending string) bool {
	for _, k := range r.Known {
		if normalizeEndings(k, ending) == body {
			return true
		}
	}
	return false
}

func short(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	if len(d) > 8 {
		d = d[:8]
	}
	return "sha256:" + d
}

func planKey(root string, k Key, ledger Ledger) Item {
	it := Item{File: k.File, Target: k.ID(), Kind: KindKey}
	want, err := k.desired()
	if err != nil {
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnproven, err.Error()
		return it
	}
	it.After = want

	path := filepath.Join(root, filepath.FromSlash(k.File))
	content, existed, rerr := readFileIfPresent(path)
	if rerr != nil {
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "cannot be read: "+rerr.Error()
		return it
	}
	if !existed {
		it.Verdict = VerdictApply
		it.Reason = k.File + " does not exist — fathom will create it carrying only its own entry"
		return it
	}

	current, present, cerr := k.current(content)
	if cerr != nil {
		it.Verdict, it.Cause = VerdictBlocked, CauseUnreadable
		it.Reason = k.File + " could not be read as the structure fathom expects (" + cerr.Error() + ") — fathom will not rewrite a file it cannot parse"
		return it
	}
	if !present {
		it.Verdict = VerdictApply
		it.Reason = "absent — fathom will add " + k.ID() + " and record it in " + ledgerRelPath
		return it
	}
	it.Present, it.Before = true, current

	owned, isOwned := ownedKey(ledger, k.File, k.ID())
	switch {
	case !isOwned && !k.AdoptUntracked:
		it.Verdict = VerdictSkip
		it.Reason = k.ID() + " is present but " + ledgerRelPath + " does not record fathom adding it — it is someone else's and fathom will not touch it"
	case !isOwned && current == want:
		it.Verdict = VerdictSkip
		it.Reason = "already current (" + ledgerRelPath + " does not record it; fathom will record ownership when it next has a reason to write)"
	case !isOwned:
		it.Verdict = VerdictApply
		it.Reason = "fathom's own entry (identified by its marker, unrecorded in " + ledgerRelPath + ") is out of date — updating it to the current template and recording ownership"
	case owned.Digest != digest(current):
		it.Verdict, it.Cause = VerdictBlocked, CauseDrift
		it.Reason = "the entry changed since fathom wrote it on " + orUnrecorded(owned.Wrote) + " (recorded " + short(owned.Digest) +
			", now " + short(digest(current)) + ") — fathom will not merge"
	case current == want:
		it.Verdict = VerdictSkip
		it.Reason = "already current"
	default:
		it.Verdict = VerdictApply
		it.Reason = "fathom's own entry is out of date — updating it to the current template"
	}
	return it
}

func orUnrecorded(s string) string {
	if s == "" {
		return "an unrecorded date"
	}
	return s
}
