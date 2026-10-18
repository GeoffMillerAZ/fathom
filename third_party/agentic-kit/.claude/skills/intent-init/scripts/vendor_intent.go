// vendor_intent copies a reviewed, bounded Project Intent snapshot into an adopter.
// Concurrency: single-writer command; run from a trusted, quiescent checkout.
// No network, global configuration, deletion of managed files, or Go dependencies.
//
// Layout. The kit keeps its skills in `.claude/skills/intent-*` (the one real copy;
// `.agents/skills` is a relative link to it, for Codex). The snapshot keeps its own stable
// layout, `.agentic-kit/upstream/.agents/skills/intent-*`, because manifest paths are an
// adopter-visible format (schema 1) and renaming them would force every existing adopter
// through a reviewed migration for no behavior gain. A source is read from `.claude/skills`
// first and from `.agents/skills` only when the newer path is not a real directory (an
// older kit checkout); either way the same bytes land at the same snapshot paths.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var skillNames = []string{"audit", "decide", "design", "doctor", "incident", "init", "prune", "release"}

const manifestPath = ".agentic-kit/manifest.json"
const snapshotPath = ".agentic-kit/upstream"

type manifest struct {
	Schema     int               `json:"schema"`
	Source     string            `json:"source"`
	BaseCommit string            `json:"source_base_commit"`
	Kind       string            `json:"kind"`
	Digest     string            `json:"snapshot_sha256"`
	Files      map[string]string `json:"files"`
}

func main() {
	root := flag.String("root", ".", "adopting repository root")
	source := flag.String("source", "", "reviewed local agentic-kit checkout; never fetched")
	check := flag.Bool("check", false, "verify installed bytes and links offline; no writes")
	flag.Parse()
	err := run(*root, *source, *check, flag.NArg())
	if err != nil {
		fmt.Fprintln(os.Stderr, "intent snapshot:", err)
		os.Exit(1)
	}
	fmt.Println("intent snapshot: bytes and project-local skill links verified")
}

func run(root, source string, check bool, extra int) error {
	if extra != 0 || (check && source != "") || (!check && source == "") {
		return fmt.Errorf("use -root <repo> -check OR -root <repo> -source <checkout>")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	gitRoot, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("root must be an existing git repository: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(strings.TrimSpace(string(gitRoot)))
	if err != nil || resolved != root {
		return fmt.Errorf("-root must name the repository root, not a subdirectory")
	}
	if check {
		return verifySnapshot(root)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	if source == root {
		return fmt.Errorf("source and adopter must differ")
	}
	base, err := exec.Command("git", "-C", source, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("source must have a base commit: %w", err)
	}
	return syncSnapshot(root, source, strings.TrimSpace(string(base)))
}

// snapshotSkill is where the snapshot keeps one skill.
func snapshotSkill(name string) string { return ".agents/skills/intent-" + name }

// sourceSkillHomes are where a kit checkout may keep a skill, newest layout first.
var sourceSkillHomes = []string{".claude/skills", ".agents/skills"}

func allowed(path string) bool { return allowedIn(path, []string{".agents/skills"}) }

// allowedIn reports whether path is a snapshot-eligible file whose skill directory
// lives under one of the given skill homes. The manifest and the snapshot use only
// `.agents/skills`; a source checkout may also use `.claude/skills`.
func allowedIn(path string, homes []string) bool {
	if !filepath.IsLocal(path) || filepath.ToSlash(filepath.Clean(path)) != path {
		return false
	}
	if path == "LICENSE" || path == "LICENSE.md" {
		return true
	}
	roots := []string{"docs/project-intent"}
	for _, home := range homes {
		for _, name := range skillNames {
			roots = append(roots, home+"/intent-"+name)
		}
	}
	for _, root := range roots {
		if strings.HasPrefix(path, root+"/") {
			switch filepath.Ext(path) {
			case ".md", ".go", ".sh", ".yaml", ".yml":
				return true
			}
		}
	}
	return false
}

// sourceSkill names the directory that holds skill `name` in a kit checkout: the
// `.claude/skills` copy when it is a real directory, else the legacy `.agents/skills`
// one. A link is never a home (guardPath refuses it), which is what makes a new
// checkout's `.agents/skills` link and an old checkout's `.claude/skills` links both
// fall through to the real copy.
func sourceSkill(source, name string) string {
	for _, home := range sourceSkillHomes {
		rel := home + "/intent-" + name
		if info, err := os.Lstat(filepath.Join(source, filepath.FromSlash(rel))); err == nil && info.IsDir() {
			return rel
		}
	}
	return sourceSkillHomes[len(sourceSkillHomes)-1] + "/intent-" + name
}

// collectSource reads the snapshot files from a kit checkout, mapping each skill from
// wherever the checkout keeps it to the snapshot layout.
func collectSource(source string) (map[string][]byte, error) {
	license := "LICENSE"
	if _, err := os.Lstat(filepath.Join(source, "LICENSE.md")); err == nil {
		license = "LICENSE.md"
	}
	files, err := collect(source, []string{license, "docs/project-intent"}, allowed)
	if err != nil {
		return nil, err
	}
	for _, name := range skillNames {
		home := sourceSkill(source, name)
		got, err := collect(source, []string{home}, func(p string) bool { return allowedIn(p, sourceSkillHomes) })
		if err != nil {
			return nil, err
		}
		for path, b := range got {
			rel := snapshotSkill(name) + strings.TrimPrefix(path, home)
			if !allowed(rel) {
				return nil, fmt.Errorf("outside snapshot allowlist: %s", rel)
			}
			files[rel] = b
		}
	}
	return files, nil
}

// guardPath rejects symlinks in every existing component, including the final file.
func guardPath(root, rel string) error {
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("unsafe path: %s", rel)
	}
	p := root
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in managed path: %s", p)
		}
	}
	return nil
}

func collect(base string, selected []string, ok func(string) bool) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, rel := range selected {
		if err := guardPath(base, rel); err != nil {
			return nil, err
		}
		err := filepath.WalkDir(filepath.Join(base, rel), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("not a regular snapshot file: %s", path)
			}
			name, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			name = filepath.ToSlash(name)
			if !ok(name) {
				return fmt.Errorf("outside snapshot allowlist: %s", name)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[name] = b
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func required(files map[string]string) error {
	paths := []string{"docs/project-intent/README.md"}
	if _, ok := files["LICENSE.md"]; !ok {
		paths = append(paths, "LICENSE")
	}
	for _, name := range skillNames {
		paths = append(paths, snapshotSkill(name)+"/SKILL.md")
	}
	for _, path := range paths {
		if _, ok := files[path]; !ok {
			return fmt.Errorf("missing required file: %s", path)
		}
	}
	return nil
}

func readManifest(root string) (manifest, error) {
	var m manifest
	if err := guardPath(root, manifestPath); err != nil {
		return m, err
	}
	b, err := os.ReadFile(filepath.Join(root, manifestPath))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.Schema != 1 || m.Kind != "working-tree-snapshot" {
		return m, fmt.Errorf("unsupported snapshot manifest")
	}
	if !validCommit(m.BaseCommit) {
		return m, fmt.Errorf("invalid base commit")
	}
	for path, sum := range m.Files {
		if !allowed(path) {
			return m, fmt.Errorf("unsafe manifest path: %s", path)
		}
		if decoded, err := hex.DecodeString(sum); err != nil || len(decoded) != sha256.Size {
			return m, fmt.Errorf("invalid hash: %s", path)
		}
	}
	if err := required(m.Files); err != nil {
		return m, err
	}
	canonical, _ := json.Marshal(m.Files)
	if digest(canonical) != m.Digest {
		return m, fmt.Errorf("snapshot manifest digest mismatch")
	}
	return m, nil
}

func validCommit(s string) bool {
	decoded, err := hex.DecodeString(s)
	return err == nil && (len(decoded) == 20 || len(decoded) == 32)
}

// agentsLinksToClaude reports whether the adopter's `.agents/skills` is the one link
// that makes it read `.claude/skills` (a repo that keeps a single skills copy). Its
// `.claude` links then serve both hosts and the `.agents` set must not be written
// through it. Any other link there is a competing configuration and is refused.
func agentsLinksToClaude(root string) (bool, error) {
	path := filepath.Join(root, ".agents", "skills")
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	got, err := os.Readlink(path)
	if err != nil {
		return false, err
	}
	if got != "../.claude/skills" {
		return false, fmt.Errorf("competing skills link (preserved): %s", path)
	}
	return true, nil
}

func links(root string, mustExist, create bool) error {
	single, err := agentsLinksToClaude(root)
	if err != nil {
		return err
	}
	for _, host := range []string{".agents", ".claude"} {
		if single && host == ".agents" {
			continue
		}
		parent := host + "/skills"
		if err := guardPath(root, parent); err != nil {
			return err
		}
		for _, name := range skillNames {
			path := filepath.Join(root, parent, "intent-"+name)
			want := "../../" + snapshotPath + "/.agents/skills/intent-" + name
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				if mustExist {
					return fmt.Errorf("missing project skill: %s", path)
				}
				if create {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						return err
					}
					if err := os.Symlink(want, path); err != nil {
						return err
					}
				}
				continue
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("competing skill (preserved): %s", path)
			}
			got, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if got != want {
				return fmt.Errorf("competing skill link (preserved): %s", path)
			}
		}
	}
	return nil
}

func verifySnapshot(root string) error {
	m, err := readManifest(root)
	if err != nil {
		return err
	}
	if err := guardPath(root, snapshotPath); err != nil {
		return err
	}
	files, err := collect(filepath.Join(root, snapshotPath), []string{"."}, allowed)
	if err != nil {
		return err
	}
	if len(files) != len(m.Files) {
		return fmt.Errorf("snapshot has missing or unmanaged files")
	}
	for path, sum := range m.Files {
		b, ok := files[path]
		if !ok || digest(b) != sum {
			return fmt.Errorf("modified or missing snapshot file: %s", path)
		}
	}
	return links(root, true, false)
}

func syncSnapshot(root, source, baseCommit string) error {
	if !validCommit(baseCommit) {
		return fmt.Errorf("invalid source base commit")
	}
	files, err := collectSource(source)
	if err != nil {
		return err
	}
	m := manifest{Schema: 1, Source: "github.com/geoffmilleraz/agentic-kit", BaseCommit: baseCommit,
		Kind: "working-tree-snapshot", Files: map[string]string{}}
	for path, b := range files {
		m.Files[path] = digest(b)
	}
	if err := required(m.Files); err != nil {
		return err
	}
	if err := guardPath(root, manifestPath); err != nil {
		return err
	}
	if err := guardPath(root, snapshotPath); err != nil {
		return err
	}
	if err := links(root, false, false); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, manifestPath)); err == nil {
		if err := verifySnapshot(root); err != nil {
			return err
		}
		old, err := readManifest(root)
		if err != nil {
			return err
		}
		for path := range old.Files {
			if _, ok := files[path]; !ok {
				return fmt.Errorf("source removed %s; explicit reviewed migration required, nothing deleted", path)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Lstat(filepath.Join(root, snapshotPath)); !os.IsNotExist(err) {
		return fmt.Errorf("unmanaged snapshot directory exists without a manifest")
	}
	// Preflight has finished. File writes are atomic individually, not as a transaction;
	// interruption leaves an invalid snapshot that check rejects. No partial pass.
	for path, b := range files {
		if err := atomicWrite(filepath.Join(root, snapshotPath, path), b); err != nil {
			return err
		}
	}
	canonical, _ := json.Marshal(m.Files)
	m.Digest = digest(canonical)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(root, manifestPath), append(b, '\n')); err != nil {
		return err
	}
	if err := links(root, false, true); err != nil {
		return err
	}
	return verifySnapshot(root)
}

func atomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".intent-snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
