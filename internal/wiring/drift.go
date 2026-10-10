package wiring

// drift.go — PC-28 (docs/plans/proof-credibility.md, vision 26 §1.4): the
// law-15 digest machinery extended to the two things it did not cover —
// MCP server entries in the harness config files each descriptor names,
// and fathom-owned skill files. The year's worst agent incidents were
// failures of trust configuration (a package that turned exfiltrating, a
// skill whose frontmatter registered hooks on invocation); an MCP entry
// whose command, args or env NAMES changed since fathom last saw it is the
// same shape. Fathom records a baseline digest in .fathom/owned.json and
// REPORTS a change by kind and path — it never merges, never rewrites the
// entry, and never digests an env or header VALUE (a secret must not leak
// into a committed ledger through its hash).

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Drift kinds, in the order they are reported.
const (
	DriftKindHook   = "hook"
	DriftKindStanza = "stanza"
	DriftKindMCP    = "mcp"
	DriftKindSkill  = "skill"
)

// Drift is one changed thing fathom owns or baselines: Kind names the
// machinery, Path the file (as the descriptor spells it: "~/.claude.json",
// ".mcp.json", ".claude/skills/x/SKILL.md"), Target the entry inside it.
type Drift struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Target string `json:"target,omitempty"`
	Reason string `json:"reason"`
}

// DriftReport is the answer to "did anything fathom trusts change?".
// Unreadable is I38's list: a file that EXISTS and could not be read is
// named, never counted as current. Unrecorded names entries fathom can see
// but has no baseline for — not drift; `fleet reconcile --apply` records
// them.
type DriftReport struct {
	Drifts     []Drift  `json:"drifts,omitempty"`
	Unreadable []string `json:"unreadable,omitempty"`
	Unrecorded []string `json:"unrecorded,omitempty"`
	Checked    int      `json:"checked"`
}

// MCPFile is one harness MCP config file to baseline: Path is absolute
// (the caller expands "~"), Label is how the descriptor spells it and is
// the ledger's file key, KeyPath addresses the fathom entry ("mcpServers",
// "fathom"), Format is "json" or "toml".
type MCPFile struct {
	Path    string
	Label   string
	Format  string
	KeyPath []string
}

// SkillFile is one fathom-owned skill file (its content carries the owned
// marker): Path absolute, Label repo-relative (or "~/…"), the ledger key.
// Shipped is what this binary renders for that path (nil when unknown): a
// file equal to it is fathom's own content, so a baseline that lags it —
// .fathom/owned.json is per checkout, the shipped skill changes in git — is
// not drift, and recording it is what `fathom init --skills` would do.
type SkillFile struct {
	Path    string
	Label   string
	Shipped []byte
	// Known holds the digests of renders earlier fathom releases shipped
	// for this file (FA-24): content equal to one is fathom's own older
	// render — stale, never a hand edit.
	Known map[string]bool
}

// isShipped reports whether data is exactly the rendering this binary ships.
func (f SkillFile) isShipped(data []byte) bool {
	return f.Shipped != nil && bytes.Equal(data, f.Shipped)
}

// isFathomRender reports whether data is a render fathom shipped: this
// binary's, or one an earlier release shipped (Known).
func (f SkillFile) isFathomRender(data []byte) bool {
	return f.isShipped(data) || f.Known[digest(string(data))]
}

// fathomOwn reports whether data is fathom's own content for f: a render
// fathom shipped, or exactly the bytes the ledger records fathom writing as
// the whole owned file (ownedFileID, the planner's record of its write).
// Content that is fathom's own is never drift, and its lagging baseline is
// re-recorded without --rebaseline.
func (f SkillFile) fathomOwn(ledger Ledger, data []byte) bool {
	if f.isFathomRender(data) {
		return true
	}
	k, ok := ownedKey(ledger, f.Label, ownedFileID)
	return ok && k.Digest == digest(string(data))
}

// ledger ids for the two new kinds; hooks and stanzas keep theirs.
func mcpKeyID(keyPath []string) string { return "mcp|" + strings.Join(keyPath, ".") }

// skillKeyID is DriftSkills' content baseline for a skill file. For a file
// the planner owns whole (ownedfile.go) it watches the same bytes as
// ownedFileID, so every write moves both (FA-23).
const skillKeyID = "skill"

// mcpEntryDigest canonicalizes one MCP server entry and digests it. Kept
// verbatim: the transport-bearing fields (type, transport, url, httpUrl,
// command, args). Reduced to sorted NAMES: env, headers, and any other
// object-valued field. Every other key contributes its name only. So a
// rotated token, a changed header value or a re-ordered env block is not
// drift, while a new env NAME, a changed argv or a changed URL is.
func mcpEntryDigest(entry map[string]any) string {
	canon := map[string]any{}
	names := make([]string, 0, len(entry))
	for k := range entry {
		names = append(names, k)
	}
	sort.Strings(names)
	var other []string
	for _, k := range names {
		v := entry[k]
		switch k {
		case "type", "transport", "url", "httpUrl", "command":
			canon[k] = fmt.Sprint(v)
		case "args":
			canon[k] = stringList(v)
		case "env", "headers":
			canon[k+"_names"] = objectKeys(v)
		default:
			if _, isObj := v.(map[string]any); isObj {
				canon[k+"_names"] = objectKeys(v)
			} else {
				other = append(other, k)
			}
		}
	}
	if len(other) > 0 {
		canon["other_keys"] = other
	}
	b, err := json.Marshal(canon)
	if err != nil {
		return ""
	}
	return digest(string(b))
}

func stringList(v any) []string {
	out := []string{}
	switch list := v.(type) {
	case []any:
		for _, e := range list {
			out = append(out, fmt.Sprint(e))
		}
	case []string:
		out = append(out, list...)
	default:
		if v != nil {
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
}

func objectKeys(v any) []string {
	out := []string{}
	switch m := v.(type) {
	case map[string]any:
		for k := range m {
			out = append(out, k)
		}
	case map[string]string:
		for k := range m {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// MCPEntryDigest parses content in format and returns the digest of the
// entry at keyPath. present=false with a nil error means the file parsed
// but carries no such entry; an error means the file could not be read as
// the structure fathom expects.
func MCPEntryDigest(content []byte, format string, keyPath []string) (dig string, present bool, err error) {
	var root map[string]any
	switch format {
	case "json":
		obj, perr := parseJSONObject(string(content))
		if perr != nil {
			return "", false, perr
		}
		root = obj
	case "toml":
		if terr := toml.Unmarshal(content, &root); terr != nil {
			return "", false, terr
		}
	default:
		return "", false, fmt.Errorf("unknown MCP config format %q", format)
	}
	cur := any(root)
	for _, k := range keyPath {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false, nil
		}
		cur, ok = m[k]
		if !ok {
			return "", false, nil
		}
	}
	entry, ok := cur.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("%s is not an object", strings.Join(keyPath, "."))
	}
	return mcpEntryDigest(entry), true, nil
}

// readFn is how drift reads files; tests inject one, callers pass
// os.ReadFile (or a read-manifest-recording wrapper).
type readFn func(path string) ([]byte, error)

// DriftMCP compares every MCP file's fathom entry to the ledger's recorded
// baseline. A missing file or entry is not drift (fathom never required
// the registration); an unreadable file is named (I38).
func DriftMCP(ledger Ledger, files []MCPFile, read readFn) DriftReport {
	var rep DriftReport
	if read == nil {
		read = os.ReadFile
	}
	for _, f := range files {
		data, err := read(f.Path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				rep.Unreadable = append(rep.Unreadable, f.Label+" — "+errnoText(err))
			}
			continue
		}
		rep.Checked++
		dig, present, perr := MCPEntryDigest(data, f.Format, f.KeyPath)
		if perr != nil {
			rep.Unreadable = append(rep.Unreadable, f.Label+" — could not be read as "+f.Format+": "+perr.Error())
			continue
		}
		if !present {
			continue
		}
		id := mcpKeyID(f.KeyPath)
		owned, recorded := ownedKey(ledger, f.Label, id)
		if !recorded {
			rep.Unrecorded = append(rep.Unrecorded, DriftKindMCP+" "+f.Label+" "+strings.Join(f.KeyPath, "."))
			continue
		}
		if owned.Digest != dig {
			rep.Drifts = append(rep.Drifts, Drift{
				Kind: DriftKindMCP, Path: f.Label, Target: strings.Join(f.KeyPath, "."),
				Reason: "the entry's transport, argv or env/header NAMES changed since fathom recorded it on " + orUnrecorded(owned.Wrote) +
					" (recorded " + short(owned.Digest) + ", now " + short(dig) + ") — fathom will not merge; re-record with fleet reconcile --apply if the change is yours",
			})
		}
	}
	sortReport(&rep)
	return rep
}

// DriftSkills compares every owned skill file's content digest to the
// ledger's baseline. Only files carrying the owned marker are passed in;
// a file with no baseline is unrecorded, not drift.
func DriftSkills(ledger Ledger, files []SkillFile, read readFn) DriftReport {
	var rep DriftReport
	if read == nil {
		read = os.ReadFile
	}
	for _, f := range files {
		data, err := read(f.Path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				rep.Unreadable = append(rep.Unreadable, f.Label+" — "+errnoText(err))
			}
			continue
		}
		rep.Checked++
		dig := digest(string(data))
		owned, recorded := ownedKey(ledger, f.Label, skillKeyID)
		if !recorded {
			rep.Unrecorded = append(rep.Unrecorded, DriftKindSkill+" "+f.Label)
			continue
		}
		if owned.Digest != dig && !f.fathomOwn(ledger, data) {
			rep.Drifts = append(rep.Drifts, Drift{
				Kind: DriftKindSkill, Path: f.Label,
				Reason: "the skill file changed since fathom recorded it on " + orUnrecorded(owned.Wrote) +
					" (recorded " + short(owned.Digest) + ", now " + short(dig) + ") — a skill's frontmatter can register hooks; fathom will not merge",
			})
		}
	}
	sortReport(&rep)
	return rep
}

// DriftFromPlan lifts a Plan's law-15 verdicts into the same vocabulary:
// a Blocked item with CauseDrift is a hook or stanza drift; CauseUnreadable
// is an unreadable file.
func DriftFromPlan(p Plan) DriftReport {
	var rep DriftReport
	for _, it := range p.Items {
		rep.Checked++
		if it.Verdict != VerdictBlocked {
			continue
		}
		kind := DriftKindHook
		switch it.Kind {
		case KindRegion:
			kind = DriftKindStanza
		case KindFile, KindLink:
			kind = DriftKindSkill // owned skill files and the .agents/skills link (ADR-059)
		}
		switch it.Cause {
		case CauseDrift:
			rep.Drifts = append(rep.Drifts, Drift{Kind: kind, Path: it.File, Target: it.Target, Reason: it.Reason})
		case CauseUnreadable:
			rep.Unreadable = append(rep.Unreadable, it.File+" — "+it.Reason)
		}
	}
	sortReport(&rep)
	return rep
}

// Merge folds other into rep, keeping every list sorted.
func (rep *DriftReport) Merge(other DriftReport) {
	rep.Drifts = append(rep.Drifts, other.Drifts...)
	rep.Unreadable = append(rep.Unreadable, other.Unreadable...)
	rep.Unrecorded = append(rep.Unrecorded, other.Unrecorded...)
	rep.Checked += other.Checked
	sortReport(rep)
}

// Lines renders each drift as "kind path[ target] — reason", sorted.
func (rep DriftReport) Lines() []string {
	out := make([]string, 0, len(rep.Drifts))
	for _, d := range rep.Drifts {
		line := d.Kind + " " + d.Path
		if d.Target != "" {
			line += " " + d.Target
		}
		out = append(out, line+" — "+d.Reason)
	}
	return out
}

func kindRank(k string) int {
	switch k {
	case DriftKindHook:
		return 0
	case DriftKindStanza:
		return 1
	case DriftKindMCP:
		return 2
	case DriftKindSkill:
		return 3
	}
	return 4
}

func sortReport(rep *DriftReport) {
	sort.SliceStable(rep.Drifts, func(i, j int) bool {
		a, b := rep.Drifts[i], rep.Drifts[j]
		if a.Kind != b.Kind {
			return kindRank(a.Kind) < kindRank(b.Kind)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Target < b.Target
	})
	sort.Strings(rep.Unreadable)
	sort.Strings(rep.Unrecorded)
}

// RecordBaselines records a ledger digest for every MCP entry and owned
// skill file that has none yet (trust on first sight — the entry was
// registered by the operator's own `mcp add` or written by `fathom init`,
// so fathom's first look is the baseline) and saves the ledger. Recorded
// entries already in the ledger are left alone: re-recording a drifted
// entry is the operator's explicit choice (`--rebaseline`), never a side
// effect. Returns the labels recorded, sorted.
func RecordBaselines(root string, mcp []MCPFile, skills []SkillFile, wrote string, read readFn, rebaseline bool) ([]string, error) {
	recorded, _, err := RecordBaselinesDetail(root, mcp, skills, wrote, read, rebaseline)
	return recorded, err
}

// RecordBaselinesDetail is RecordBaselines naming what it refused (FA-24):
// a skill file's baseline moves to content that is fathom's own (a render
// fathom shipped, or the bytes the ledger records it writing) with or
// without rebaseline, because a lagging baseline under fathom's own bytes
// is not a change anyone made; with rebaseline, any other skill content is
// refused, stays DRIFT and is named — under fathom's marker the remedy is
// to delete the marker line (adopt the file) or remove it (take fathom's),
// never to bless the bytes. An MCP entry is re-recorded on rebaseline as
// before: fathom renders none, the entry is the operator's own `mcp add`.
func RecordBaselinesDetail(root string, mcp []MCPFile, skills []SkillFile, wrote string, read readFn, rebaseline bool) (recorded, refused []string, err error) {
	if read == nil {
		read = os.ReadFile
	}
	ledger, err := LoadLedger(root)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range mcp {
		data, rerr := read(f.Path)
		if rerr != nil {
			continue
		}
		dig, present, perr := MCPEntryDigest(data, f.Format, f.KeyPath)
		if perr != nil || !present {
			continue
		}
		id := mcpKeyID(f.KeyPath)
		if _, has := ownedKey(ledger, f.Label, id); has && !rebaseline {
			continue
		}
		ledger.Record(f.Label, OwnedKey{ID: id, Digest: dig, Wrote: wrote})
		recorded = append(recorded, DriftKindMCP+" "+f.Label+" "+strings.Join(f.KeyPath, "."))
	}
	for _, f := range skills {
		data, rerr := read(f.Path)
		if rerr != nil {
			continue
		}
		dig := digest(string(data))
		if owned, has := ownedKey(ledger, f.Label, skillKeyID); has {
			switch {
			case owned.Digest == dig:
				continue
			case f.fathomOwn(ledger, data):
				// fathom's own bytes under a lagging baseline: re-record.
			case rebaseline:
				refused = append(refused, DriftKindSkill+" "+f.Label+" — not a render fathom shipped, so --rebaseline will not record it (recorded "+short(owned.Digest)+", now "+short(dig)+"); delete the fathom:owned line to keep your edit, or remove the file to take fathom's")
				continue
			default:
				continue
			}
		}
		ledger.Record(f.Label, OwnedKey{ID: skillKeyID, Digest: dig, Wrote: wrote})
		recorded = append(recorded, DriftKindSkill+" "+f.Label)
	}
	sort.Strings(recorded)
	sort.Strings(refused)
	if len(recorded) == 0 {
		return nil, refused, nil
	}
	if err := SaveLedger(root, ledger); err != nil {
		return nil, refused, err
	}
	return recorded, refused, nil
}

// RecordWritten records, after a writer outside the wiring planner wrote
// them (`fathom init --skills`, FA-23), the baselines of every file in
// files from the bytes now on disk: the skill baseline DriftSkills reads
// and, for a repository file (a label not under "~/"), the owned-file
// baseline the planner reads. A file whose bytes are not the render this
// binary ships is left alone: only bytes fathom wrote become fathom's
// baseline. Returns the labels whose baselines moved, sorted.
func RecordWritten(root string, files []SkillFile, wrote string) ([]string, error) {
	ledger, err := LoadLedger(root)
	if err != nil {
		return nil, err
	}
	var recorded []string
	for _, f := range files {
		data, rerr := os.ReadFile(f.Path)
		if rerr != nil || !f.isShipped(data) {
			continue
		}
		dig := digest(string(data))
		ids := []string{skillKeyID}
		if !strings.HasPrefix(f.Label, "~/") {
			ids = append(ids, ownedFileID)
		}
		moved := false
		for _, id := range ids {
			if k, ok := ownedKey(ledger, f.Label, id); ok && k.Digest == dig {
				continue
			}
			ledger.Record(f.Label, OwnedKey{ID: id, Digest: dig, Wrote: wrote})
			moved = true
		}
		if moved {
			recorded = append(recorded, DriftKindSkill+" "+f.Label)
		}
	}
	sort.Strings(recorded)
	if len(recorded) == 0 {
		return nil, nil
	}
	if err := SaveLedger(root, ledger); err != nil {
		return nil, err
	}
	return recorded, nil
}

// RecordKeyBaselines re-records the ledger digest for every Key currently
// BLOCKED with CauseDrift — a structured-file entry (a Claude Code hook row
// is the shipping case) whose on-disk value no longer matches what fathom
// recorded in .fathom/owned.json. It is the Key counterpart of
// RecordBaselines' MCP/skill rebaseline: it writes nothing to the file
// itself, ever — only the ledger entry moves, to the digest of the value AS
// IT NOW STANDS, because a rebaseline is the operator saying the change is
// theirs, not fathom re-asserting its own template. An entry that is not
// currently drifted (no baseline yet, or baseline already matches) is left
// alone: a fresh entry is baselined by the ordinary apply path the first
// time fathom writes it, never by rebaseline.
//
// rebaseline=false is a deliberate no-op (law 15: fathom never re-records a
// drifted entry on its own, only on the operator's explicit --rebaseline),
// so callers may pass it unconditionally in the same request that also
// baselines MCP/skill entries. Returns the labels rebaselined, sorted.
func RecordKeyBaselines(root string, keys []Key, wrote string, read readFn, rebaseline bool) ([]string, error) {
	if !rebaseline || len(keys) == 0 {
		return nil, nil
	}
	if read == nil {
		read = os.ReadFile
	}
	ledger, err := LoadLedger(root)
	if err != nil {
		return nil, err
	}
	var recorded []string
	for _, k := range keys {
		data, rerr := read(filepath.Join(root, filepath.FromSlash(k.File)))
		if rerr != nil {
			continue
		}
		current, present, cerr := k.current(string(data))
		if cerr != nil || !present {
			continue
		}
		id := k.ID()
		owned, has := ownedKey(ledger, k.File, id)
		if !has || owned.Digest == digest(current) {
			continue // unrecorded or not drifted: nothing for rebaseline to do
		}
		ledger.Record(k.File, OwnedKey{ID: id, Digest: digest(current), Wrote: wrote})
		recorded = append(recorded, DriftKindHook+" "+k.File+" "+id)
	}
	sort.Strings(recorded)
	if len(recorded) == 0 {
		return nil, nil
	}
	if err := SaveLedger(root, ledger); err != nil {
		return nil, err
	}
	return recorded, nil
}

func errnoText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}
