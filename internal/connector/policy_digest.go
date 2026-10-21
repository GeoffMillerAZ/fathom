package connector

// A discovery-policy receipt is available for LocalConnector. Other
// connectors do not claim a complete policy identity.

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// policyPart frames data unambiguously. The digest is a receipt of effective
// discovery inputs, not a claim that indexed bytes still match the worktree.
func policyPart(h hash.Hash, label string, data []byte) {
	for _, b := range [][]byte{[]byte(label), data} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(b)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(b)
	}
}

func policyFile(h hash.Hash, label, path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		policyPart(h, label, []byte("absent"))
		return nil
	}
	if err != nil {
		return err
	}
	policyPart(h, label+":present", b)
	return nil
}

func gitPolicyFiles(ctx context.Context, h hash.Hash, dir string) error {
	config, err := gitro.Output(ctx, dir, "config", "--list", "--show-origin", "-z")
	if err != nil {
		return fmt.Errorf("git config: %w", err)
	}
	policyPart(h, "git-config:"+dir, config)
	info, err := gitro.Output(ctx, dir, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return fmt.Errorf("git info exclude path: %w", err)
	}
	infoPath := strings.TrimSpace(string(info))
	if !filepath.IsAbs(infoPath) {
		infoPath = filepath.Join(dir, infoPath)
	}
	if err := policyFile(h, "git-info-exclude:"+dir, infoPath); err != nil {
		return err
	}
	excludes, err := gitro.Output(ctx, dir, "config", "--path", "--get", "core.excludesFile")
	if err == nil {
		path := strings.TrimSpace(string(excludes))
		if path != "" {
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			if err := policyFile(h, "git-global-exclude:"+path, path); err != nil {
				return err
			}
		}
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("git excludes config: %w", err)
		}
		// Git's default global exclude when core.excludesFile is unset.
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return homeErr
			}
			base = filepath.Join(home, ".config")
		}
		path := filepath.Join(base, "git", "ignore")
		if err := policyFile(h, "git-default-exclude:"+path, path); err != nil {
			return err
		}
	}
	return nil
}

// DiscoveryPolicyDigest uses the same scoped ignore load, connector config,
// and Git mode that Discover used. In Git mode it binds enclosing/nested
// .gitignore files, per-repository info/exclude, effective Git configuration,
// and the global excludes file. A bounded metadata walk finds nested policy
// files even when an ignore change leaves discovered membership unchanged.
func (c *LocalConnector) DiscoveryPolicyDigest(ctx context.Context, svc core.ServiceRef) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	h := sha256.New()
	policyPart(h, "version", []byte("local-discovery-policy-v2"))
	policyPart(h, "root", []byte(svc.Path))
	_, patterns, _, err := LoadIgnoreScopedForWithPatterns(svc.Path, svc.ID)
	if err != nil {
		return "", err
	}
	for _, p := range patterns {
		policyPart(h, "scoped-ignore", []byte(p))
	}
	if err := policyFile(h, "connector-config", filepath.Join(svc.Path, connectorConfigName)); err != nil {
		return "", err
	}
	include, configuredGit := loadConnectorConfig(svc.Path)
	gitSelected := configuredGit || c.RespectGitignore
	ex := c.Exclusions(svc.ID)
	if ex.Git {
		policyPart(h, "mode", []byte("git"))
	} else {
		policyPart(h, "mode", []byte("walk"))
	}
	if gitSelected {
		policyPart(h, "git-opt-in", []byte("true"))
	}
	if !gitSelected || !isGitWorkTree(ctx, svc.Path) {
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	topBytes, err := gitro.Output(ctx, svc.Path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(topBytes)))
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(svc.Path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(top, root)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("corpus root outside Git top")
	}
	if err := gitPolicyFiles(ctx, h, root); err != nil {
		return "", err
	}
	// Parent ignores govern a corpus rooted below the Git top level.
	parent := top
	if err := policyFile(h, "gitignore:"+parent, filepath.Join(parent, ".gitignore")); err != nil {
		return "", err
	}
	if rel != "." {
		for _, segment := range strings.Split(rel, string(filepath.Separator)) {
			parent = filepath.Join(parent, segment)
			if parent == root {
				break
			}
			if err := policyFile(h, "gitignore:"+parent, filepath.Join(parent, ".gitignore")); err != nil {
				return "", err
			}
		}
	}
	ignore, _, err := LoadIgnoreScopedFor(svc.Path, svc.ID)
	if err != nil {
		return "", err
	}
	entries := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > 200000 {
			return fmt.Errorf("discovery policy metadata bound exceeded")
		}
		if entries%256 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() && path != root {
			if d.Name() == ".git" {
				if filepath.Dir(path) != root {
					if err := gitPolicyFiles(ctx, h, filepath.Dir(path)); err != nil {
						return err
					}
				}
				return filepath.SkipDir
			}
			if skipDir(d.Name()) || defaultIgnoredDir(d.Name(), rel, include) || (ignore != nil && ignore.Match(rel+"/")) {
				return filepath.SkipDir
			}
		}
		if d.Name() == ".gitignore" && !d.IsDir() {
			return policyFile(h, "gitignore:"+path, path)
		}
		if d.Name() == ".git" && path != root && !d.IsDir() && filepath.Dir(path) != root {
			return gitPolicyFiles(ctx, h, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
