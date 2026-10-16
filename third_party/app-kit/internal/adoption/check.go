package adoption

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

type Finding struct {
	ID     string `json:"id"`
	Status string `json:"status"` // pass, fail, unknown, not-applicable
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

type FileEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type Report struct {
	SchemaVersion   int             `json:"schema_version"`
	Mode            string          `json:"mode"`
	Outcome         string          `json:"outcome"`
	ExitCode        int             `json:"exit_code"`
	CatalogRevision string          `json:"catalog_revision"`
	CheckerBuild    BuildEvidence   `json:"checker_build"`
	SelectionSHA256 string          `json:"selection_sha256,omitempty"`
	Selection       *Selection      `json:"selection,omitempty"`
	Module          *ModuleEvidence `json:"module,omitempty"`
	Files           []FileEvidence  `json:"files"`
	Checks          []Finding       `json:"checks"`
	Limitations     []string        `json:"limitations"`
}

func newReport(mode string) Report {
	return Report{SchemaVersion: 1, Mode: mode, CheckerBuild: CurrentBuild(), Files: []FileEvidence{}, Checks: []Finding{}, Limitations: []string{
		"Static declarations and file bytes only; no runtime integration, browser, performance or host-invocation proof.",
		"Go module declarations are not the effective build graph; replacements are not followed and ambient workspaces are not resolved.",
		"Hashes do not prove origin, licensing, regeneration, tracked-file status or clean sibling-free delivery.",
		"No project commands, downloads, writes, upgrades, host registration or telemetry occur.",
	}}
}

func (r *Report) add(id, status, path, detail, remedy string) {
	r.Checks = append(r.Checks, Finding{id, status, path, detail, remedy})
}

func (r *Report) finish() Report {
	r.Outcome, r.ExitCode = "passed", 0
	if r.Mode == "inspect" || r.Mode == "pins" {
		r.Outcome = "observed"
	}
	for _, check := range r.Checks {
		if check.Status == "fail" && r.ExitCode == 0 {
			r.Outcome, r.ExitCode = "failed", 1
		}
		if check.Status == "unknown" {
			r.Outcome, r.ExitCode = "unavailable", 2
		}
	}
	return *r
}

var errUnsafe = errors.New("path is not a regular, symlink-free relative file")
var errLimit = errors.New("read exceeds bounded inspection budget")

type reader struct {
	ctx   context.Context
	root  *os.Root
	total int
}

// Reject symlinks, special nodes and traversal before opening. os.Root also
// prevents a swapped path from escaping the explicitly selected consumer root.
func (r *reader) read(p string, limit int) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if !SafePath(p) {
		return nil, errUnsafe
	}
	parts := strings.Split(p, "/")
	for i := range parts {
		info, err := r.root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, errUnsafe
		}
		if i == len(parts)-1 && (info.Size() > int64(limit) || info.Size() > int64(MaxTotalBytes-r.total)) {
			return nil, errLimit
		}
	}
	f, err := openAdoptionFile(r.root, p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errUnsafe
	}
	limit = min(limit, MaxTotalBytes-r.total)
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	r.total += len(body)
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, errLimit
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

func (r *Report) file(rd *reader, id, p string, limit int, required bool) ([]byte, bool) {
	return r.fileAbsent(rd, id, p, limit, required, "Optional inventory file is absent; no selection inferred.")
}

// fileAbsent is file with the not-applicable explanation for an optional file
// that does not exist, so a finding says why absence is acceptable.
func (r *Report) fileAbsent(rd *reader, id, p string, limit int, required bool, absent string) ([]byte, bool) {
	body, err := rd.read(p, limit)
	if err == nil {
		r.Files = append(r.Files, FileEvidence{p, hash(body), len(body)})
		return body, true
	}
	status, detail, remedy := "unknown", "File could not be read within inspection limits.", "Check access, file size and cancellation; retry the same explicit root."
	if errors.Is(err, fs.ErrNotExist) {
		status, detail, remedy = "fail", "Required file is missing.", "Restore the selected file or review the adoption contract."
		if !required {
			status, detail, remedy = "not-applicable", absent, ""
		}
	} else if errors.Is(err, errUnsafe) {
		status, detail, remedy = "fail", "Path is not a regular, symlink-free file inside the selected root.", "Use an explicit consumer-owned regular file; do not follow external paths."
	}
	r.add(id, status, p, detail, remedy)
	return nil, false
}

func open(ctx context.Context, root, mode string) (Report, *reader, *catalog.Catalog) {
	r := newReport(mode)
	c, err := catalog.Default()
	if err != nil {
		r.add("catalog", "unknown", "", "Embedded catalog could not be loaded.", "Rebuild a validated appkit binary.")
		return r, nil, nil
	}
	r.CatalogRevision = c.Revision
	handle, err := os.OpenRoot(root)
	if err != nil {
		r.add("root", "unknown", "", "Explicit consumer root could not be opened.", "Choose an accessible consumer directory.")
		return r, nil, c
	}
	return r, &reader{ctx: ctx, root: handle}, c
}

func (r *Report) selection(rd *reader, manifest string, required bool) *Selection {
	body, ok := r.file(rd, "selection", manifest, MaxManifestBytes, required)
	if !ok {
		return nil
	}
	s, err := Decode(body)
	if err != nil {
		// Keep the v1 invalid-input diagnostic stable. Version2 is mentioned only
		// when explicitly selected; unknown versions still fail closed.
		detail := "Selection is not valid version-1 adoption JSON."
		var version struct {
			SchemaVersion int `json:"schema_version"`
		}
		if json.Unmarshal(body, &version) == nil && version.SchemaVersion == 2 {
			detail = "Selection is not valid version-2 adoption JSON."
		}
		r.add("selection", "fail", manifest, detail, "Use the published CUE schema; remove duplicate, unknown or unsafe fields.")
		return nil
	}
	r.Selection = &s
	canonical, _ := json.Marshal(s)
	r.SelectionSHA256 = hash(canonical)
	r.add("selection", "pass", manifest, "Explicit selection decoded; no capabilities inferred.", "")
	return &s
}

// Inspect inventories known files and an optional explicit selection. It does
// not hash declared artifacts or check adoption fitness; use Check for that.
func Inspect(ctx context.Context, root, manifest string) Report {
	r, rd, _ := open(ctx, root, "inspect")
	if rd == nil {
		return r.finish()
	}
	defer rd.root.Close()
	s := r.selection(rd, manifest, false)
	moduleFile := "go.mod"
	if s != nil && s.Module != nil {
		moduleFile = s.Module.File
	}
	if body, ok := r.file(rd, "module", moduleFile, 1<<20, false); ok {
		observed, err := ObserveModule(body)
		if err != nil {
			r.add("module", "fail", moduleFile, "Module declarations are malformed or ambiguous.", "Review go.mod; no Go command was executed.")
		} else {
			r.Module = &observed
			r.add("module", "pass", moduleFile, "Parsed declarations only; effective revision remains unverified.", "")
		}
	}
	for _, p := range []string{"go.work", "CLAUDE.md", "AGENTS.md"} {
		if _, ok := r.file(rd, "inventory:"+p, p, 1<<20, false); ok {
			r.add("inventory:"+p, "pass", p, "File bytes observed; contents are not executed or treated as adoption proof.", "")
		}
	}
	return r.finish()
}

// Check verifies only the selected static contract. Passing does not certify
// clean delivery, host invocation, runtime behavior or unselected capabilities.
func Check(ctx context.Context, root, manifest string) Report {
	r, rd, c := open(ctx, root, "check")
	if rd == nil {
		return r.finish()
	}
	defer rd.root.Close()
	s := r.selection(rd, manifest, true)
	if s == nil {
		return r.finish()
	}
	if s.SchemaVersion == 2 {
		r.checkCapabilityRevisions(c, *s.CapabilityRevisions)
	} else if s.CatalogRevision != c.Revision {
		r.add("catalog", "fail", "", "Selected catalog revision differs from this checker.", "Review the catalog delta with a deliberately pinned checker before updating the selection.")
	} else {
		r.add("catalog", "pass", "", "Selected catalog revision matches this checker.", "")
	}
	r.checkSelectionFit(c, s)
	if body, ok := r.file(rd, "contract", s.Contract, 1<<20, true); ok {
		if strings.TrimSpace(string(body)) == "" {
			r.add("contract", "fail", s.Contract, "Human adoption contract is empty.", "Record the actual task, design context, constraints and proof workflow.")
		} else {
			r.add("contract", "pass", s.Contract, "Human contract exists; its semantic completeness still requires review.", "")
		}
	}
	for i, artifact := range s.Artifacts {
		id := fmt.Sprintf("artifact:%d", i+1)
		if body, ok := r.file(rd, id, artifact.Path, MaxFileBytes, true); ok {
			if hash(body) != artifact.SHA256 {
				r.add(id, "fail", artifact.Path, "Artifact bytes differ from the authored contract hash.", "Rebuild or review the intended change; never bless unexplained drift.")
			} else {
				r.add(id, "pass", artifact.Path, "Artifact bytes match the authored hash; origin and regeneration are not proven.", "")
			}
		}
	}
	r.checkModule(rd, s.Module)
	if !s.Agents {
		r.add("agent-routing", "not-applicable", "", "Agent routing was not selected; no harness requirement imposed.", "")
	} else {
		block, err := Router(s.Contract)
		if err != nil {
			r.add("agent-routing", "fail", s.Contract, "Contract path cannot be represented in the short router.", "Use a plain relative documentation path.")
		} else {
			r.checkAgentRouting(rd, block)
		}
	}
	return r.finish()
}

const (
	routerPass = "Canonical router text exists; this does not prove the host loaded it."
	routerFail = "Selected router is missing, duplicated or differs from the canonical short contract."
	routerFix  = "Review appkit adoption router output and preserve unrelated consumer instructions."
)

// checkAgentRouting applies ADR-0088. AGENTS.md is the one instruction file and
// must carry the canonical router. CLAUDE.md is optional; when it exists it must
// reach that router, because a consumer that keeps one loads it in place of
// AGENTS.md. Text and link parity is not host execution.
func (r *Report) checkAgentRouting(rd *reader, block string) {
	// CLAUDE.md first, then AGENTS.md: the report order of a consumer that keeps
	// both files is unchanged from ADR-0024.
	r.checkClaudeRouting(rd, block)
	if body, ok := r.file(rd, "router:AGENTS.md", "AGENTS.md", 1<<20, true); ok {
		if carriesRouter(body, block) {
			r.add("router:AGENTS.md", "pass", "AGENTS.md", routerPass, "")
		} else {
			r.add("router:AGENTS.md", "fail", "AGENTS.md", routerFail, routerFix)
		}
	}
}

func (r *Report) checkClaudeRouting(rd *reader, block string) {
	const id, p = "router:CLAUDE.md", "CLAUDE.md"
	if info, err := rd.root.Lstat(p); err == nil && info.Mode()&os.ModeSymlink != 0 {
		// The layout that keeps one file: CLAUDE.md is a link to AGENTS.md. Only
		// that exact target counts; no other link is followed.
		if target, err := rd.root.Readlink(p); err == nil && path.Clean(target) == "AGENTS.md" {
			r.add(id, "pass", p, "CLAUDE.md links to AGENTS.md, the one instruction file; this does not prove the host loaded it.", "")
			return
		}
	}
	body, ok := r.fileAbsent(rd, id, p, 1<<20, false, "No CLAUDE.md; AGENTS.md is the one instruction file.")
	if !ok {
		return
	}
	switch {
	case importsAgentsMD(body):
		r.add(id, "pass", p, "CLAUDE.md imports AGENTS.md, the one instruction file; this does not prove the host loaded it.", "")
	case carriesRouter(body, block):
		r.add(id, "pass", p, routerPass, "")
	default:
		r.add(id, "fail", p, "CLAUDE.md neither imports AGENTS.md nor carries the canonical router, so the router is unreachable through it.", "Add an @AGENTS.md import line to CLAUDE.md, or remove CLAUDE.md; preserve unrelated consumer instructions.")
	}
}

// carriesRouter reports whether the text holds the canonical marked block
// exactly once.
func carriesRouter(body []byte, block string) bool {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	return strings.Count(text, "<!-- appkit:begin -->") == 1 && strings.Count(text, "<!-- appkit:end -->") == 1 && strings.Contains("\n"+text, "\n"+block)
}

var codeSpan = regexp.MustCompile("`[^`]*`")

// importsAgentsMD reports whether CLAUDE.md text pulls AGENTS.md in with a
// Claude Code memory import: a whitespace-delimited @AGENTS.md or @./AGENTS.md
// token outside fenced code blocks and inline code spans, which the host does
// not evaluate. Any other spelling is not counted, so a near miss fails loudly.
func importsAgentsMD(body []byte) bool {
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		for _, token := range strings.Fields(codeSpan.ReplaceAllString(line, " ")) {
			if token == "@AGENTS.md" || token == "@./AGENTS.md" {
				return true
			}
		}
	}
	return false
}

// Keep authoring and checking on one profile/layer/metadata-fit contract.
func (r *Report) checkSelectionFit(c *catalog.Catalog, s *Selection) {
	_, profileErr := c.Search(catalog.Query{Profile: s.Profile})
	if profileErr != nil || s.Profile != "cli" && len(s.Layers) == 0 {
		r.add("profile", "fail", "", "Unknown profile or missing applicable layer selection.", "Select catalog vocabulary appropriate to the consumer; native desktop support is not implied.")
	} else {
		r.add("profile", "pass", "", "Selected profile is recognized; target execution is not proven.", "")
	}
	for i, layer := range s.Layers {
		id := fmt.Sprintf("layer:%d", i+1)
		if _, err := c.Search(catalog.Query{Layer: layer}); err != nil {
			r.add(id, "fail", "", "Selected layer is not in catalog vocabulary.", "Review the explicit selected layers.")
		} else {
			r.add(id, "pass", "", "Selected layer is recognized.", "")
		}
	}
	for i, id := range s.Capabilities {
		entry, ok := c.Get(id)
		fit := ok && slices.Contains(entry.Profiles, s.Profile) && (s.Profile == "cli" && len(entry.Layers) == 0 || slices.ContainsFunc(entry.Layers, func(layer string) bool { return slices.Contains(s.Layers, layer) }))
		key := fmt.Sprintf("capability:%d", i+1)
		if !fit {
			r.add(key, "fail", "", fmt.Sprintf("%q is absent or incompatible with the selected profile/layers.", id), "Choose a fitting exact catalog ID; do not silently change renderers.")
		} else {
			r.add(key, "pass", "", fmt.Sprintf("%q metadata fits; integration and catalog evidence gaps remain separate.", id), "")
		}
	}
}

func (r *Report) checkModule(rd *reader, requirement *ModuleRequirement) {
	if requirement == nil {
		r.add("module", "not-applicable", "", "No Go module requirement selected; artifact/vocabulary adoption remains valid.", "")
		return
	}
	body, ok := r.file(rd, "module", requirement.File, 1<<20, true)
	if !ok {
		return
	}
	observed, err := ObserveModule(body)
	if err != nil {
		r.add("module", "fail", requirement.File, "Module declarations are malformed or ambiguous.", "Review the selected module file.")
		return
	}
	r.Module = &observed
	if observed.DeclaredVersion != requirement.Version {
		r.add("module", "fail", requirement.File, "Declared kit requirement does not match the selected exact version.", "Review the explicit kit pin and adoption contract together.")
		return
	}
	if observed.Replacement != nil {
		if observed.Replacement.Version != "" || !requirement.AllowLocalReplace {
			r.add("module", "fail", requirement.File, "Replacement is not an explicitly allowed local development replacement.", "Remove the replacement for delivery or explicitly review local development use.")
			return
		}
		r.add("module", "pass", requirement.File, "Declared pin matches; local replacement is allowed for development only and its effective revision is unverified.", "")
		return
	}
	r.add("module", "pass", requirement.File, "Declared kit version matches; no module-local replacement. Effective resolution and clean delivery remain unverified.", "")
}
