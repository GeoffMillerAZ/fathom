package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	// lefthook's own dispatcher is the installed pre-push hook (O27).
	notInstalled := ""
	if hook, rel, ok := effectivePushHook(ctx, &r, root); ok && !isLefthookDispatcher(hook) {
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
	probe := func(match func([]string) bool) doctorWiringEvidence {
		reader := doctorWrapperReader{ctx: ctx, root: root, remaining: doctorWrapperBytes, matches: match}
		return reader.command(command, "", 0)
	}
	validate := probe(func(w []string) bool {
		return len(w) > 1 && filepath.Base(w[0]) == "task" && (w[1] == "validate" || w[1] == "validate:unleased")
	})
	verify := probe(func(w []string) bool {
		return len(w) > 2 && (filepath.Base(w[0]) == "fathom" || w[0] == "$FATHOM" || w[0] == "${FATHOM}") && w[1] == "verify" && slices.Contains(w[2:], "--strict")
	})
	verifyAny := probe(func(w []string) bool {
		return len(w) > 1 && (filepath.Base(w[0]) == "fathom" || w[0] == "$FATHOM" || w[0] == "${FATHOM}") && w[1] == "verify"
	})
	if validate.found && verify.found && (validate.indirect || verify.indirect || native) {
		files := dedupeStrings([]string{source, validate.file, verify.file})
		files = slices.DeleteFunc(files, func(s string) bool { return s == "" })
		slices.Sort(files)
		return doctorCheck{Name: "lefthook_gates", Status: doctorPass, Reason: fmt.Sprintf("%spre-push reaches task validate and fathom verify --strict; proved by %s", notInstalled, strings.Join(files, ", "))}, true
	}
	if native && validate.found && !verify.found && verify.opaque != "" {
		// The hook provably reaches task validate; the strict verify step may
		// sit in a script this reader cannot read. Never a guess either way.
		return doctorCheck{Name: "lefthook_gates", Status: doctorUnknown, Reason: fmt.Sprintf("%s%s reaches task validate; the strict fathom verify step is beyond %s, a non-shell script doctor cannot read further, so whether it runs fathom verify --strict is unknown", notInstalled, source, verify.opaque), Remedy: "no change needed if " + verify.opaque + " runs `fathom verify --strict`; doctor never executes hooks or tasks"}, true
	}
	if native && (validate.found || verifyAny.found) {
		return doctorCheck{Name: "lefthook_gates", Status: doctorFail, Reason: notInstalled + "pre-push does not reach both task validate and fathom verify --strict", Remedy: "wire both commands in the repository-local pre-push wrapper"}, true
	}
	return doctorCheck{}, false
}

// effectivePushHook reads the pre-push hook git runs: core.hooksPath's, else
// .git/hooks/pre-push; under husky's generated dispatcher, the authored
// .husky/pre-push it delegates to. ok is false when none exists or it cannot
// be read. Nothing runs.
func effectivePushHook(ctx context.Context, r *doctorWrapperReader, root string) (body []byte, rel string, ok bool) {
	paths := []string{".git/hooks/pre-push"}
	if b, err := doctorGitCommand(ctx, root, "config", "--get", "core.hooksPath").Output(); err == nil {
		if hooksPath := strings.TrimSpace(string(b)); hooksPath != "" {
			paths = []string{filepath.Join(hooksPath, "pre-push")}
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
		b, err := r.read(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		return b, rel, true
	}
	return nil, "", false
}

// isLefthookDispatcher reports whether a hook body is the dispatcher lefthook
// install writes (it defines call_lefthook and honors LEFTHOOK=0).
func isLefthookDispatcher(body []byte) bool {
	t := string(body)
	return strings.Contains(t, "call_lefthook") || strings.Contains(t, "LEFTHOOK_BIN") || strings.Contains(t, "LEFTHOOK_VERBOSE")
}
