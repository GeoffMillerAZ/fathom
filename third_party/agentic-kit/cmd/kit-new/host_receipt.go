package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type hostControl struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Basis  string `json:"basis"`
	Detail string `json:"detail"`
}

type hostReceipt struct {
	Version      int           `json:"version"`
	Host         string        `json:"host"`
	HostVersion  string        `json:"host_version"`
	GeneratedAt  string        `json:"generated_at"`
	ConfigDigest string        `json:"config_digest"`
	HelpDigest   string        `json:"help_digest"`
	Controls     []hostControl `json:"controls"`
	Ready        bool          `json:"ready"`
	Scope        string        `json:"scope"`
}

type hostEvidence struct {
	Version      int               `json:"version"`
	Host         string            `json:"host"`
	HostVersion  string            `json:"host_version"`
	ConfigDigest string            `json:"config_digest"`
	Observations []hostObservation `json:"observations"`
}

type hostObservation struct {
	Control string `json:"control"`
	Status  string `json:"status"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Detail  string `json:"detail"`
}

var hostControlNames = []string{"configuration", "runtime", "startup", "resume", "nested-instructions", "tool", "edit", "shell", "stdin", "hosted-tools", "compaction", "stop", "hook-failure-closed"}

func hostControlKnown(name string) bool {
	for _, candidate := range hostControlNames {
		if candidate == name {
			return true
		}
	}
	return false
}

func validateInstalledHost(root *os.Root, rootPath, host string) (hostOwnership, hostPolicy, error) {
	configPath, _, policyPath, ownershipPath := hostPaths(host)
	var own hostOwnership
	var policy hostPolicy
	raw, exists, err := hostRead(root, ownershipPath)
	if err != nil || !exists {
		return own, policy, errors.New("ownership missing/unreadable")
	}
	if err = hostDecode(raw, &own); err != nil {
		return own, policy, err
	}
	if err = validateHostOwnership(own, host, rootPath); err != nil {
		return own, policy, err
	}
	raw, exists, err = hostRead(root, policyPath)
	if err != nil || !exists || hostHash(raw) != own.PolicyHash {
		return own, policy, errors.New("policy missing or drifted")
	}
	if err = hostDecode(raw, &policy); err == nil {
		err = validateHostPolicy(policy)
	}
	if err != nil {
		return own, policy, err
	}
	instructionPath := hostInstructionPath(host, own)
	text, exists, err := hostRead(root, instructionPath)
	if err != nil || !exists || strings.Count(string(text), own.Stanza) != 1 {
		return own, policy, errors.New("instruction stanza missing or drifted")
	}
	if host == "claude" && instructionPath == hostInstructionsFile {
		// A CLAUDE.md that does not import AGENTS.md hides the stanza from Claude
		// Code, whether the adopter wrote it before or after this install.
		claude, claudeExists, err := hostRead(root, hostClaudeFile)
		if err != nil {
			return own, policy, err
		}
		if claudeExists && !hostImportsAgents(claude) {
			return own, policy, errors.New("CLAUDE.md does not import AGENTS.md, so Claude Code will not load the managed stanza: add a line `@AGENTS.md` to it")
		}
	}
	raw, exists, err = hostRead(root, configPath)
	if err != nil || !exists {
		return own, policy, errors.New("native hook config missing")
	}
	doc, hooks, err := hostConfig(raw)
	if err != nil {
		return own, policy, err
	}
	if bytes.Equal(bytes.TrimSpace(doc["disableAllHooks"]), []byte("true")) {
		return own, policy, errors.New("native hooks disabled")
	}
	for event, entry := range own.Entries {
		n := 0
		for _, candidate := range hooks[event] {
			if hostJSONEqual(candidate, entry) {
				n++
			}
		}
		if n != 1 {
			return own, policy, fmt.Errorf("owned %s entry missing or changed", event)
		}
	}
	if host == "claude" {
		raw, exists, err = hostRead(root, ".claude/settings.local.json")
		if err != nil {
			return own, policy, err
		}
		if exists {
			var local map[string]json.RawMessage
			if hostValidateJSON(raw) != nil || json.Unmarshal(raw, &local) != nil || local == nil {
				return own, policy, errors.New("local settings malformed")
			}
			if bytes.Equal(bytes.TrimSpace(local["disableAllHooks"]), []byte("true")) {
				return own, policy, errors.New("local settings disable hooks")
			}
		}
	} else {
		raw, _, err = hostRead(root, ".codex/config.toml")
		if err != nil {
			return own, policy, err
		}
		// Conservative lexical detection; this is deliberately not a TOML parser
		// or a claim to resolve user/system/managed configuration precedence.
		if regexp.MustCompile(`(?m)^\s*(?:hooks|codex_hooks)\s*=\s*false\b`).Match(raw) {
			return own, policy, errors.New("project config contains disabled hooks")
		}
	}
	binary, err := hostReadExternal(own.Binary, hostMaxBinaryBytes)
	if err != nil || hostHash(binary) != own.BinaryHash {
		return own, policy, errors.New("guard binary missing or drifted")
	}
	stat, err := os.Stat(own.Binary)
	if err != nil || stat.Mode().Perm()&0111 == 0 {
		return own, policy, errors.New("guard binary is not executable")
	}
	return own, policy, nil
}

func inspectHost(root *os.Root, rootPath, host, binary, evidencePath string, deps hostDeps) hostReceipt {
	receipt := hostReceipt{Version: hostFormatVersion, Host: host, GeneratedAt: deps.now().UTC().Format("2006-01-02T15:04:05Z"), Scope: "Project-local configuration and declared controls only. Local version/help probes are not live host enforcement tests. Supplied live observations are operator evidence, not re-executed or independently authenticated. Hooks are partial guardrails, not a sandbox."}
	controls := map[string]hostControl{}
	for _, name := range hostControlNames {
		controls[name] = hostControl{Name: name, Status: "unknown", Basis: "unprobed", Detail: "No live observation for this host/version/configuration."}
	}
	controls["hook-failure-closed"] = hostControl{Name: "hook-failure-closed", Status: "unsupported", Basis: "documented-limitation", Detail: "Host hook failures, timeout or disabled hooks can permit work; this adapter cannot certify fail-closed host execution."}
	if host == "codex" {
		controls["stdin"] = hostControl{Name: "stdin", Status: "unsupported", Basis: "documented-limitation", Detail: "write_stdin does not run PreToolUse again for an existing exec session."}
		controls["hosted-tools"] = hostControl{Name: "hosted-tools", Status: "unsupported", Basis: "documented-limitation", Detail: "Hosted tools do not use the local function-tool hook path."}
	}
	_, policy, configErr := validateInstalledHost(root, rootPath, host)
	controls["configuration"] = hostControl{Name: "configuration", Status: "supported", Basis: "local-file-check", Detail: "Owned policy, instructions, hooks and guard executable match."}
	if configErr != nil {
		controls["configuration"] = hostControl{Name: "configuration", Status: "unknown", Basis: "local-file-check", Detail: configErr.Error()}
	}
	digest, err := hostConfigDigest(root, host)
	if err != nil {
		configErr = err
		controls["configuration"] = hostControl{Name: "configuration", Status: "unknown", Basis: "local-file-check", Detail: err.Error()}
	} else {
		receipt.ConfigDigest = digest
	}
	version, versionErr := deps.command(context.Background(), rootPath, binary, "--version")
	help, helpErr := deps.command(context.Background(), rootPath, binary, "--help")
	receipt.HostVersion = strings.TrimSpace(version)
	receipt.HelpDigest = hostHash([]byte(help))
	expected := "2.1.270 (Claude Code)"
	if host == "codex" {
		expected = "codex-cli 0.154.0"
	}
	runtimeOK := versionErr == nil && helpErr == nil && receipt.HostVersion == expected && strings.Contains(help, "Usage:")
	runtimeDetail := "Installed version and help match the tested adapter baseline; event behavior still needs live observation."
	if host == "codex" && runtimeOK {
		features, err := deps.command(context.Background(), rootPath, binary, "features", "list")
		if err != nil || !regexp.MustCompile(`(?m)^hooks\s+stable\s+true\s*$`).MatchString(features) {
			runtimeOK = false
			runtimeDetail = "Installed Codex did not report hooks enabled."
		}
	}
	if !runtimeOK && runtimeDetail == "Installed version and help match the tested adapter baseline; event behavior still needs live observation." {
		runtimeDetail = fmt.Sprintf("Unverified version/help: version=%q, version error=%v, help error=%v", receipt.HostVersion, versionErr, helpErr)
	}
	status := "unknown"
	if runtimeOK {
		status = "supported"
	}
	controls["runtime"] = hostControl{Name: "runtime", Status: status, Basis: "local-version-help", Detail: runtimeDetail}
	if evidencePath != "" && configErr == nil && runtimeOK {
		if err := applyHostEvidence(root, host, receipt.HostVersion, receipt.ConfigDigest, evidencePath, controls); err != nil {
			controls["configuration"] = hostControl{Name: "configuration", Status: "unknown", Basis: "evidence-validation", Detail: err.Error()}
			configErr = err
		}
	}
	receipt.Ready = configErr == nil && runtimeOK
	for _, name := range policy.Required {
		if controls[name].Status != "supported" {
			receipt.Ready = false
		}
	}
	for _, name := range hostControlNames {
		receipt.Controls = append(receipt.Controls, controls[name])
	}
	return receipt
}

func applyHostEvidence(root *os.Root, host, version, digest, path string, controls map[string]hostControl) error {
	raw, exists, err := hostRead(root, path)
	if err != nil || !exists {
		return errors.New("observation file missing or invalid")
	}
	var evidence hostEvidence
	if err := hostDecode(raw, &evidence); err != nil {
		return err
	}
	if evidence.Version != hostFormatVersion || evidence.Host != host || evidence.HostVersion != version || evidence.ConfigDigest != digest {
		return errors.New("observations are stale or for another host/version/configuration")
	}
	if len(evidence.Observations) > len(hostControlNames) {
		return errors.New("too many observations")
	}
	seen := map[string]bool{}
	for _, observation := range evidence.Observations {
		if !hostControlKnown(observation.Control) || observation.Control == "configuration" || observation.Control == "runtime" || seen[observation.Control] {
			return errors.New("invalid or duplicate observed control")
		}
		seen[observation.Control] = true
		if observation.Status != "supported" && observation.Status != "unsupported" && observation.Status != "unknown" {
			return errors.New("invalid observation status")
		}
		if observation.Method != "live-host" || observation.Detail == "" || len(observation.Detail) > 4096 {
			return errors.New("support observations require explicit live-host method and bounded detail")
		}
		artifact, exists, err := hostRead(root, observation.Path)
		if err != nil || !exists || hostHash(artifact) != observation.Digest {
			return errors.New("observation evidence missing or changed")
		}
		if controls[observation.Control].Status == "unsupported" {
			continue
		}
		controls[observation.Control] = hostControl{Name: observation.Control, Status: observation.Status, Basis: "supplied-live-observation", Detail: observation.Detail + " [" + observation.Path + " sha256:" + observation.Digest + "]"}
	}
	return nil
}
