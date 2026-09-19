package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const hostFormatVersion = 1
const hostMaxBytes = 1 << 20
const hostMaxBinaryBytes = 128 << 20

// hostDeps keeps time and host processes injectable. Unset now defaults to
// time.Now; command defaults to bounded, context-cancellable execution.
type hostDeps struct {
	now     func() time.Time
	command func(context.Context, string, string, ...string) (string, error)
}

type hostPolicy struct {
	Version      int      `json:"version"`
	DenyTools    []string `json:"deny_tools"`
	DenyPaths    []string `json:"deny_paths"`
	Required     []string `json:"required_controls"`
	Instructions string   `json:"instructions,omitempty"`
}

type hostFileChange struct {
	Path         string `json:"path"`
	Before       string `json:"before"`
	BeforeExists bool   `json:"before_exists"`
	After        string `json:"after"`
	AfterExists  bool   `json:"after_exists"`
}

type hostPlan struct {
	Version   int              `json:"version"`
	Host      string           `json:"host"`
	Root      string           `json:"root"`
	Operation string           `json:"operation"`
	Changes   []hostFileChange `json:"changes"`
	Digest    string           `json:"digest"`
}

func runHost(args []string, stdin io.Reader, stdout, stderr io.Writer, deps hostDeps) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: kit-new host plan|install|uninstall|doctor|probe|guard -host claude|codex -dir PROJECT")
		return 2
	}
	operation := args[0]
	fs := flag.NewFlagSet("kit-new host "+operation, flag.ContinueOnError)
	fs.SetOutput(stderr)
	host := fs.String("host", "", "claude or codex")
	dir := fs.String("dir", ".", "existing project root")
	policyPath := fs.String("policy", "", "bounded policy JSON for plan/install")
	binary := fs.String("binary", "", "absolute installed kit-new executable for plan/install")
	expect := fs.String("expect", "", "reviewed plan digest; rejects stale plans")
	dryRun := fs.Bool("dry-run", false, "print uninstall plan without writing")
	policyDigest := fs.String("policy-digest", "", "expected policy digest for guard")
	hostBinary := fs.String("host-binary", "", "host executable for doctor/probe (default host name)")
	observations := fs.String("observations", "", "independently observed evidence JSON for doctor/probe")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return 2
	}
	if *host != "claude" && *host != "codex" {
		fmt.Fprintln(stderr, "host must be claude or codex")
		return 2
	}
	rootPath, err := filepath.Abs(*dir)
	if err == nil {
		rootPath, err = filepath.EvalSymlinks(rootPath)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return hostFailure(operation, stdout, stderr, err)
	}
	defer root.Close()
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.command == nil {
		deps.command = hostCommand
	}
	if *hostBinary == "" {
		*hostBinary = *host
	}
	switch operation {
	case "guard":
		return hostGuard(root, rootPath, *host, *policyDigest, stdin, stdout, stderr)
	case "plan", "install":
		if *policyPath == "" || !filepath.IsAbs(*binary) {
			fmt.Fprintln(stderr, "plan/install require -policy and an absolute -binary")
			return 2
		}
		var policy hostPolicy
		raw, err := hostReadExternal(*policyPath, hostMaxBytes)
		if err == nil {
			err = hostDecode(raw, &policy)
		}
		if err == nil {
			err = validateHostPolicy(policy)
		}
		if err != nil {
			return hostFailure(operation, stdout, stderr, err)
		}
		plan, err := planHostInstall(root, rootPath, *host, *binary, policy)
		if err != nil {
			return hostFailure(operation, stdout, stderr, err)
		}
		if *expect != "" && *expect != plan.Digest {
			return hostFailure(operation, stdout, stderr, errors.New("reviewed plan is stale"))
		}
		if operation == "install" {
			if err = applyHostPlan(root, plan); err != nil {
				return hostFailure(operation, stdout, stderr, err)
			}
		}
		return hostPrint(stdout, stderr, plan)
	case "uninstall":
		plan, err := planHostUninstall(root, rootPath, *host)
		if err != nil {
			return hostFailure(operation, stdout, stderr, err)
		}
		if *expect != "" && *expect != plan.Digest {
			return hostFailure(operation, stdout, stderr, errors.New("reviewed plan is stale"))
		}
		if !*dryRun {
			if err = applyHostPlan(root, plan); err != nil {
				return hostFailure(operation, stdout, stderr, err)
			}
		}
		return hostPrint(stdout, stderr, plan)
	case "doctor", "probe":
		receipt := inspectHost(root, rootPath, *host, *hostBinary, *observations, deps)
		if hostPrint(stdout, stderr, receipt) != 0 {
			return 1
		}
		if !receipt.Ready {
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "unknown host operation", operation)
		return 2
	}
}

func hostFailure(operation string, stdout, stderr io.Writer, err error) int {
	if operation == "guard" {
		return denyHost(stdout, stderr, "guard unavailable: "+err.Error())
	}
	fmt.Fprintln(stderr, "kit-new host:", err)
	return 1
}

func hostPrint(stdout, stderr io.Writer, value any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func hostDecode(raw []byte, out any) error {
	if err := hostValidateJSON(raw); err != nil {
		return err
	}
	if len(raw) > hostMaxBytes {
		return errors.New("host JSON exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("host JSON must contain one value")
	}
	return nil
}

func hostJSON(value any) []byte {
	raw, _ := json.MarshalIndent(value, "", "  ")
	return append(raw, '\n')
}
func hostHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func hostReadExternal(path string, cap int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New("expected regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, cap+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > cap {
		return nil, errors.New("file exceeds byte cap")
	}
	return raw, nil
}

func validateHostPolicy(p hostPolicy) error {
	if p.Version != hostFormatVersion {
		return errors.New("unsupported host policy version")
	}
	if len(p.Instructions) > 8192 {
		return errors.New("instructions exceed 8192 bytes")
	}
	for label, values := range map[string][]string{"deny_tools": p.DenyTools, "deny_paths": p.DenyPaths, "required_controls": p.Required} {
		if len(values) > 128 {
			return fmt.Errorf("%s exceeds 128 entries", label)
		}
		seen := map[string]bool{}
		for _, v := range values {
			if strings.TrimSpace(v) == "" || len(v) > 512 || seen[v] {
				return fmt.Errorf("invalid or duplicate %s entry", label)
			}
			seen[v] = true
			if label == "deny_paths" && !hostRelative(v) {
				return errors.New("deny_paths must be clean project-relative paths")
			}
			if label == "deny_paths" {
				for _, r := range v {
					if r < 32 || r > 126 {
						return errors.New("deny_paths must use printable ASCII; protect an ASCII-named ancestor for Unicode paths")
					}
				}
			}
			if label == "deny_tools" {
				if _, err := filepath.Match(v, "probe"); err != nil {
					return err
				}
			}
			if label == "required_controls" && !hostControlKnown(v) {
				return fmt.Errorf("unknown required control %q", v)
			}
		}
	}
	return nil
}

func hostRelative(path string) bool {
	return path != "" && path != "." && filepath.IsLocal(path) && filepath.ToSlash(filepath.Clean(path)) == path && !strings.Contains(path, "\\")
}

type hostOutput struct {
	buffer   bytes.Buffer
	overflow bool
}

func (b *hostOutput) String() string { return b.buffer.String() }

func (b *hostOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := 65536 - b.buffer.Len()
	if n > left {
		b.overflow = true
		p = p[:left]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

func hostCommand(ctx context.Context, dir, binary string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	var output hostOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return output.String(), ctx.Err()
	}
	if output.overflow {
		return output.String(), errors.New("host probe output exceeds 64 KiB")
	}
	return output.String(), err
}
