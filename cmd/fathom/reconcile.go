package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// fathom fleet reconcile (vision 14 §7, plan IO-17/18): wiring is
// reconciled, not scaffolded — a drifted hook stack is a DIFF to apply,
// never a refusal to touch. Dry-run by default; --apply applies the SAFE
// subset only: repointing stale absolute binary paths to the shim-stable
// `fathom`, adding missing template entries (project scope, deferring to
// user scope — one scope, never both), and refreshing the stanza regions
// fathom already owns. Structural surgery (scope moves, removals) stays a
// human decision.
//
// Every write goes through internal/wiring as ONE plan → apply per
// repository (INC-12, law 15): a stanza region fathom is TRACKING (marker
// format v3, digest recorded) that a human edited is reported BLOCKED and
// never merged — the other files in the same repository still apply — and
// each apply lands one receipt in .fathom/receipts.jsonl that `fathom
// fleet undo` reverses. A pre-v3 region (marker pair, no digest) is
// fathom's by its markers and is upgraded on the first write; that is the
// migration, not drift. --apply exits 1 when any file was BLOCKED.

// wiringDiff is one repo's drift against the current template.
type wiringDiff struct {
	Repo          string   `json:"repo"`
	Service       string   `json:"service,omitempty"`        // the registry's id, when known
	Missing       []string `json:"missing,omitempty"`        // template entries absent at every readable scope
	StaleCmds     []string `json:"stale_cmds,omitempty"`     // commands spelling an absolute fathom path
	MissingSkills []string `json:"missing_skills,omitempty"` // shipped skill files absent (FR-3; RP-4: .claude/skills and .agents/skills)
	StaleSkills   []string `json:"stale_skills,omitempty"`   // marker-owned skill files behind the shipped source (RP-4)
	// BlockedSkills names the skill files law 15 stops on — "FILE — why": a
	// hand-authored file by a skill's name (no marker), or a hand edit
	// under the marker since fathom wrote it. --apply leaves them alone.
	BlockedSkills []string `json:"blocked_skills,omitempty"`
	StanzaNote    string   `json:"stanza_note,omitempty"`   // pre-rename branding etc.
	StaleStanzas  []string `json:"stale_stanzas,omitempty"` // marker-owned regions whose content != the current template
	// RemarkStanzas are owned regions whose PROSE already equals the
	// current template but whose marker predates format v3 (no digest) or
	// records an older template version: up to date, NOT stale — --apply
	// re-marks them with a digest and changes no prose. Reporting these as
	// "behind the template" told a reader a rewrite was coming when only a
	// marker line was.
	RemarkStanzas []string `json:"remark_stanzas,omitempty"`
	// DriftedStanzas names the files law 15 stops on — "FILE — why": a
	// tracked region edited by hand, or a corrupted marker pair. --apply
	// reports them and writes nothing to those files.
	DriftedStanzas []string `json:"drifted_stanzas,omitempty"`
	SettingsNote   string   `json:"settings_note,omitempty"` // unreadable / absent
}

// staleFathomPath matches an absolute path ending in /fathom (optionally
// bin/fathom) inside a hook command — the class every audit found: hooks
// pinned to a checkout's bin/ that drifts from the deployed binary.
var staleFathomPath = regexp.MustCompile(`(^|[\s"'=])(/[^\s"']*/fathom)([\s"']|$)`)

// analyzeWiring diffs one repo's installed hooks against fathomHookSpecs.
// User-scope entries count as present (they serve every repo), but stale
// paths are reported wherever they appear.
func analyzeWiring(root string) wiringDiff { return analyzeWiringFor(root, "") }

// analyzeWiringFor is analyzeWiring with the registry's service id, which
// the stanza comparison must use.
func analyzeWiringFor(root, service string) wiringDiff {
	d := wiringDiff{Repo: root, Service: service}
	scopes := [][2]string{{"project", filepath.Join(root, ".claude", "settings.json")}}
	if home, err := os.UserHomeDir(); err == nil {
		scopes = append(scopes, [2]string{"user", filepath.Join(home, ".claude", "settings.json")})
	}

	type installed struct{ present bool }
	have := map[string]*installed{}
	for i := range fathomHookSpecs {
		s := fathomHookSpecs[i]
		have[s.event+"|"+s.matcher] = &installed{}
	}
	readable := false
	for _, sc := range scopes {
		data, err := os.ReadFile(sc[1])
		if err != nil {
			continue
		}
		readable = true
		var rootObj map[string]any
		if json.Unmarshal(data, &rootObj) != nil {
			d.SettingsNote = sc[0] + " settings.json unparsable"
			continue
		}
		hooksObj, _ := rootObj["hooks"].(map[string]any)
		for _, spec := range fathomHookSpecs {
			key := spec.event + "|" + spec.matcher
			arr, _ := hooksObj[spec.event].([]any)
			for _, g := range arr {
				gm, ok := g.(map[string]any)
				if !ok {
					continue
				}
				gMatcher, _ := gm["matcher"].(string)
				if gMatcher != spec.matcher {
					continue
				}
				hs, _ := gm["hooks"].([]any)
				for _, h := range hs {
					hm, ok := h.(map[string]any)
					if !ok {
						continue
					}
					cmdStr, _ := hm["command"].(string)
					if strings.Contains(cmdStr, spec.marker) {
						have[key].present = true
						if m := staleFathomPath.FindStringSubmatch(cmdStr); m != nil {
							d.StaleCmds = append(d.StaleCmds, fmt.Sprintf("%s scope %s[%s]: %s", sc[0], spec.event, spec.matcher, cmdStr))
						}
					}
				}
			}
		}
	}
	if !readable {
		d.SettingsNote = "no readable settings.json at project or user scope"
	}
	for _, spec := range fathomHookSpecs {
		if !have[spec.event+"|"+spec.matcher].present {
			d.Missing = append(d.Missing, spec.event+"["+spec.matcher+"] → fathom "+spec.sub)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.StaleCmds)

	// Shipped-skills drift (FR-3, docs/plans/exploration-ledger.md; RP-4,
	// docs/plans/reach-parity.md): a repo whose hooks are current can still
	// fail sessions on Unknown-skill — repos wired before a skill shipped
	// never receive it — and a skill fathom rendered once drifts from the
	// shipped source with every release. Both repo-scoped shapes are
	// planned through the wiring engine (reconcile_skills.go); the
	// user-global prompts twin is not a per-repo drift signal.
	d.MissingSkills, d.StaleSkills, d.BlockedSkills = skillDriftFiles(root)

	if cm, err := os.ReadFile(filepath.Join(root, "CLAUDE.md")); err == nil {
		low := strings.ToLower(string(cm))
		if strings.Contains(low, ".sextantignore") || strings.Contains(low, "sextant mcp") {
			d.StanzaNote = "CLAUDE.md carries pre-rename branding (sextant) — regenerate with fathom gen-claude-md"
		}
	}

	// Stanza CONTENT drift (found 2026-08-13): the template gained the
	// continuity/Codex line and reconcile reported "wiring current" for
	// every repo — it checked stanza PRESENCE, never content against the
	// current template. A template change that no reconcile can deliver is
	// a template change that never ships. Compared per-file (CLAUDE.md and
	// AGENTS.md independently), rendered with the REGISTRY's service id
	// (dirname only as a daemon-unreachable fallback) so a repo that chose
	// a different id is neither falsely flagged nor rewritten wrongly.
	d.StaleStanzas, d.RemarkStanzas, d.DriftedStanzas = staleStanzaFiles(root, d.Service)
	return d
}

// stanzaFiles are the files that carry the operating-contract stanza.
var stanzaFiles = []string{"CLAUDE.md", "AGENTS.md"}

// stanzaServiceID resolves the service id the stanza is rendered with: the
// registry's, else the directory basename as a daemon-unreachable fallback.
func stanzaServiceID(root, service string) string {
	if service == "" {
		return strings.ToLower(filepath.Base(root))
	}
	return service
}

// stanzaPlan is the wiring plan for the stanza regions of one repository —
// the same plan --apply executes, computed without touching anything.
func stanzaPlan(root, service string) (wiring.Plan, error) {
	svc := stanzaServiceID(root, service)
	intentOn := intentChannelWired(root)
	spec := wiring.Spec{}
	for _, name := range stanzaFiles {
		spec.Regions = append(spec.Regions, stanzaRegion(name, svc, intentOn))
	}
	return wiring.PlanWiring(root, spec)
}

// staleStanzaFiles reports which of the repo's stanza-carrying files hold a
// fathom-owned region whose content differs from the current template
// (stale — refreshed on --apply), which hold one whose prose already equals
// it but whose marker will be re-marked with a digest (remark — up to date,
// no prose changes on --apply), and which ones law 15 stops on (drifted —
// "FILE — why", never written). Files without markers are NOT reported —
// wiring a never-wired repo is init's job; reconcile only refreshes what
// fathom already owns (the marker pair IS the ownership claim, and a
// tracked region's digest is the proof it is still fathom's).
//
// Stale and remark are both VerdictApply on a present region; they part on
// whether the prose changes: the engine's Before is the body on disk and
// After is the template, and equal bodies mean the only write is the marker
// line.
func staleStanzaFiles(root, service string) (stale, remark, drifted []string) {
	plan, err := stanzaPlan(root, service)
	if err != nil {
		return nil, nil, nil
	}
	for _, it := range plan.Items {
		switch it.Verdict {
		case wiring.VerdictApply:
			switch {
			case !it.Present:
				// no region yet — init's job, not reconcile's
			case it.Before == it.After:
				remark = append(remark, it.File)
			default:
				stale = append(stale, it.File)
			}
		case wiring.VerdictBlocked:
			drifted = append(drifted, it.File+" — "+it.Reason)
		}
	}
	sort.Strings(stale)
	sort.Strings(remark)
	sort.Strings(drifted)
	return stale, remark, drifted
}

// repointCommand rewrites an absolute fathom path inside a hook command to
// the bare `fathom` (the shim). Everything else in the command survives.
func repointCommand(cmd string) string {
	return staleFathomPath.ReplaceAllString(cmd, "${1}fathom${3}")
}

// reconcileSpec is the safe subset for one repository as ONE wiring spec:
//
//   - stanza regions that are present and stale, up to date but awaiting
//     the digest re-mark, or blocked (so --apply reports the law-15 stop
//     from the same run that applies the rest); a file without markers is
//     never created here;
//   - hook entries absent at both scopes, added with the shim spelling
//     (`fathom <sub>`) at project scope;
//   - hook entries present at project scope whose command is fathom's own
//     spelling once its absolute path is repointed — adopted (they predate
//     the ownership ledger) and rewritten to the shim, keeping the timeout
//     the file already carries. An entry with any other spelling is not
//     fathom's to rewrite and is returned as a note.
func reconcileSpec(root string, d wiringDiff) (wiring.Spec, []string) {
	svc := stanzaServiceID(root, d.Service)
	spec := wiring.Spec{}
	want := map[string]bool{}
	for _, f := range d.StaleStanzas {
		want[f] = true
	}
	for _, f := range d.RemarkStanzas {
		want[f] = true
	}
	for _, f := range d.DriftedStanzas {
		if i := strings.Index(f, " — "); i > 0 {
			want[f[:i]] = true
		}
	}
	intentOn := intentChannelWired(root)
	for _, name := range stanzaFiles {
		if want[name] {
			spec.Regions = append(spec.Regions, stanzaRegion(name, svc, intentOn))
		}
	}

	keys, notes := shimHookKeys(root)
	spec.Keys = keys
	// RP-4: the shipped skills ride the same plan → apply → receipt, so a
	// hand-authored file by a skill's name is BLOCKED by name and a rewrite
	// is undoable with everything else in the receipt.
	spec.Files = skillOwnedFiles(root)
	return spec, notes
}

// shimHookKeys is the hook half of the safe subset (see reconcileSpec): the
// keys to add at project scope with the shim spelling, and the present
// fathom-spelled entries to adopt and repoint. Notes name what was left
// alone and why; they are never silent.
func shimHookKeys(root string) ([]wiring.Key, []string) {
	var keys []wiring.Key
	var notes []string
	userWired, userReadable := userScopeHookKeys()
	present := projectHookEntries(root)
	for _, hs := range fathomHookSpecs {
		template := "fathom " + hs.sub
		if entry, ok := present[hookSpecKeyOf(hs)]; ok {
			if repointCommand(entry.command) != template {
				notes = append(notes, fmt.Sprintf("  note:    %s[%s] spells %q — not fathom's template spelling, left alone", hs.event, hs.matcher, entry.command))
				continue
			}
			k := hookKeyFor(hs, template)
			if entry.timeout > 0 {
				k.Timeout = entry.timeout
			}
			k.AdoptUntracked = true
			keys = append(keys, k)
			continue
		}
		if userReadable && userWired[hookSpecKeyOf(hs)] {
			continue // served by user scope — one scope, never both
		}
		keys = append(keys, hookKeyFor(hs, template))
	}
	return keys, notes
}

func cmdFleetReconcile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet reconcile", stderr)
	apply := fs.Bool("apply", false, "apply the safe subset (shim repoint + additive template merge); dry-run otherwise")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (repo discovery)")
	var repos repeatedFlag
	fs.Var(&repos, "repo", "repo root to reconcile (repeatable; default: every daemon service with a local path)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	roots := []string(repos)
	// svcByRoot carries the REGISTRY's service id per root. Rendering the
	// stanza with an inferred dirname would rewrite a repo that chose a
	// different id with the WRONG name — the registry knows, so ask it.
	svcByRoot := map[string]string{}
	if len(roots) == 0 {
		hc := &http.Client{Timeout: 4 * time.Second}
		svcs, err := fetchServices(hc, *url)
		if err != nil {
			fmt.Fprintf(stderr, "error: no --repo given and service discovery failed: %v\n", err)
			return 1
		}
		for name, info := range svcs {
			if info.path != "" {
				if _, serr := os.Stat(info.path); serr == nil {
					roots = append(roots, info.path)
					svcByRoot[info.path] = strings.ToLower(name)
				}
			}
		}
		sort.Strings(roots)
	}
	if len(roots) == 0 {
		fmt.Fprintln(stderr, "error: nothing to reconcile — no local repo roots found")
		return 1
	}

	if msg, err := installShim(); err != nil {
		fmt.Fprintf(stdout, "shim: %v\n", err)
	} else {
		fmt.Fprintf(stdout, "%s\n", msg)
	}

	drifted := 0
	blockedFiles := map[string]bool{}
	for _, root := range roots {
		d := analyzeWiringFor(root, svcByRoot[root])
		hasDrift := len(d.Missing) > 0 || len(d.StaleCmds) > 0 || len(d.MissingSkills) > 0 || len(d.StaleSkills) > 0 || len(d.BlockedSkills) > 0 || d.StanzaNote != "" || d.SettingsNote != "" || len(d.StaleStanzas) > 0 || len(d.DriftedStanzas) > 0
		if !hasDrift && len(d.RemarkStanzas) == 0 {
			fmt.Fprintf(stdout, "%s: wiring current\n", root)
			continue
		}
		if hasDrift {
			drifted++
			fmt.Fprintf(stdout, "%s:\n", root)
		} else {
			// Up to date in every signal; only a pre-v3 marker awaits its
			// digest. Not drift — but still listed, so --apply's re-mark
			// is announced before it happens.
			fmt.Fprintf(stdout, "%s: wiring current\n", root)
		}
		for _, m := range d.Missing {
			fmt.Fprintf(stdout, "  missing: %s\n", m)
		}
		for _, s := range d.StaleCmds {
			fmt.Fprintf(stdout, "  stale:   %s\n", s)
		}
		if len(d.MissingSkills) > 0 {
			fmt.Fprintf(stdout, "  missing: %d shipped skill file(s) (%s; Unknown-skill failures in sessions)\n", len(d.MissingSkills), skillDirsSummary(d.MissingSkills))
		}
		if len(d.StaleSkills) > 0 {
			fmt.Fprintf(stdout, "  stale:   %d shipped skill file(s) behind the shipped source (%s; rewritten under their marker on --apply)\n", len(d.StaleSkills), skillDirsSummary(d.StaleSkills))
		}
		if d.StanzaNote != "" {
			fmt.Fprintf(stdout, "  stanza:  %s\n", d.StanzaNote)
		}
		if len(d.StaleStanzas) > 0 {
			fmt.Fprintf(stdout, "  stanza:  %s behind the current template (the owned region refreshes on --apply)\n", strings.Join(d.StaleStanzas, ", "))
		}
		if len(d.RemarkStanzas) > 0 {
			fmt.Fprintf(stdout, "  stanza:  %s up to date (will be re-marked with a digest on apply; no prose changes)\n", strings.Join(d.RemarkStanzas, ", "))
		}
		if !*apply {
			for _, f := range d.DriftedStanzas {
				fmt.Fprintf(stdout, "  BLOCKED: %s (fathom will not merge; --apply leaves this file untouched)\n", f)
			}
			for _, f := range d.BlockedSkills {
				fmt.Fprintf(stdout, "  BLOCKED: %s (--apply leaves this file untouched)\n", f)
			}
		}
		if d.SettingsNote != "" {
			fmt.Fprintf(stdout, "  note:    %s\n", d.SettingsNote)
		}

		if !*apply {
			continue
		}
		// Safe subset as ONE plan → apply (law 15): the stanza regions
		// fathom owns and the hook entries, in one receipt per repository.
		// A BLOCKED file is named and left byte-identical; the rest applies.
		spec, notes := reconcileSpec(root, d)
		for _, n := range notes {
			fmt.Fprintln(stdout, n)
		}
		if len(spec.Regions) > 0 || len(spec.Keys) > 0 {
			res, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{Now: time.Now})
			if err != nil {
				fmt.Fprintf(stdout, "  apply failed: %v\n", err)
			}
			// One line per file and per distinct reason — not per item: a
			// settings.json that will not parse blocks every hook key with
			// the same words.
			for _, bf := range blockedByFile(res.Blocked) {
				blockedFiles[root+"\x00"+bf.File] = true
				fmt.Fprintf(stdout, "  BLOCKED: %s (%s) — %s\n", bf.File, blockedCauseLabel(bf.Cause), strings.Join(bf.Reasons, "; "))
			}
			hooksApplied, skillsApplied := 0, 0
			for _, it := range res.Applied {
				switch {
				case it.Kind == wiring.KindFile:
					skillsApplied++
				case it.Kind != wiring.KindRegion:
					hooksApplied++
				case it.Present && it.Before == it.After:
					fmt.Fprintf(stdout, "  applied: %s stanza re-marked with a digest (prose unchanged)\n", it.File)
				default:
					fmt.Fprintf(stdout, "  applied: %s stanza refreshed to the current template\n", it.File)
				}
			}
			if hooksApplied == 1 {
				fmt.Fprintln(stdout, "  applied: 1 hook entry written to .claude/settings.json (shim spelling; project scope, deferring to user scope)")
			} else if hooksApplied > 1 {
				fmt.Fprintf(stdout, "  applied: %d hook entries written to .claude/settings.json (shim spelling; project scope, deferring to user scope)\n", hooksApplied)
			}
			if skillsApplied > 0 {
				fmt.Fprintf(stdout, "  applied: %d shipped skill file(s) rendered under their marker (.claude/skills, .agents/skills)\n", skillsApplied)
			}
			if res.Receipt != nil {
				fmt.Fprintf(stdout, "  receipt: %s (fathom fleet undo %s --repo %s reverses it)\n", res.Receipt.ID, res.Receipt.ID, root)
			}
		}
	}
	mode := "dry-run — pass --apply for the safe subset"
	if *apply {
		mode = "applied the safe subset"
	}
	// CC-10 (docs/plans/custodian.md): concern coverage is INFORMATIONAL,
	// never drift — a repo with no config contracts is normal; reconcile
	// just refuses to let the absence be invisible.
	uncovered := 0
	for _, root := range roots {
		if _, err := os.Stat(filepath.Join(root, ".fathom", "concerns.cue")); err != nil {
			uncovered++
		}
	}
	if uncovered > 0 {
		fmt.Fprintf(stdout, "concern coverage: %d repo(s) carry no .fathom/concerns.cue (fathom init scaffolds a commented template; not drift)\n", uncovered)
	}
	fmt.Fprintf(stdout, "reconcile: %d repo(s), %d drifted (%s)\n", len(roots), drifted, mode)
	if *apply && len(blockedFiles) > 0 {
		fmt.Fprintf(stdout, "reconcile: %s BLOCKED — a drifted (hand-edited) region or a file fathom could not read/parse, named above; nothing was written to them (law 15)\n", plural(len(blockedFiles), "file"))
		return 1
	}
	return 0
}

// repeatedFlag collects a repeatable string flag.
type repeatedFlag []string

func (r *repeatedFlag) String() string     { return strings.Join(*r, ",") }
func (r *repeatedFlag) Set(v string) error { *r = append(*r, v); return nil }
