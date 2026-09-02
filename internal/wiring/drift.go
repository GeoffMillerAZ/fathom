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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
type SkillFile struct {
	Path  string
	Label string
}

// ledger ids for the two new kinds; hooks and stanzas keep theirs.
func mcpKeyID(keyPath []string) string { return "mcp|" + strings.Join(keyPath, ".") }

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
// os.ReadFile (or a readset-recording wrapper).
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
		if owned.Digest != dig {
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
		if it.Kind == KindRegion {
			kind = DriftKindStanza
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
	if read == nil {
		read = os.ReadFile
	}
	ledger, err := LoadLedger(root)
	if err != nil {
		return nil, err
	}
	var recorded []string
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
		if _, has := ownedKey(ledger, f.Label, skillKeyID); has && !rebaseline {
			continue
		}
		ledger.Record(f.Label, OwnedKey{ID: skillKeyID, Digest: digest(string(data)), Wrote: wrote})
		recorded = append(recorded, DriftKindSkill+" "+f.Label)
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
