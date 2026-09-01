// Package config is the cli-kit's CUE-config loader: schema defaults,
// system config, user config, project config, and caller-supplied
// overrides, unified in one deterministic order through a real
// io/fs-backed seam so nothing ever touches a real $HOME in a test.
//
// Layer order (lowest → highest): schema defaults → system
// (/etc/<app>/config.cue) → user ($XDG_CONFIG_HOME or ~/.config/<app>/
// config.cue) → project (walking cwd upward for <app>.cue, bounded at the
// git worktree root) → explicit overrides.
//
// "Lowest → highest" describes intent, not CUE override priority — CUE
// has no such thing. Two layers that set the same field to different
// concrete values do not silently pick a winner; unification produces a
// bottom and Load reports it as a refusal (TestConfigConflictRefused).
// That is deliberate: a config loader that let a later file silently
// clobber an earlier one's concrete value would hide exactly the mistake
// a layered config system exists to catch.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Host is the filesystem + environment seam Load reads through: fs.FS
// carries every file read (rooted at "/" — see hostPath), Getenv and
// Getwd supply the two pieces of process state the layer search needs
// (XDG_CONFIG_HOME/HOME, and the project walk's starting directory)
// without Load ever touching the real environment or working directory.
// NewOSHost wraps the real filesystem; tests use an fstest.MapFS-backed
// Host instead.
type Host interface {
	fs.FS
	Getenv(key string) string
	Getwd() (string, error)
}

// Layer describes one config layer Load actually applied.
type Layer struct {
	// Name is the layer's role: "schema", "system", "user", "project", or
	// "overrides".
	Name string
	// Path is the file the layer was read from. Empty for "schema" and
	// "overrides", which are not file-backed.
	Path string
	// Digest is the sha256 hex digest of the layer file's raw bytes.
	// Empty for "schema" and "overrides".
	Digest string
}

// Options configures Load. Host is required; CWD, if empty, comes from
// Host.Getwd().
type Options struct {
	Host Host
	// CWD is the starting directory for the project-layer walk. Empty
	// means Host.Getwd().
	CWD string
	// Overrides is unified last (highest) — the caller-provided layer,
	// e.g. values already resolved from flags or environment variables
	// by the caller before Load ever runs.
	Overrides map[string]any
}

// Result is Load's output: the fully unified cue.Value and the ordered
// list of layers that actually contributed to it. A file-backed layer
// whose file did not exist lands in Absent instead — Explain prints those
// too, because "where COULD config live" is exactly what a person running
// `config explain` on a fresh machine is asking.
type Result struct {
	// Value is the schema unified with every loaded layer, in order.
	Value Value
	// Layers is Load's Layer list, in application order (index 0 is
	// always "schema").
	Layers []Layer
	// Absent lists the file-backed layers that were checked and not found,
	// in check order. Path holds the location (or, for the project layer,
	// the walk description) that was checked; Digest is always empty.
	Absent []Layer

	// snapshots[i] is Value's cumulative state after applying Layers[0..i]
	// inclusive — Explain's field-tracing reads this, never re-derives it.
	snapshots []Value
}

// Value is a decoded CUE configuration value. It is cue.Value itself —
// Result does not impose a second, app-specific Go struct shape; callers
// decode the fields they care about with Value.Decode.
type Value = cue.Value

// Load unifies app's config schema with every layer that resolves for it,
// in the fixed lowest-to-highest order (package doc), and returns the
// result. Any layer conflict — including one between the schema's own
// defaults/constraints and a layer's concrete value — is returned as an
// error rather than silently resolved.
func Load(app string, schemaSrc []byte, o Options) (*Result, error) {
	if o.Host == nil {
		return nil, fmt.Errorf("config: Load(%q): Options.Host is required", app)
	}
	if app == "" {
		return nil, fmt.Errorf("config: Load: app name is required")
	}
	host := o.Host
	ctx := cuecontext.New()

	schemaVal := ctx.CompileBytes(schemaSrc, cue.Filename(app+" schema"))
	if err := schemaVal.Err(); err != nil {
		return nil, fmt.Errorf("config: Load(%q): compile schema: %w", app, err)
	}

	cwd := o.CWD
	if cwd == "" {
		wd, err := host.Getwd()
		if err != nil {
			return nil, fmt.Errorf("config: Load(%q): Getwd: %w", app, err)
		}
		cwd = wd
	}

	cumulative := schemaVal
	layers := []Layer{{Name: "schema"}}
	absent := []Layer{}
	snapshots := []Value{cumulative}

	apply := func(name, filePath string) error {
		v, l, err := loadFileLayer(ctx, host, name, filePath)
		if err != nil {
			return err
		}
		if l == nil {
			// Not an error — but worth remembering: Explain teaches where
			// config could live, not just where it was found.
			absent = append(absent, Layer{Name: name, Path: filePath})
			return nil
		}
		cumulative = cumulative.Unify(v)
		layers = append(layers, *l)
		snapshots = append(snapshots, cumulative)
		return nil
	}

	if err := apply("system", path.Join("/etc", app, "config.cue")); err != nil {
		return nil, fmt.Errorf("config: Load(%q): %w", app, err)
	}

	if userPath, ok := userConfigPath(host, app); ok {
		if err := apply("user", userPath); err != nil {
			return nil, fmt.Errorf("config: Load(%q): %w", app, err)
		}
	}

	projectPath, ok, err := findProjectConfig(host, app, cwd)
	if err != nil {
		return nil, fmt.Errorf("config: Load(%q): project config walk: %w", app, err)
	}
	if ok {
		if err := apply("project", projectPath); err != nil {
			return nil, fmt.Errorf("config: Load(%q): %w", app, err)
		}
	} else {
		absent = append(absent, Layer{Name: "project", Path: app + ".cue (walked " + cwd + " up to the git root)"})
	}

	if len(o.Overrides) > 0 {
		ov := ctx.Encode(o.Overrides)
		if err := ov.Err(); err != nil {
			return nil, fmt.Errorf("config: Load(%q): encode overrides: %w", app, err)
		}
		cumulative = cumulative.Unify(ov)
		layers = append(layers, Layer{Name: "overrides"})
		snapshots = append(snapshots, cumulative)
	}

	if err := cumulative.Err(); err != nil {
		return nil, fmt.Errorf("config: Load(%q): layer conflict: %w", app, err)
	}
	if err := cumulative.Validate(cue.Concrete(false)); err != nil {
		return nil, fmt.Errorf("config: Load(%q): layer conflict: %w", app, err)
	}

	return &Result{Value: cumulative, Layers: layers, Absent: absent, snapshots: snapshots}, nil
}

// loadFileLayer reads and compiles one file-backed layer. A missing file
// is reported as (zero Value, nil Layer, nil error) — absence, not
// failure; any other read/compile error is returned.
func loadFileLayer(ctx *cue.Context, host Host, name, absPath string) (Value, *Layer, error) {
	exists, err := fileExists(host, absPath)
	if err != nil {
		return cue.Value{}, nil, fmt.Errorf("stat %s: %w", absPath, err)
	}
	if !exists {
		return cue.Value{}, nil, nil
	}
	data, err := fs.ReadFile(host, hostPath(absPath))
	if err != nil {
		return cue.Value{}, nil, fmt.Errorf("read %s: %w", absPath, err)
	}
	v := ctx.CompileBytes(data, cue.Filename(absPath))
	if err := v.Err(); err != nil {
		return cue.Value{}, nil, fmt.Errorf("compile %s: %w", absPath, err)
	}
	sum := sha256.Sum256(data)
	return v, &Layer{Name: name, Path: absPath, Digest: hex.EncodeToString(sum[:])}, nil
}

// userConfigPath resolves the user layer's path: $XDG_CONFIG_HOME/<app>/
// config.cue if XDG_CONFIG_HOME is set, else ~/.config/<app>/config.cue.
// ok is false only when neither XDG_CONFIG_HOME nor HOME resolves —
// there is then no path to even check for existence.
func userConfigPath(host Host, app string) (string, bool) {
	if xdg := host.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return path.Join(xdg, app, "config.cue"), true
	}
	home := host.Getenv("HOME")
	if home == "" {
		return "", false
	}
	return path.Join(home, ".config", app, "config.cue"), true
}

// findProjectConfig walks from start upward looking for <app>.cue,
// BOUNDED at the git worktree root: once a directory containing .git has
// been checked (whether or not it held <app>.cue), the walk stops — it
// never reads a config file above the repo. If the walk reaches the
// filesystem root without ever finding a .git, it likewise stops there;
// nothing above "/" is ever considered.
func findProjectConfig(host Host, app, start string) (string, bool, error) {
	dir := path.Clean(start)
	for {
		candidate := path.Join(dir, app+".cue")
		exists, err := fileExists(host, candidate)
		if err != nil {
			return "", false, fmt.Errorf("stat %s: %w", candidate, err)
		}
		if exists {
			return candidate, true, nil
		}

		isGitRoot, err := fileExists(host, path.Join(dir, ".git"))
		if err != nil {
			return "", false, fmt.Errorf("stat %s: %w", path.Join(dir, ".git"), err)
		}
		if isGitRoot {
			return "", false, nil
		}

		parent := path.Dir(dir)
		if parent == dir {
			return "", false, nil // reached "/" without finding a git root
		}
		dir = parent
	}
}

func fileExists(host Host, absPath string) (bool, error) {
	_, err := fs.Stat(host, hostPath(absPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// hostPath converts an absolute unix-style path to the relative,
// leading-slash-free form io/fs.FS requires, given Host's fs.FS is always
// rooted at "/".
func hostPath(absPath string) string {
	p := path.Clean(absPath)
	p = trimLeadingSlash(p)
	if p == "" {
		return "."
	}
	return p
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}

// Explain writes one line per loaded layer (name, path, digest), then —
// when fieldPath is non-empty — one more line naming which loaded layer
// last affected that field: the last layer, in application order, whose
// cumulative value at fieldPath differs (by canonical JSON encoding) from
// the cumulative value one layer before it. This is the pdk TraceValue
// idea's minimal version: diff cumulative prefixes instead of tracking
// provenance per-node.
func (r *Result) Explain(w io.Writer, fieldPath string) error {
	for _, l := range r.Layers {
		if l.Path == "" {
			fmt.Fprintf(w, "%s: (built in)\n", l.Name)
			continue
		}
		fmt.Fprintf(w, "%s: %s (sha256:%s)\n", l.Name, l.Path, l.Digest)
	}
	for _, l := range r.Absent {
		fmt.Fprintf(w, "%s: (absent) %s\n", l.Name, l.Path)
	}
	if fieldPath == "" {
		return nil
	}
	layerName, found, err := r.traceField(fieldPath)
	if err != nil {
		return fmt.Errorf("config: Explain: %w", err)
	}
	if !found {
		fmt.Fprintf(w, "%s: not set by any loaded layer\n", fieldPath)
		return nil
	}
	fmt.Fprintf(w, "%s: last set by %s\n", fieldPath, layerName)
	return nil
}

// traceField finds the last layer (by application order) whose cumulative
// snapshot changed fieldPath's canonical encoding relative to the
// snapshot immediately before it.
func (r *Result) traceField(fieldPath string) (layerName string, found bool, err error) {
	p := cue.ParsePath(fieldPath)
	if err := p.Err(); err != nil {
		return "", false, fmt.Errorf("bad field path %q: %w", fieldPath, err)
	}

	var prevJSON []byte
	prevExists := false
	for i, snap := range r.snapshots {
		v := snap.LookupPath(p)
		curExists := v.Exists()
		var curJSON []byte
		if curExists {
			// A value that exists but is not yet concrete (e.g. only a
			// schema default with no layer having set it) still encodes —
			// MarshalJSON resolves defaults. An incomplete, non-default
			// value fails to encode; treated as "no change to report"
			// rather than an error, since Load already validated the
			// FINAL value and an intermediate snapshot may legitimately
			// be incomplete.
			if b, encErr := v.MarshalJSON(); encErr == nil {
				curJSON = b
			} else {
				curExists = false
			}
		}
		if curExists && (!prevExists || string(curJSON) != string(prevJSON)) {
			layerName = r.Layers[i].Name
			found = true
		}
		prevJSON, prevExists = curJSON, curExists
	}
	return layerName, found, nil
}
