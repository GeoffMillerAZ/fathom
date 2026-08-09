// Package wiring is phase U's engine (docs/design/self-healing-fleet.md):
// fathom edits the repositories it is wired into, and law 15 bounds exactly
// what it may touch.
//
//	Law 15. When fathom modifies a file it did not create, it writes only
//	inside a region it marked as its own. If that region has drifted from
//	what fathom last wrote, reconciliation stops and reports the drift; it
//	never merges, never overwrites, and never "helpfully" resolves. Every
//	write produces a receipt sufficient to undo it.
//
// The asymmetry is the whole argument for how conservative this package is.
// A false stop costs a human one diff and one more click. A false merge
// silently destroys a hand-tuned configuration — possibly across seventeen
// repositories in one action, possibly discovered weeks later. Those are
// not comparable, so no cleverness is warranted anywhere in this package:
// when fathom cannot prove it wrote something, it stops and reports.
//
// Three tiers of ownership, three mechanisms:
//
//   - Tier 1, files fathom created: whole-file authority (not modelled here
//     yet; it is the easy case and still writes a receipt).
//   - Tier 2, marked regions in shared text files (CLAUDE.md, AGENTS.md,
//     .gitignore): marker format v3 carries the template version AND the
//     sha256 of the body as fathom last wrote it. See region.go.
//   - Tier 3, structured files owned by other tools (lefthook.yml,
//     .claude/settings.json): there is nowhere to put a comment marker in
//     JSON and a YAML comment is too fragile to trust with someone's CI, so
//     ownership is keyed and the key is recorded out of band in
//     .fathom/owned.json. See ledger.go and structured.go.
package wiring

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// MarkerSyntax is how a marked region is delimited in one file's comment
// syntax. HTML comments serve Markdown; hash comments serve .gitignore.
type MarkerSyntax struct {
	BeginPrefix string // e.g. "<!-- fathom:begin"
	BeginSuffix string // e.g. " -->" ("" where the syntax has no closer)
	End         string // e.g. "<!-- fathom:end -->"
}

// HTMLMarkers delimits regions in Markdown files (CLAUDE.md, AGENTS.md).
var HTMLMarkers = MarkerSyntax{
	BeginPrefix: "<!-- fathom:begin",
	BeginSuffix: " -->",
	End:         "<!-- fathom:end -->",
}

// HashMarkers delimits regions in hash-commented files (.gitignore).
var HashMarkers = MarkerSyntax{
	BeginPrefix: "# fathom:begin",
	BeginSuffix: "",
	End:         "# fathom:end",
}

// digest is the one hash spelling this package uses, everywhere: the
// "sha256:" prefix is part of the value so a digest is never mistaken for
// an opaque id, and a marker line reads as documentation.
func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ownedRegion is one marked region as it exists on disk RIGHT NOW, next to
// what the marker claims fathom last wrote.
//
// The two digests are the whole point of marker format v3. Digest is what
// fathom recorded when it wrote the region; BodyDigest is what the bytes
// between the markers hash to today. Equal means fathom's; different means
// a human has been inside fathom's region and law 15 applies.
type ownedRegion struct {
	Present    bool   // a well-formed marker pair was found
	Start      int    // byte offset of the begin marker line
	End        int    // byte offset just past the end marker (its terminator excluded)
	Version    string // template version recorded in the marker ("" for a pre-v3 marker)
	Digest     string // body digest recorded in the marker ("" for a pre-v3 marker)
	Body       string // the bytes between the markers, as they are today
	BodyDigest string // digest of Body
	Ending     string // the line terminator the region is written with
}

// Tracked reports whether the marker recorded a digest at all. A pre-v3
// marker did not, so fathom cannot prove it wrote the body — which is
// unknown provenance, NOT evidence of an edit. Law 3: the two are named
// separately because they demand different responses.
func (r ownedRegion) Tracked() bool { return r.Present && r.Digest != "" }

// Drifted reports the law-15 condition: fathom recorded a digest and the
// body no longer matches it.
func (r ownedRegion) Drifted() bool { return r.Tracked() && r.Digest != r.BodyDigest }

// parseOwnedRegion finds fathom's marked region in content and reports what
// the marker claims alongside what the bytes actually are.
//
// It refuses — returns an error, changing nothing — on every corrupted
// shape: one marker of the pair, the end before the begin, a second begin
// marker, or a begin marker line with no terminator. Guessing at a broken
// pair is how you get two BEGINs and an orphaned END; a file with NEITHER
// marker is not corruption, it is a repository fathom has not wired yet.
func parseOwnedRegion(content string, syn MarkerSyntax) (ownedRegion, error) {
	beginCount := strings.Count(content, syn.BeginPrefix)
	endCount := strings.Count(content, syn.End)
	// HashMarkers' End is not a prefix of its BeginPrefix and vice versa, and
	// HTMLMarkers' likewise, so the two counts are independent.

	switch {
	case beginCount == 0 && endCount == 0:
		return ownedRegion{}, nil
	case beginCount > 1:
		return ownedRegion{}, fmt.Errorf("found %d %q markers — fathom owns at most one region per file; fix the file by hand", beginCount, syn.BeginPrefix)
	case endCount > 1:
		return ownedRegion{}, fmt.Errorf("found %d %q markers — fathom owns at most one region per file; fix the file by hand", endCount, syn.End)
	case beginCount == 0:
		return ownedRegion{}, fmt.Errorf("found %q with no matching %q — fix or remove the stray marker by hand", syn.End, syn.BeginPrefix)
	case endCount == 0:
		return ownedRegion{}, fmt.Errorf("found %q with no matching %q — fix or remove the stray marker by hand", syn.BeginPrefix, syn.End)
	}

	begin := strings.Index(content, syn.BeginPrefix)
	end := strings.Index(content, syn.End)
	if end < begin {
		return ownedRegion{}, fmt.Errorf("%q appears before %q — fix the marker order by hand", syn.End, syn.BeginPrefix)
	}

	nl := strings.IndexByte(content[begin:], '\n')
	if nl < 0 {
		return ownedRegion{}, fmt.Errorf("the %q line has no terminator — fix the file by hand", syn.BeginPrefix)
	}
	markerLine := content[begin : begin+nl]
	bodyStart := begin + nl + 1
	if bodyStart > end {
		return ownedRegion{}, fmt.Errorf("%q is inside the %q line — fix the file by hand", syn.End, syn.BeginPrefix)
	}

	ending := "\n"
	if strings.HasSuffix(markerLine, "\r") {
		ending = "\r\n"
		markerLine = strings.TrimSuffix(markerLine, "\r")
	}

	raw := content[bodyStart:end]
	body := strings.TrimSuffix(raw, ending)

	version, dig := parseMarkerAttrs(markerLine, syn)
	return ownedRegion{
		Present:    true,
		Start:      begin,
		End:        end + len(syn.End),
		Version:    version,
		Digest:     dig,
		Body:       body,
		BodyDigest: digest(body),
		Ending:     ending,
	}, nil
}

// parseMarkerAttrs reads the two attributes marker format v3 carries: the
// template version that produced the region ("v3") and the digest of the
// body as fathom wrote it. A pre-v3 marker carries neither and yields two
// empty strings — the honest answer, not a guess.
func parseMarkerAttrs(line string, syn MarkerSyntax) (version, dig string) {
	attrs := strings.TrimPrefix(line, syn.BeginPrefix)
	if syn.BeginSuffix != "" {
		attrs = strings.TrimSuffix(strings.TrimRight(attrs, " \t"), strings.TrimRight(syn.BeginSuffix, " \t"))
	}
	for _, tok := range strings.Fields(attrs) {
		switch {
		case strings.HasPrefix(tok, "sha256:"):
			dig = tok
		case len(tok) > 1 && tok[0] == 'v' && isDigits(tok[1:]):
			version = tok
		}
	}
	return version, dig
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// renderRegion writes a marker-format-v3 region: the begin marker carries
// the template version and the digest of the body BELOW it, then the body,
// then the end marker. The digest is computed over the body exactly as
// written (line endings included), so a re-parse of the rendered text
// reports Drifted() == false and any subsequent edit reports true.
func renderRegion(syn MarkerSyntax, version, body, ending string) string {
	body = normalizeEndings(body, ending)
	begin := syn.BeginPrefix + " " + version + " " + digest(body) + syn.BeginSuffix
	return begin + ending + body + ending + syn.End
}

// normalizeEndings rewrites body's line terminators to ending, so a region
// written into a CRLF file is CRLF throughout and its digest is stable
// across re-reads.
func normalizeEndings(body, ending string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if ending == "\n" {
		return body
	}
	return strings.ReplaceAll(body, "\n", ending)
}

// lineEnding reports the terminator a file is written with, judged by its
// first line — the only judgement available that does not require guessing
// about mixed files.
func lineEnding(content string) string {
	if i := strings.IndexByte(content, '\n'); i > 0 && content[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}
