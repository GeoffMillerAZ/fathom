package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type hostOwnership struct {
	Version             int                        `json:"version"`
	Host                string                     `json:"host"`
	Root                string                     `json:"root"`
	Binary              string                     `json:"binary"`
	BinaryHash          string                     `json:"binary_hash"`
	PolicyHash          string                     `json:"policy_hash"`
	ConfigBeforeHash    string                     `json:"config_before_hash"`
	ConfigExisted       bool                       `json:"config_existed"`
	InstructionsExisted bool                       `json:"instructions_existed"`
	Stanza              string                     `json:"stanza"`
	Entries             map[string]json.RawMessage `json:"entries"`
}

func hostPaths(host string) (config, instructions, policy, ownership string) {
	config = ".claude/settings.json"
	instructions = "CLAUDE.md"
	if host == "codex" {
		config = ".codex/hooks.json"
		instructions = "AGENTS.md"
	}
	base := ".agentic-kit/hosts/" + host + "/"
	return config, instructions, base + "policy.json", base + "ownership.json"
}

// hostRead rejects symlink components and special files as well as escapes.
// os.Root additionally confines any concurrent resolution to the project.
func hostRead(root *os.Root, path string) ([]byte, bool, error) {
	exists, err := hostRegularPath(root, path)
	if err != nil || !exists {
		return nil, exists, err
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, hostMaxBytes+1))
	if len(raw) > hostMaxBytes {
		return nil, false, fmt.Errorf("%s exceeds 1 MiB", path)
	}
	return raw, true, err
}

func hostRegularPath(root *os.Root, path string) (bool, error) {
	if !hostRelative(path) {
		return false, errors.New("invalid managed path")
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("symlink at %s", path)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return false, fmt.Errorf("not a directory: %s", path)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return false, fmt.Errorf("not a regular file: %s", path)
		}
	}
	return true, nil
}

func hostConfig(raw []byte) (map[string]json.RawMessage, map[string][]json.RawMessage, error) {
	if len(raw) > 0 {
		if err := hostValidateJSON(raw); err != nil {
			return nil, nil, err
		}
	}
	doc := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
			return nil, nil, errors.New("native hook config must be a JSON object")
		}
	}
	hooks := map[string][]json.RawMessage{}
	if value, ok := doc["hooks"]; ok {
		if err := json.Unmarshal(value, &hooks); err != nil || hooks == nil {
			return nil, nil, errors.New("native hooks must be an event object")
		}
	}
	return doc, hooks, nil
}

func hostQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func hostStanza(host string, p hostPolicy) string {
	return "\n\n<!-- agentic-kit:host:" + host + ":begin -->\n" +
		"## Agentic-kit host policy\n\n" +
		"Project policy: `.agentic-kit/hosts/" + host + "/policy.json`. Run `kit-new host doctor -host " + host + " -dir .` before relying on its receipt.\n" +
		"Native hooks cover only the tested entrypoints; instructions are advisory. Hooks are not a sandbox. Unknown or missing required controls cannot certify this workflow.\n" +
		p.Instructions + "\n<!-- agentic-kit:host:" + host + ":end -->\n"
}

func planHostInstall(root *os.Root, rootPath, host, binary string, policy hostPolicy) (hostPlan, error) {
	configPath, instructionPath, policyPath, ownershipPath := hostPaths(host)
	if err := validateHostPolicy(policy); err != nil {
		return hostPlan{}, err
	}
	info, err := os.Stat(binary)
	if err != nil || info == nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return hostPlan{}, errors.New("-binary must name an existing regular executable")
	}
	binaryBytes, err := hostReadExternal(binary, hostMaxBinaryBytes)
	if err != nil {
		return hostPlan{}, err
	}
	policyBytes := hostJSON(policy)
	ownershipRaw, exists, err := hostRead(root, ownershipPath)
	if err != nil {
		return hostPlan{}, err
	}
	if exists {
		var own hostOwnership
		if err := hostDecode(ownershipRaw, &own); err != nil {
			return hostPlan{}, err
		}
		if err := validateHostOwnership(own, host, rootPath); err != nil {
			return hostPlan{}, err
		}
		if own.PolicyHash != hostHash(policyBytes) || own.Binary != binary || own.BinaryHash != hostHash(binaryBytes) {
			return hostPlan{}, errors.New("installed policy/binary differs; uninstall reviewed generated content before replacing")
		}
		if _, _, err := validateInstalledHost(root, rootPath, host); err != nil {
			return hostPlan{}, err
		}
		return sealHostPlan(hostPlan{Version: hostFormatVersion, Host: host, Root: rootPath, Operation: "install", Changes: []hostFileChange{}}), nil
	}
	config, configExists, err := hostRead(root, configPath)
	if err != nil {
		return hostPlan{}, err
	}
	text, textExists, err := hostRead(root, instructionPath)
	if err != nil {
		return hostPlan{}, err
	}
	if _, exists, err := hostRead(root, policyPath); err != nil || exists {
		return hostPlan{}, errors.New("unowned policy path already exists or is inaccessible")
	}
	if bytes.Contains(text, []byte("<!-- agentic-kit:host:"+host+":")) {
		return hostPlan{}, errors.New("unowned instruction marker exists")
	}
	doc, hooks, err := hostConfig(config)
	if err != nil {
		return hostPlan{}, err
	}
	if raw := doc["disableAllHooks"]; bytes.Equal(bytes.TrimSpace(raw), []byte("true")) {
		return hostPlan{}, errors.New("native config disables all hooks; installer does not change this setting")
	}
	entries := map[string]json.RawMessage{}
	command := hostQuote(binary) + " host guard -host " + host + " -dir " + hostQuote(rootPath) + " -policy-digest " + hostHash(policyBytes)
	for _, event := range []string{"SessionStart", "PreToolUse", "PreCompact", "Stop"} {
		entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 5, "statusMessage": "agentic-kit " + host + " " + event}}}
		if event == "SessionStart" {
			entry["matcher"] = "startup|resume|compact"
		}
		if event == "PreToolUse" {
			entry["matcher"] = ".*"
		}
		raw, _ := json.Marshal(entry)
		for _, foreign := range hooks[event] {
			if bytes.Contains(foreign, []byte("agentic-kit "+host+" ")) {
				return hostPlan{}, errors.New("unowned host hook marker exists")
			}
		}
		hooks[event] = append(hooks[event], raw)
		entries[event] = raw
	}
	doc["hooks"], _ = json.Marshal(hooks)
	own := hostOwnership{Version: hostFormatVersion, Host: host, Root: rootPath, Binary: binary, BinaryHash: hostHash(binaryBytes), PolicyHash: hostHash(policyBytes), ConfigBeforeHash: hostHash(config), ConfigExisted: configExists, InstructionsExisted: textExists, Stanza: hostStanza(host, policy), Entries: entries}
	// Ownership goes first, hooks last. An interrupted install has a recovery
	// record before any native hook can reference its policy.
	changes := []hostFileChange{
		{Path: ownershipPath, After: string(hostJSON(own)), AfterExists: true},
		{Path: policyPath, After: string(policyBytes), AfterExists: true},
		{Path: instructionPath, Before: string(text), BeforeExists: textExists, After: string(text) + own.Stanza, AfterExists: true},
		{Path: configPath, Before: string(config), BeforeExists: configExists, After: string(hostJSON(doc)), AfterExists: true},
	}
	return sealHostPlan(hostPlan{Version: hostFormatVersion, Host: host, Root: rootPath, Operation: "install", Changes: changes}), nil
}

func validateHostOwnership(own hostOwnership, host, rootPath string) error {
	if own.Version != hostFormatVersion || own.Host != host || own.Root != rootPath || !filepath.IsAbs(own.Binary) || len(own.PolicyHash) != 64 || len(own.BinaryHash) != 64 || len(own.Entries) != 4 || !strings.Contains(own.Stanza, "<!-- agentic-kit:host:"+host+":begin -->") {
		return errors.New("invalid host ownership record")
	}
	for _, event := range []string{"SessionStart", "PreToolUse", "PreCompact", "Stop"} {
		if len(own.Entries[event]) == 0 {
			return errors.New("missing owned event")
		}
	}
	return nil
}

func planHostUninstall(root *os.Root, rootPath, host string) (hostPlan, error) {
	configPath, instructionPath, policyPath, ownershipPath := hostPaths(host)
	ownershipRaw, exists, err := hostRead(root, ownershipPath)
	if err != nil || !exists {
		return hostPlan{}, errors.New("host ownership record missing or unreadable")
	}
	var own hostOwnership
	if err = hostDecode(ownershipRaw, &own); err != nil {
		return hostPlan{}, err
	}
	if err = validateHostOwnership(own, host, rootPath); err != nil {
		return hostPlan{}, err
	}
	config, configExists, err := hostRead(root, configPath)
	if err != nil {
		return hostPlan{}, err
	}
	doc, hooks, err := hostConfig(config)
	if err != nil {
		return hostPlan{}, err
	}
	changed := false
	if hostHash(config) != own.ConfigBeforeHash {
		for event, entry := range own.Entries {
			keep := []json.RawMessage{}
			found := 0
			for _, candidate := range hooks[event] {
				if hostJSONEqual(candidate, entry) {
					found++
					continue
				}
				keep = append(keep, candidate)
			}
			if found != 1 {
				return hostPlan{}, fmt.Errorf("owned %s hook changed or missing; refusing ambiguous removal", event)
			}
			if len(keep) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = keep
			}
			changed = true
		}
	}
	var changes []hostFileChange
	if changed {
		if len(hooks) == 0 {
			delete(doc, "hooks")
		} else {
			doc["hooks"], _ = json.Marshal(hooks)
		}
		afterExists := own.ConfigExisted || len(doc) > 0
		after := ""
		if afterExists {
			after = string(hostJSON(doc))
		}
		changes = append(changes, hostFileChange{Path: configPath, Before: string(config), BeforeExists: configExists, After: after, AfterExists: afterExists})
	}
	text, textExists, err := hostRead(root, instructionPath)
	if err != nil {
		return hostPlan{}, err
	}
	if textExists {
		count := strings.Count(string(text), own.Stanza)
		if count == 0 && strings.Contains(string(text), "<!-- agentic-kit:host:"+host+":") {
			return hostPlan{}, errors.New("owned instruction stanza changed")
		}
		if count > 1 {
			return hostPlan{}, errors.New("duplicate owned instruction stanza")
		}
		if count == 1 {
			after := strings.Replace(string(text), own.Stanza, "", 1)
			changes = append(changes, hostFileChange{Path: instructionPath, Before: string(text), BeforeExists: true, After: after, AfterExists: own.InstructionsExisted || after != ""})
		}
	}
	policy, policyExists, err := hostRead(root, policyPath)
	if err != nil {
		return hostPlan{}, err
	}
	if policyExists {
		if hostHash(policy) != own.PolicyHash {
			return hostPlan{}, errors.New("owned policy changed; refusing removal")
		}
		changes = append(changes, hostFileChange{Path: policyPath, Before: string(policy), BeforeExists: true})
	}
	changes = append(changes, hostFileChange{Path: ownershipPath, Before: string(ownershipRaw), BeforeExists: true})
	return sealHostPlan(hostPlan{Version: hostFormatVersion, Host: host, Root: rootPath, Operation: "uninstall", Changes: changes}), nil
}

func hostJSONEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return bytes.Equal(hostJSON(x), hostJSON(y))
}
func sealHostPlan(plan hostPlan) hostPlan {
	plan.Digest = ""
	plan.Digest = hostHash(hostJSON(plan))
	return plan
}

func applyHostPlan(root *os.Root, plan hostPlan) error {
	// Compare all baselines before touching anything, then compare again at each
	// write. This detects ordinary concurrent edits; it is not an adversarial lock.
	for _, change := range plan.Changes {
		if err := checkHostBaseline(root, change); err != nil {
			return err
		}
	}
	var applied []hostFileChange
	for _, change := range plan.Changes {
		err := checkHostBaseline(root, change)
		if err == nil {
			err = writeHostChange(root, change)
		}
		if err != nil {
			for i := len(applied) - 1; i >= 0; i-- {
				undo := applied[i]
				undo.Before, undo.After = undo.After, undo.Before
				undo.BeforeExists, undo.AfterExists = undo.AfterExists, undo.BeforeExists
				if checkHostBaseline(root, undo) == nil {
					err = errors.Join(err, writeHostChange(root, undo))
				}
			}
			return err
		}
		applied = append(applied, change)
	}
	return nil
}

func checkHostBaseline(root *os.Root, change hostFileChange) error {
	raw, exists, err := hostRead(root, change.Path)
	if err != nil {
		return err
	}
	if exists != change.BeforeExists || string(raw) != change.Before {
		return fmt.Errorf("conflict: %s changed since plan", change.Path)
	}
	return nil
}

func writeHostChange(root *os.Root, change hostFileChange) error {
	if !change.AfterExists {
		if !change.BeforeExists {
			return nil
		}
		return root.Remove(change.Path)
	}
	if err := root.MkdirAll(filepath.Dir(change.Path), 0700); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info, err := root.Stat(change.Path); err == nil {
		mode = info.Mode().Perm()
	}
	// Deterministic exclusive staging name avoids overwriting an existing file;
	// leftovers after interruption are visible conflicts for manual recovery.
	stage := change.Path + ".agentic-kit-stage"
	f, err := root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer root.Remove(stage)
	_, err = f.WriteString(change.After)
	if err == nil {
		// OpenFile applies umask; restore the promised exact existing mode.
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return root.Rename(stage, change.Path)
}

func hostConfigDigest(root *os.Root, host string) (string, error) {
	config, instructions, policy, ownership := hostPaths(host)
	paths := []string{config, instructions, policy, ownership}
	if host == "claude" {
		paths = append(paths, ".claude/settings.local.json")
	} else {
		paths = append(paths, ".codex/config.toml")
	}
	sort.Strings(paths)
	parts := map[string]string{}
	for _, path := range paths {
		raw, exists, err := hostRead(root, path)
		if err != nil {
			return "", err
		}
		if exists {
			parts[path] = hostHash(raw)
		} else {
			parts[path] = "absent"
		}
	}
	return hostHash(hostJSON(parts)), nil
}
