package main

// doctor_claudeload.go — AG-11 (docs/plans/agents-md.md, ADR-059): how
// Claude Code will reach this repository's AGENTS.md. The repository half
// (the root CLAUDE file and its import) is the custodian's
// agents_md_reaches_claude kind; this is the half only doctor reads:
//
//   - a CLAUDE.md, .claude/CLAUDE.md or CLAUDE.local.md in an ANCESTOR
//     directory, which under Claude Code's default agents-md mode switches
//     off every AGENTS.md below it (~/.claude/CLAUDE.md, the user memory
//     file, does not count);
//   - the effective pluginConfigs["agents-md@builtin"].options.instructionFiles
//     setting, managed settings first, then the user's (project settings are
//     ignored by Claude Code for this key, and fathom never writes it: it is
//     user-global, ask-first);
//   - `claude --version` against 2.1.277, the first release that reads
//     AGENTS.md — bounded and cancellable, and "version unknown" when the
//     binary cannot be resolved or answered, never a guess.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// agentsMDMinClaudeVersion is the first Claude Code release that reads
// AGENTS.md natively (agents-md@builtin).
const agentsMDMinClaudeVersion = "2.1.277"

// The instructionFiles modes Claude Code documents.
const (
	claudeModeDefault = "claude-md-or-agents-md"
	claudeModeBoth    = "claude-md-and-agents-md"
	claudeModeClaude  = "claude-md"
	claudeModeManaged = "managed-only"
)

// claudeLoad is what doctor knows about how Claude Code loads AGENTS.md
// here. Every field is a reading, never a guess: an unknown is empty and
// named in VersionNote.
type claudeLoad struct {
	// ClaudeFile is the first present root CLAUDE file ("" when none),
	// Imports whether it imports the root AGENTS.md, ImportOnly whether it
	// carries nothing but fathom's import region.
	ClaudeFile string
	Imports    bool
	ImportOnly bool
	// LegacyStanza names the root CLAUDE files whose fathom region still
	// holds the stanza — the pre-ADR-059 two-copy layout — and Corrupted
	// those whose marker pair is broken (fleet reconcile will not touch it).
	LegacyStanza []string
	Corrupted    []string
	// Ancestors are the CLAUDE files above the repository root.
	Ancestors []string
	// Mode is the effective instructionFiles setting ("" = unset, the
	// default mode) and ModeSource the settings file it came from.
	Mode, ModeSource string
	// Version is `claude --version`'s release ("" when unknown), and
	// VersionNote says why it is unknown.
	Version, VersionNote string
}

// claudeLoadEnv is the outside world claudeLoadPath reads, injectable so a
// test never reads the machine's settings or spawns the real binary.
var claudeLoadEnv = struct {
	home            func() (string, error)
	managedSettings func() []string
	version         func(ctx context.Context) (string, error)
}{
	home:            os.UserHomeDir,
	managedSettings: defaultManagedSettingsPaths,
	version:         probeClaudeVersion,
}

// defaultManagedSettingsPaths is where Claude Code reads managed settings.
func defaultManagedSettingsPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/ClaudeCode/managed-settings.json"}
	case "windows":
		return []string{`C:\Program Files\ClaudeCode\managed-settings.json`}
	default:
		return []string{"/etc/claude-code/managed-settings.json"}
	}
}

var claudeVersionRe = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)\b`)

// errClaudeUnresolved is the version probe's "no binary to ask".
var errClaudeUnresolved = errors.New("claude is not on PATH")

// probeClaudeVersion runs `claude --version` (FATHOM_CLAUDE_BIN overrides
// the PATH lookup, as it does for the compile path) under ctx.
func probeClaudeVersion(ctx context.Context) (string, error) {
	bin := os.Getenv("FATHOM_CLAUDE_BIN")
	if bin == "" {
		p, err := exec.LookPath("claude")
		if err != nil {
			return "", errClaudeUnresolved
		}
		bin = p
	}
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	m := claudeVersionRe.FindString(string(out))
	if m == "" {
		return "", fmt.Errorf("unrecognized output %q", strings.TrimSpace(string(out)))
	}
	return m, nil
}

// versionBefore reports whether release a is before release b (both
// "major.minor.patch").
func versionBefore(a, b string) bool {
	pa, pb := claudeVersionRe.FindStringSubmatch(a), claudeVersionRe.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return false
}

// claudeLoadPath reads the load path for root. The binary is asked for its
// version only when probeVersion is set and nothing imports AGENTS.md: an
// import loads it on every release, so the version cannot change the
// answer. `doctor --repo-only` passes false — it spawns no harness process.
func claudeLoadPath(root string, probeVersion bool) claudeLoad {
	var l claudeLoad
	if f := rootClaudeFile(root); f != "" {
		l.ClaudeFile = f
		if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f))); err == nil {
			content := string(data)
			l.Imports = custodian.ImportsRootAgentsMD(content, f) > 0
			if i := strings.Index(content, fathomStanzaBeginPrefix); i >= 0 {
				rest := content
				if j := strings.Index(content[i:], fathomStanzaEnd); j >= 0 {
					rest = content[:i] + content[i+j+len(fathomStanzaEnd):]
				}
				l.ImportOnly = isImportBody(ownedBodyIn(content)) && strings.TrimSpace(rest) == ""
			}
		}
	}
	for _, f := range claudeFiles() {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		content := string(data)
		begins, ends := strings.Count(content, fathomStanzaBeginPrefix), strings.Count(content, fathomStanzaEnd)
		switch {
		case begins == 0 && ends == 0:
		case begins != 1 || ends != 1 || strings.Index(content, fathomStanzaEnd) < strings.Index(content, fathomStanzaBeginPrefix):
			l.Corrupted = append(l.Corrupted, f)
		case !isImportBody(ownedBodyIn(content)):
			l.LegacyStanza = append(l.LegacyStanza, f)
		}
	}
	home, _ := claudeLoadEnv.home()
	l.Ancestors = ancestorClaudeFiles(root, home)
	l.Mode, l.ModeSource = effectiveInstructionFilesMode(home)

	switch {
	case l.Imports:
	case !probeVersion:
		l.VersionNote = "claude version not probed (--repo-only spawns no harness process)"
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if v, err := claudeLoadEnv.version(ctx); err == nil {
			l.Version = v
		} else {
			l.VersionNote = "claude version unknown: " + err.Error()
		}
	}
	return l
}

// ownedBodyIn is the body of fathom's region in content, "" when none.
func ownedBodyIn(content string) string {
	i := strings.Index(content, fathomStanzaBeginPrefix)
	if i < 0 {
		return ""
	}
	return stanzaBodyOf(content[i:])
}

// ancestorClaudeFiles lists the CLAUDE files in every directory above root,
// nearest first. home's .claude/CLAUDE.md is the user memory file, which
// Claude Code does not count for the default-mode check.
func ancestorClaudeFiles(root, home string) []string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	userMemory := ""
	if home != "" {
		userMemory = filepath.Join(home, ".claude", "CLAUDE.md")
	}
	var out []string
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		for _, f := range claudeFiles() {
			p := filepath.Join(dir, filepath.FromSlash(f))
			if p == userMemory {
				continue
			}
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				out = append(out, p)
			}
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return out
}

// effectiveInstructionFilesMode reads
// pluginConfigs["agents-md@builtin"].options.instructionFiles from the
// managed settings first, then the user's; "" when neither sets it.
func effectiveInstructionFilesMode(home string) (mode, source string) {
	paths := append([]string{}, claudeLoadEnv.managedSettings()...)
	if home != "" {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"))
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc struct {
			PluginConfigs map[string]struct {
				Options struct {
					InstructionFiles string `json:"instructionFiles"`
				} `json:"options"`
			} `json:"pluginConfigs"`
		}
		if json.Unmarshal(data, &doc) != nil {
			continue
		}
		if m := doc.PluginConfigs["agents-md@builtin"].Options.InstructionFiles; m != "" {
			return m, p
		}
	}
	return "", ""
}

// claudeLoadWarnings are the machine-side reasons Claude Code will not load
// AGENTS.md, each with its remedy — only when nothing imports it.
func (l claudeLoad) warnings() (reasons, remedies []string) {
	if l.Imports || l.Mode == claudeModeBoth {
		if l.Mode == claudeModeBoth && len(l.LegacyStanza) > 0 {
			reasons = append(reasons, fmt.Sprintf("mode %s loads both files, so the stanza still in %s loads twice", claudeModeBoth, strings.Join(l.LegacyStanza, ", ")))
		}
		return reasons, remedies
	}
	if len(l.Ancestors) > 0 {
		reasons = append(reasons, fmt.Sprintf("%s above the repository switches AGENTS.md off under Claude Code's default mode", strings.Join(l.Ancestors, ", ")))
		remedies = append(remedies, "move "+l.Ancestors[0]+" out of this repository's ancestry, or add an @AGENTS.md import to a root CLAUDE.md here")
	}
	if l.Mode == claudeModeClaude || l.Mode == claudeModeManaged {
		reasons = append(reasons, fmt.Sprintf("%s sets pluginConfigs[\"agents-md@builtin\"].options.instructionFiles = %q, so Claude Code does not load AGENTS.md", l.ModeSource, l.Mode))
		remedies = append(remedies, fmt.Sprintf("set instructionFiles to %q in %s (fathom never writes it: user-global), or import @AGENTS.md from a root CLAUDE.md", claudeModeDefault, l.ModeSource))
	}
	if l.Version != "" && versionBefore(l.Version, agentsMDMinClaudeVersion) {
		reasons = append(reasons, fmt.Sprintf("claude --version reports %s, before %s, the first release that reads AGENTS.md", l.Version, agentsMDMinClaudeVersion))
		remedies = append(remedies, "upgrade Claude Code to "+agentsMDMinClaudeVersion+" or later, or import @AGENTS.md from a root CLAUDE.md")
	}
	return reasons, remedies
}

// passReason names the path AGENTS.md takes into a Claude Code session.
func (l claudeLoad) passReason() string {
	var r string
	switch {
	case l.Mode == claudeModeBoth:
		r = "mode " + claudeModeBoth
	case l.Imports:
		r = "imported by " + l.ClaudeFile
		if l.ImportOnly {
			r += " (it carries nothing but fathom's import region: deleting it lets Claude Code read AGENTS.md natively)"
		}
	default:
		r = "native (no CLAUDE file, default mode)"
	}
	switch {
	case l.Version != "":
		r += "; Claude Code " + l.Version
	case l.VersionNote != "":
		r += "; " + l.VersionNote
	}
	return r
}
