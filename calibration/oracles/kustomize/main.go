// Command kustomize-oracle produces calibration truth by ablating entries from
// scratch copies and comparing native Kustomize 5.8.1 builds.
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
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/types"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

var errOutputCap = errors.New("build output exceeds 16 MiB")

const (
	maxFiles       = 512
	maxEntries     = 8192
	maxContexts    = 64
	maxOutput      = 16 << 20
	maxSourceFiles = 100000
	maxSourceBytes = 256 << 20
	maxNativeCalls = 65536
	maxWholeRun    = 30 * time.Minute
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}
type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}
type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
}
type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}
type field struct {
	name, key, kind string
	paths           bool
}

var fields = []field{
	{"resources", "", "resource", true}, {"bases", "", "resource", true}, {"components", "", "resource", true},
	{"patchesStrategicMerge", "", "resource", true}, {"patches", "path", "resource", true}, {"patchesJson6902", "path", "resource", true},
	{"images", "name", "data_source", false}, {"configMapGenerator", "name", "resource", false}, {"secretGenerator", "name", "resource", false},
}

type entry struct {
	Field string `json:"-"`
	Index int    `json:"i"`
	Value string `json:"v"`
	Line  int    `json:"l"`
}
type kfile struct {
	rel, base, kind string
	line            int
	entries         []entry
	fieldOf         []field
	typed           types.Kustomization
	raw             []byte
	inventory       map[string]int
}
type receipt struct {
	Version            string                   `json:"version"`
	KustomizationsHash string                   `json:"kustomizations_sha256"`
	BinaryHash         string                   `json:"binary_sha256"`
	Files              int                      `json:"files"`
	Entries            int                      `json:"typed_entries"`
	Built              int                      `json:"built_roots"`
	Excluded           map[string]int           `json:"excluded"`
	Inventory          map[string]int           `json:"inventory"`
	Contexts           map[string]contextCounts `json:"component_contexts"`
	Stats              map[string]int           `json:"stats"`
}
type contextCounts struct {
	Valid     int `json:"valid"`
	Tried     int `json:"tried"`
	Excluded  int `json:"excluded"`
	Effective int `json:"effective_entries"`
	Noop      int `json:"noop_entries"`
}

func withinCap(n, max int) error {
	if n > max {
		return fmt.Errorf("subject cap exceeded: %d > %d", n, max)
	}
	return nil
}
func decode(rel string, src []byte) (kfile, error) {
	k := kfile{rel: rel, base: path.Base(rel), raw: src, inventory: map[string]int{}}
	if err := k.typed.Unmarshal(src); err != nil {
		return k, err
	}
	checked := k.typed
	checked.FixKustomization()
	if err := checked.CheckEmpty(); err != nil {
		return k, err
	}
	if es := checked.EnforceFields(); len(es) > 0 {
		return k, errors.New(strings.Join(es, "; "))
	}
	k.kind = k.typed.Kind
	node, err := kyaml.Parse(string(src))
	if err != nil {
		return k, err
	}
	if node == nil || node.YNode() == nil {
		return k, errors.New("empty YAML node")
	}
	k.line = node.YNode().Line
	count := map[string]int{}
	for _, f := range fields {
		count[f.name] = typedLength(k.typed, f.name)
	}
	for _, f := range fields {
		n := count[f.name]
		if n == 0 {
			continue
		}
		m := node.Field(f.name)
		if m == nil || m.Value == nil {
			return k, fmt.Errorf("%s: typed field has no source node", f.name)
		}
		elems, err := m.Value.Elements()
		if err != nil {
			return k, fmt.Errorf("%s: %w", f.name, err)
		}
		if len(elems) != n {
			return k, fmt.Errorf("%s: typed/source count mismatch %d/%d", f.name, n, len(elems))
		}
		for i, e := range elems {
			value := typedValue(k.typed, f.name, i)
			if value == "" {
				if f.name == "patches" || f.name == "patchesJson6902" {
					k.inventory["inline:"+f.name]++
				} else {
					k.inventory["unnamed:"+f.name]++
				}
				continue
			}
			var source string
			if f.key == "" {
				source = e.YNode().Value
			} else {
				m := e.Field(f.key)
				if m != nil && m.Value != nil {
					source = m.Value.YNode().Value
				}
			}
			if source != value {
				return k, fmt.Errorf("%s[%d]: typed/source value mismatch", f.name, i)
			}
			k.entries = append(k.entries, entry{Field: f.name, Index: i, Value: value, Line: e.YNode().Line})
			k.fieldOf = append(k.fieldOf, f)
		}
		k.inventory["graded:"+f.name] = n
	}
	// Marshal the native typed value, honoring its inline and omitempty rules.
	// The JSON inventory is kept separate from source-position reconciliation.
	typedJSON, err := json.Marshal(k.typed)
	if err != nil {
		return k, err
	}
	var populated map[string]json.RawMessage
	if err := json.Unmarshal(typedJSON, &populated); err != nil {
		return k, err
	}
	for name, raw := range populated {
		if _, graded := count[name]; graded {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return k, err
		}
		n := 1
		switch v := value.(type) {
		case []any:
			n = len(v)
		case map[string]any:
			n = len(v)
		}
		category := "unsupported:"
		if name == "kind" || name == "apiVersion" || name == "metadata" {
			category = "control:"
		}
		k.inventory[category+name] = n
	}
	return k, nil
}
func typedEntryCount(k types.Kustomization) int {
	v := reflect.ValueOf(k)
	n := 0
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Slice {
			n += v.Field(i).Len()
		}
	}
	return n
}
func typedLength(k types.Kustomization, n string) int {
	switch n {
	case "resources":
		return len(k.Resources)
	case "bases":
		return len(k.Bases)
	case "components":
		return len(k.Components)
	case "patchesStrategicMerge":
		return len(k.PatchesStrategicMerge)
	case "patches":
		return len(k.Patches)
	case "patchesJson6902":
		return len(k.PatchesJson6902)
	case "images":
		return len(k.Images)
	case "configMapGenerator":
		return len(k.ConfigMapGenerator)
	case "secretGenerator":
		return len(k.SecretGenerator)
	}
	return 0
}
func typedValue(k types.Kustomization, n string, i int) string {
	switch n {
	case "resources":
		return k.Resources[i]
	case "bases":
		return k.Bases[i]
	case "components":
		return k.Components[i]
	case "patchesStrategicMerge":
		return string(k.PatchesStrategicMerge[i])
	case "patches":
		return k.Patches[i].Path
	case "patchesJson6902":
		return k.PatchesJson6902[i].Path
	case "images":
		return k.Images[i].Name
	case "configMapGenerator":
		return k.ConfigMapGenerator[i].Name
	case "secretGenerator":
		return k.SecretGenerator[i].Name
	}
	return ""
}
func removeEntry(src []byte, e entry) ([]byte, error) {
	n, err := kyaml.Parse(string(src))
	if err != nil {
		return nil, err
	}
	m := n.Field(e.Field)
	if m == nil || m.Value == nil {
		return nil, errors.New("missing field")
	}
	y := m.Value.YNode()
	if e.Index < 0 || e.Index >= len(y.Content) {
		return nil, errors.New("index out of range")
	}
	y.Content = append(y.Content[:e.Index], y.Content[e.Index+1:]...)
	s, err := n.String()
	return []byte(s), err
}
func findKustomizations(root string) ([]string, error) {
	names := map[string]bool{}
	for _, n := range konfig.RecognizedKustomizationFileNames() {
		names[n] = true
	}
	var result []string
	dirs := map[string][]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !names[d.Name()] {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("recognized kustomization is not regular: %s", p)
		}
		rel, _ := filepath.Rel(root, p)
		s := filepath.ToSlash(rel)
		dirs[path.Dir(s)] = append(dirs[path.Dir(s)], s)
		result = append(result, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(result)
	var dirNames []string
	for d := range dirs {
		dirNames = append(dirNames, d)
	}
	sort.Strings(dirNames)
	for _, d := range dirNames {
		if len(dirs[d]) > 1 {
			return nil, fmt.Errorf("multiple kustomization files under %s", d)
		}
	}
	return result, nil
}
func guardScratch(root, scratch string) error {
	a, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	b, err := filepath.Abs(scratch)
	if err != nil {
		return err
	}
	a, err = filepath.EvalSymlinks(a)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(b, 0700); err != nil {
		return err
	}
	b, err = filepath.EvalSymlinks(b)
	if err != nil {
		return err
	}
	if b == a || strings.HasPrefix(b, a+string(filepath.Separator)) || strings.HasPrefix(a, b+string(filepath.Separator)) {
		return errors.New("source and scratch overlap")
	}
	return nil
}
func guardOutput(root, dest string) error {
	if dest == "" {
		return nil
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return err
	}
	resolved := filepath.Join(parent, filepath.Base(abs))
	if actual, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = actual
	} else if !os.IsNotExist(err) {
		return err
	}
	if resolved == root || strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return fmt.Errorf("output overlaps source: %s", dest)
	}
	return nil
}
func preflightSource(root string) (int, int64, error) {
	return preflightSourceWithLimits(root, maxSourceFiles, maxSourceBytes)
}
func preflightSourceWithLimits(root string, fileLimit int, byteLimit int64) (int, int64, error) {
	files := 0
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		files++
		if err := withinCap(files, fileLimit); err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			if total > byteLimit {
				return fmt.Errorf("source bytes cap exceeded: %d > %d", total, byteLimit)
			}
		}
		return nil
	})
	return files, total, err
}
func ownedScratch(root, scratch string) (string, error) {
	if err := guardScratch(root, scratch); err != nil {
		return "", err
	}
	return os.MkdirTemp(scratch, "kustomize-oracle-")
}
func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func nativeRejection(err error) bool {
	var e *exec.ExitError
	return errors.As(err, &e) && e.ExitCode() == 1 && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) && !errors.Is(err, errOutputCap)
}
func plannedNativeCalls(roots []string, files map[string]kfile, contexts map[string][]string) (int, error) {
	return plannedNativeCallsWithLimit(roots, files, contexts, maxNativeCalls)
}
func plannedNativeCallsWithLimit(roots []string, files map[string]kfile, contexts map[string][]string, limit int) (int, error) {
	n := 1 + len(roots)
	if err := withinCap(n, limit); err != nil {
		return n, err
	}
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		k := files[p]
		mult := 0
		if k.kind == "Component" {
			mult = len(contexts[p])
		} else {
			mult = 1
		}
		n += len(k.entries) * mult
		if err := withinCap(n, limit); err != nil {
			return n, err
		}
	}
	return n, nil
}
func copyTree(src, dst string) (int, error) {
	unsupported := 0
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0755)
		}
		if !d.Type().IsRegular() {
			unsupported++
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0644)
	})
	return unsupported, err
}

// nativePathValues covers paths Kustomize can load even when the oracle does not
// grade that field. Empty paths represent inline patches and are skipped.
func nativePathValues(k types.Kustomization) []string {
	values := make([]string, 0)
	values = append(values, k.Resources...)
	values = append(values, k.Bases...)
	values = append(values, k.Components...)
	values = append(values, k.Configurations...)
	values = append(values, k.Transformers...)
	values = append(values, k.Generators...)
	values = append(values, k.Validators...)
	values = append(values, k.Crds...)
	for _, p := range k.PatchesStrategicMerge {
		values = append(values, string(p))
	}
	for _, p := range k.Patches {
		values = append(values, p.Path)
	}
	for _, p := range k.PatchesJson6902 {
		values = append(values, p.Path)
	}
	if p := k.OpenAPI["path"]; p != "" {
		values = append(values, p)
	}
	return values
}
func closureUnsafeReason(root string, graph map[string][]string, files map[string]kfile) string {
	seen := map[string]bool{}
	var walk func(string) string
	walk = func(p string) string {
		if seen[p] {
			return ""
		}
		seen[p] = true
		k, ok := files[p]
		if !ok {
			return ""
		}
		for _, value := range nativePathValues(k.typed) {
			if value == "" {
				continue
			}
			if remoteRE.MatchString(value) {
				return "remote"
			}
			if path.IsAbs(value) || filepath.IsAbs(value) {
				return "absolute"
			}
		}
		for _, next := range graph[p] {
			if reason := walk(next); reason != "" {
				return reason
			}
		}
		return ""
	}
	return walk(root)
}

var remoteRE = regexp.MustCompile(`^(?:[a-z][a-z0-9+.-]*::|[a-z][a-z0-9+.-]*://|git@|github\.com[/:]|gitlab\.com[/:]|bitbucket\.org[/:])`)

func targetFile(work, dir, value string, known map[string]bool) (string, bool) {
	if remoteRE.MatchString(value) || path.IsAbs(value) {
		return "", false
	}
	p := path.Clean(path.Join(dir, value))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	abs := filepath.Join(work, filepath.FromSlash(p))
	st, err := os.Lstat(abs)
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		return "", false
	}
	if !st.IsDir() {
		return p, true
	}
	for _, n := range konfig.RecognizedKustomizationFileNames() {
		s := path.Join(p, n)
		if known[s] {
			return s, true
		}
	}
	return "", false
}
func pathExists(work, dir, value string) bool {
	if remoteRE.MatchString(value) || path.IsAbs(value) {
		return false
	}
	p := path.Clean(path.Join(dir, value))
	if p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	st, err := os.Lstat(filepath.Join(work, filepath.FromSlash(p)))
	return err == nil && st.Mode()&os.ModeSymlink == 0
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		return 0, errOutputCap
	}
	return b.Buffer.Write(p)
}

type runner struct {
	binary  string
	timeout time.Duration
	env     []string
}

func (r runner) run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.binary, args...)
	cmd.Env = r.env
	so := &boundedBuffer{limit: maxOutput}
	se := &boundedBuffer{limit: 1 << 20}
	cmd.Stdout = so
	cmd.Stderr = se
	err := cmd.Run()
	if so.exceeded || se.exceeded {
		return nil, nil, errOutputCap
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	return so.Bytes(), se.Bytes(), err
}
func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
func build(ctx context.Context, r runner, work, rootRel string) ([]byte, error) {
	so, se, err := r.run(ctx, "build", filepath.Join(work, filepath.FromSlash(path.Dir(rootRel))))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, firstLine(se))
	}
	return so, nil
}
func variantBuild(ctx context.Context, r runner, work, rootRel, fileRel string, src []byte, e entry) ([]byte, error) {
	v, err := removeEntry(src, e)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(work, filepath.FromSlash(fileRel))
	if err := os.WriteFile(p, v, 0644); err != nil {
		if restoreErr := os.WriteFile(p, src, 0644); restoreErr != nil {
			return nil, fmt.Errorf("write variant: %v; restore: %w", err, restoreErr)
		}
		return nil, err
	}
	got, buildErr := build(ctx, r, work, rootRel)
	if restoreErr := os.WriteFile(p, src, 0644); restoreErr != nil {
		return nil, fmt.Errorf("restore %s: %w", fileRel, restoreErr)
	}
	return got, buildErr
}
func nativeEnv(scratch, binary string) []string {
	p := filepath.Join(scratch, "home")
	_ = os.MkdirAll(p, 0700)
	_ = os.MkdirAll(filepath.Join(scratch, "tmp"), 0700)
	_ = os.MkdirAll(filepath.Join(scratch, "xdg"), 0700)
	return []string{"HOME=" + p, "TMPDIR=" + filepath.Join(scratch, "tmp"), "XDG_CONFIG_HOME=" + filepath.Join(scratch, "xdg"), "XDG_CACHE_HOME=" + filepath.Join(scratch, "xdg"), "KUBECONFIG=/dev/null", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9", "ALL_PROXY=http://127.0.0.1:9", "NO_PROXY=", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=file", "GIT_SSH_COMMAND=/usr/bin/false", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
}
func main() {
	subject := flag.String("subject", "", "checkout")
	binary := flag.String("kustomize", "kustomize", "native Kustomize 5.8.1 binary")
	scratch := flag.String("scratch", "", "required isolated scratch directory")
	filesOut := flag.String("files-out", "", "built kustomization file list")
	receiptOut := flag.String("receipt-out", "", "private counts and hashes receipt")
	expectedSHA := flag.String("kustomize-sha256", "", "required SHA256 of pinned native binary")
	timeout := flag.Duration("timeout", 2*time.Minute, "per build")
	flag.Parse()
	if *subject == "" || *scratch == "" || *expectedSHA == "" {
		fatal(errors.New("-subject, -scratch and -kustomize-sha256 are required"))
	}
	root, err := filepath.EvalSymlinks(*subject)
	if err != nil {
		fatal(err)
	}
	if err := guardScratch(root, *scratch); err != nil {
		fatal(err)
	}
	for _, dest := range []string{*filesOut, *receiptOut} {
		if err := guardOutput(root, dest); err != nil {
			fatal(err)
		}
	}
	bin, err := exec.LookPath(*binary)
	if err != nil {
		fatal(err)
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		fatal(err)
	}
	if _, _, err := preflightSource(root); err != nil {
		fatal(err)
	}
	actualSHA, err := fileSHA256(bin)
	if err != nil {
		fatal(err)
	}
	if !strings.EqualFold(actualSHA, *expectedSHA) {
		fatal(fmt.Errorf("native binary SHA256 mismatch: got %s", actualSHA))
	}
	owned, err := ownedScratch(root, *scratch)
	if err != nil {
		fatal(err)
	}
	ownedRunDir = owned
	defer func() { _ = os.RemoveAll(owned); ownedRunDir = "" }()
	work := filepath.Join(owned, "subject")
	if err := os.MkdirAll(work, 0700); err != nil {
		fatal(err)
	}
	symlinks, err := copyTree(root, work)
	if err != nil {
		fatal(err)
	}
	kpaths, err := findKustomizations(work)
	if err != nil {
		fatal(err)
	}
	if err := withinCap(len(kpaths), maxFiles); err != nil {
		fatal(err)
	}
	if len(kpaths) == 0 {
		fatal(errors.New("no recognized kustomization files"))
	}
	r := runner{binary: bin, timeout: *timeout, env: nativeEnv(owned, bin)}
	ctx, cancel := context.WithTimeout(context.Background(), maxWholeRun)
	defer cancel()
	ver, _, err := r.run(ctx, "version")
	if err != nil {
		fatal(fmt.Errorf("native version: %w", err))
	}
	version := strings.TrimSpace(string(ver))
	if version != "v5.8.1" {
		fatal(fmt.Errorf("expected Kustomize 5.8.1, got %q", version))
	}
	known := map[string]bool{}
	decoded := map[string]kfile{}
	for _, p := range kpaths {
		known[p] = true
	}
	totalEntries := 0
	inventory := map[string]int{}
	rawHash := sha256.New()
	for _, p := range kpaths {
		b, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(p)))
		if err != nil {
			fatal(err)
		}
		rawHash.Write([]byte(p))
		rawHash.Write(b)
		k, err := decode(p, b)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", p, err))
		}
		decoded[p] = k
		totalEntries += typedEntryCount(k.typed)
		for key, n := range k.inventory {
			inventory[key] += n
		}
	}
	if err := withinCap(totalEntries, maxEntries); err != nil {
		fatal(err)
	}
	inventory["unsupported:symlinks"] = symlinks
	// Directed local-kustomization graph. A root context is real only when a native
	// resources, bases or components edge connects it to the component.
	graph := map[string][]string{}
	for _, p := range kpaths {
		k := decoded[p]
		for i, e := range k.entries {
			f := k.fieldOf[i]
			if f.name != "resources" && f.name != "bases" && f.name != "components" {
				continue
			}
			if to, ok := targetFile(work, path.Dir(p), e.Value, known); ok && known[to] {
				graph[p] = append(graph[p], to)
			}
		}
		sort.Strings(graph[p])
	}
	roots := []string{}
	for _, p := range kpaths {
		if decoded[p].kind != "Component" {
			roots = append(roots, p)
		}
	}
	reach := func(root, target string) bool {
		seen := map[string]bool{}
		var walk func(string) bool
		walk = func(p string) bool {
			if p == target {
				return true
			}
			if seen[p] {
				return false
			}
			seen[p] = true
			for _, q := range graph[p] {
				if walk(q) {
					return true
				}
			}
			return false
		}
		return walk(root)
	}
	contexts := map[string][]string{}
	for _, p := range kpaths {
		if decoded[p].kind != "Component" {
			continue
		}
		for _, root := range roots {
			if reach(root, p) {
				contexts[p] = append(contexts[p], root)
			}
		}
		if err := withinCap(len(contexts[p]), maxContexts); err != nil {
			fatal(fmt.Errorf("%s: %w", p, err))
		}
	}
	if planned, err := plannedNativeCalls(roots, decoded, contexts); err != nil {
		fatal(fmt.Errorf("planned native calls %d: %w", planned, err))
	}
	// Admit every root against its full local closure before any native build.
	unsafeRoots := map[string]string{}
	for _, p := range roots {
		unsafeRoots[p] = closureUnsafeReason(p, graph, decoded)
	}
	result := out{Tool: "kustomize", Version: version, Tier: "A", Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	rec := receipt{Version: version, BinaryHash: actualSHA, KustomizationsHash: hex.EncodeToString(rawHash.Sum(nil)), Files: len(kpaths), Entries: totalEntries, Excluded: map[string]int{}, Inventory: inventory, Contexts: map[string]contextCounts{}, Stats: map[string]int{}}
	baselines := map[string][]byte{}
	var built []string
	for _, p := range roots {
		k := decoded[p]
		invalid := unsafeRoots[p]
		for i, e := range k.entries {
			if !k.fieldOf[i].paths {
				continue
			}
			if invalid != "" {
				break
			}
			if !pathExists(work, path.Dir(p), e.Value) {
				invalid = "missing-local-path"
				break
			}
		}
		if invalid != "" {
			rec.Excluded[invalid]++
			continue
		}
		b, err := build(ctx, r, work, p)
		if err != nil {
			if !nativeRejection(err) {
				fatal(err)
			}
			rec.Excluded["native-build-failed"]++
			continue
		}
		baselines[p] = b
		built = append(built, p)
		rec.Built++
	}
	for _, p := range kpaths {
		k := decoded[p]
		targets := []string{}
		if k.kind == "Component" {
			for _, root := range contexts[p] {
				if _, ok := baselines[root]; ok {
					targets = append(targets, root)
				}
			}
			cc := contextCounts{Valid: len(targets), Tried: len(contexts[p]), Excluded: len(contexts[p]) - len(targets)}
			rec.Contexts[p] = cc
			if len(targets) == 0 {
				rec.Excluded["component-no-valid-context"]++
				continue
			}
			built = append(built, p)
		} else {
			if _, ok := baselines[p]; !ok {
				continue
			}
			targets = []string{p}
		}
		result.Definitions = append(result.Definitions, def{Path: p, Name: k.base, Line: k.line, Kind: "package"})
		for i, e := range k.entries {
			f := k.fieldOf[i]
			rec.Stats["entries"]++
			effective := false
			for _, root := range targets {
				got, err := variantBuild(ctx, r, work, root, p, k.raw, e)
				rec.Stats["variant-builds"]++
				if err != nil && !nativeRejection(err) {
					fatal(err)
				}
				if err != nil || !bytes.Equal(got, baselines[root]) {
					effective = true
				}
			}
			if !effective {
				rec.Stats["noop"]++
				rec.Stats["noop:"+f.name]++
				if k.kind == "Component" {
					cc := rec.Contexts[p]
					cc.Noop++
					rec.Contexts[p] = cc
				}
				continue
			}
			rec.Stats["effective"]++
			if k.kind == "Component" {
				cc := rec.Contexts[p]
				cc.Effective++
				rec.Contexts[p] = cc
			}
			result.Definitions = append(result.Definitions, def{Path: p, Name: e.Value, Line: e.Line, Kind: f.kind})
			result.References = append(result.References, ref{Path: p, From: k.base, To: e.Value})
			if f.paths {
				if to, ok := targetFile(work, path.Dir(p), e.Value, known); ok {
					result.CrossReferences = append(result.CrossReferences, xref{Path: p, From: k.base, ToPath: to})
				} else {
					rec.Stats["xref-unresolved"]++
				}
			}
		}
	}
	sort.Slice(result.Definitions, func(i, j int) bool {
		a, b := result.Definitions[i], result.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(result.References, func(i, j int) bool {
		a, b := result.References[i], result.References[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.To < b.To
	})
	sort.Slice(result.CrossReferences, func(i, j int) bool {
		a, b := result.CrossReferences[i], result.CrossReferences[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})
	sort.Strings(built)
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(built, "\n")+"\n"), 0644); err != nil {
			fatal(err)
		}
	}
	if *receiptOut != "" {
		b, _ := json.MarshalIndent(rec, "", "  ")
		if err := os.WriteFile(*receiptOut, append(b, '\n'), 0600); err != nil {
			fatal(err)
		}
	}
	summary(os.Stderr, rec)
	b, err := json.MarshalIndent(result, "", " ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(b))
}
func summary(w io.Writer, r receipt) {
	fmt.Fprintf(w, "kustomizations: %d found, %d built roots; typed entries: %d; effective: %d; noop: %d\n", r.Files, r.Built, r.Entries, r.Stats["effective"], r.Stats["noop"])
	keys := make([]string, 0, len(r.Excluded))
	for k := range r.Excluded {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "excluded %s: %d\n", k, r.Excluded[k])
	}
}

var ownedRunDir string

func fatal(err error) {
	if ownedRunDir != "" {
		_ = os.RemoveAll(ownedRunDir)
	}
	fmt.Fprintln(os.Stderr, "kustomize-oracle:", err)
	os.Exit(1)
}
