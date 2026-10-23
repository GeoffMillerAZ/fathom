package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// Inspect only local, bounded scripts; never execute a hook or task. Existing
// literal lefthook entries retain their historical diagnosis.
func checkComposedPushGate(root string) (doctorCheck, bool) {
	ctx, cancel := doctorProbeContext(context.Background())
	defer cancel()
	r := doctorWrapperReader{ctx: ctx, root: root, remaining: doctorWrapperBytes}
	var commands []string
	source := ""
	// The hook git runs decides first: a lefthook.yml only counts when
	// lefthook's own dispatcher is the installed pre-push hook (an adopter's report, 2026-10-22).
	notInstalled := ""
	hook, rel, ok, hookErr := effectivePushHook(ctx, &r, root)
	if hookErr != nil {
		// I38: a hook that exists and cannot be read is never "absent".
		return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: "pre-push hook exists but could not be read: " + rel + " (" + doctorErrCause(hookErr) + ")"}, true
	}
	if ok && !isLefthookDispatcher(hook) {
		for _, p := range []string{"lefthook.yml", "lefthook.yaml"} {
			if _, err := os.Stat(filepath.Join(root, p)); err == nil {
				notInstalled = p + " is not installed as the pre-push hook; "
				break
			}
		}
		if notInstalled != "" {
			commands = append(commands, string(hook))
			source = rel
		}
	}
	for _, p := range []string{"lefthook.yml", "lefthook.yaml"} {
		if source != "" {
			break
		}
		b, err := r.read(filepath.Join(root, p))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return doctorCheck{}, false
		}
		var doc struct {
			Push struct {
				Commands map[string]struct {
					Run string `yaml:"run"`
				} `yaml:"commands"`
				Jobs []struct {
					Run string `yaml:"run"`
				} `yaml:"jobs"`
			} `yaml:"pre-push"`
		}
		if yaml.Unmarshal(b, &doc) != nil {
			return doctorCheck{}, false
		}
		keys := make([]string, 0, len(doc.Push.Commands))
		for k := range doc.Push.Commands {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			commands = append(commands, doc.Push.Commands[k].Run)
		}
		for _, j := range doc.Push.Jobs {
			commands = append(commands, j.Run)
		}
		source = p
		break
	}
	native := source == "" || notInstalled != ""
	if native && notInstalled == "" {
		paths := []string{".git/hooks/pre-push", ".husky/pre-push"}
		if b, err := doctorGitCommand(ctx, root, "config", "--get", "core.hooksPath").Output(); err == nil {
			if hooksPath := strings.TrimSpace(string(b)); hooksPath != "" {
				paths = []string{filepath.Join(hooksPath, "pre-push"), ".husky/pre-push"}
				hooksDir := hooksPath
				if filepath.IsAbs(hooksDir) {
					hooksDir, _ = filepath.Rel(root, hooksDir)
				}
				if filepath.ToSlash(filepath.Clean(hooksDir)) == ".husky/_" {
					// Husky's generated dispatcher delegates to this authored hook.
					paths = []string{".husky/pre-push", filepath.Join(hooksPath, "pre-push")}
				}
			}
		}
		for _, p := range dedupeStrings(paths) {
			path := p
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			b, err := r.read(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: "pre-push hook could not be inspected: " + err.Error()}, true
			}
			commands = append(commands, string(b))
			rel, _ := filepath.Rel(root, path)
			source = rel
			break
		}
		if source == "" {
			return doctorCheck{}, false
		}
	}
	command := strings.Join(commands, "\n")
	verifyCut := false
	probe := func(match func([]string) bool) doctorWiringEvidence {
		reader := doctorWrapperReader{ctx: ctx, root: root, remaining: doctorWrapperBytes, matches: match}
		ev := reader.command(command, "", 0)
		verifyCut = verifyCut || reader.cut
		return ev
	}
	validate := probe(func(w []string) bool {
		return len(w) > 1 && filepath.Base(w[0]) == "task" && (w[1] == "validate" || w[1] == "validate:unleased")
	})
	verifyCut = false // only the strict-verify walk's budget matters below
	verify := probe(func(w []string) bool {
		return len(w) > 2 && (filepath.Base(w[0]) == "fathom" || w[0] == "$FATHOM" || w[0] == "${FATHOM}") && w[1] == "verify" && slices.Contains(w[2:], "--strict")
	})
	verifyCutStrict := verifyCut
	verifyAny := probe(func(w []string) bool {
		return len(w) > 1 && (filepath.Base(w[0]) == "fathom" || w[0] == "$FATHOM" || w[0] == "${FATHOM}") && w[1] == "verify"
	})
	if validate.found && verify.found && (validate.indirect || verify.indirect || native) {
		files := dedupeStrings([]string{source, validate.file, verify.file})
		files = slices.DeleteFunc(files, func(s string) bool { return s == "" })
		slices.Sort(files)
		return doctorCheck{Name: "lefthook_gates", Status: doctorPass, Reason: fmt.Sprintf("%spre-push reaches task validate and fathom verify --strict; proved by %s", notInstalled, strings.Join(files, ", "))}, true
	}
	if native && validate.found && !verify.found && verifyCutStrict {
		return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: fmt.Sprintf("%s%s reaches task validate, but the 64 KiB read budget ran out before the walk found a strict fathom verify step, so whether it runs one is unknown", notInstalled, source), Remedy: "no change needed if the gate runs `fathom verify --strict`; doctor never executes hooks or tasks"}, true
	}
	if native && validate.found && !verify.found && verify.opaque != "" {
		// The hook provably reaches task validate; the strict verify step may
		// sit in a script this reader cannot read. Never a guess either way.
		return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: fmt.Sprintf("%s%s reaches task validate; the strict fathom verify step is beyond %s, a non-shell script doctor cannot read further, so whether it runs fathom verify --strict is unknown", notInstalled, source, verify.opaque), Remedy: "no change needed if " + verify.opaque + " runs `fathom verify --strict`; doctor never executes hooks or tasks"}, true
	}
	if native && (validate.found || verifyAny.found) {
		return doctorCheck{Name: "lefthook_gates", Status: doctorFail, Reason: notInstalled + "pre-push does not reach both task validate and fathom verify --strict", Remedy: "wire both commands in the repository-local pre-push wrapper"}, true
	}
	if notInstalled != "" {
		// git never runs lefthook here, so lefthook.yml proves nothing.
		if validate.unknown || verify.unknown || verifyAny.unknown {
			return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: notInstalled + source + " could not be read far enough to tell whether it reaches task validate and fathom verify --strict"}, true
		}
		return doctorCheck{Name: "lefthook_gates", Status: doctorFail, Reason: notInstalled + source + " reaches neither task validate nor fathom verify --strict", Remedy: "run `lefthook install` so lefthook.yml's pre-push runs, or add the gate to the installed hook"}, true
	}
	return doctorCheck{}, false
}

// effectivePushHook reads the pre-push hook git runs: core.hooksPath's, else
// the (common) git dir's hooks/pre-push; under husky's generated dispatcher,
// the authored .husky/pre-push it delegates to. ok is false when none exists.
// A hook that exists but cannot be read returns err with its label in rel.
// Nothing runs.
func effectivePushHook(ctx context.Context, r *doctorWrapperReader, root string) (body []byte, rel string, ok bool, err error) {
	paths := []string{".git/hooks/pre-push"}
	label := "git-dir hooks/pre-push"
	// git names the hook it runs: core.hooksPath, else the common git dir's
	// hooks (a linked worktree or submodule has a .git file, not a directory).
	if b, e := doctorGitCommand(ctx, root, "rev-parse", "--git-path", "hooks/pre-push").Output(); e == nil {
		if p := strings.TrimSpace(string(b)); p != "" {
			paths = []string{p}
		}
	}
	if b, e := doctorGitCommand(ctx, root, "config", "--get", "core.hooksPath").Output(); e == nil {
		if hooksPath := strings.TrimSpace(string(b)); hooksPath != "" {
			label = "core.hooksPath pre-push"
			hooksDir := hooksPath
			if filepath.IsAbs(hooksDir) {
				hooksDir, _ = filepath.Rel(root, hooksDir)
			}
			if filepath.ToSlash(filepath.Clean(hooksDir)) == ".husky/_" {
				paths = []string{".husky/pre-push", filepath.Join(hooksPath, "pre-push")}
			}
		}
	}
	for _, p := range paths {
		path := p
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		inRel, relErr := filepath.Rel(root, path)
		inside := relErr == nil && !strings.HasPrefix(inRel, "..")
		var data []byte
		var rerr error
		name := label
		if inside {
			name = filepath.ToSlash(inRel)
			data, rerr = r.read(path)
		} else {
			data, rerr = r.readExternal(path)
		}
		switch {
		case rerr == nil:
			return data, name, true, nil
		case errors.Is(rerr, fs.ErrNotExist) || errors.Is(rerr, syscall.ENOTDIR):
			continue
		default:
			return nil, name, false, rerr
		}
	}
	return nil, "", false, nil
}

// isLefthookDispatcher reports whether a hook body is the dispatcher lefthook
// install writes (it defines call_lefthook), or a hand-written hook whose
// command `lefthook run` hands off to it. Comment lines never count.
func isLefthookDispatcher(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i] // a trailing comment is not a command
		}
		if strings.Contains(line, "call_lefthook") || strings.Contains(line, "LEFTHOOK_BIN") || strings.Contains(line, "LEFTHOOK_VERBOSE") {
			return true
		}
		words := strings.Fields(strings.NewReplacer("\"", "", "'", "").Replace(line))
		for i := 0; i+1 < len(words); i++ {
			if (filepath.Base(words[i]) == "lefthook" || words[i] == "$LEFTHOOK" || words[i] == "${LEFTHOOK}") && words[i+1] == "run" {
				return true
			}
		}
	}
	return false
}

// doctorErrCause is err without the path a *PathError carries, so a row never
// names a machine path.
func doctorErrCause(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
