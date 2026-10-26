package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// wire implements "wire --host H [--bin PATH] [--dry-run]": merge this
// pack's hook registration into the host's user-level hook file.
//
// The merge is idempotent and conservative. An event that already has an
// agent-hooks handler is left exactly as it is; otherwise the pack's group
// is appended to the end of that event's array. Nothing is removed or
// reordered: Codex keys hook trust by file, event, group index and handler
// index, so appending keeps every existing approval valid. Object key order
// is preserved, the original is backed up once, and the file is replaced
// atomically. --dry-run prints the result instead of writing it.
func (a *app) wire(args []string) int {
	flags := flag.NewFlagSet("wire", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	host := flags.String("host", "", "claude or codex")
	bin := flags.String("bin", portableHome+".local/bin/agent-hooks", "agent-hooks path: absolute, or $HOME/... to resolve per machine")
	dryRun := flags.Bool("dry-run", false, "print the merged file instead of writing it")
	if flags.Parse(args) != nil {
		return 2
	}
	if *host != hostClaude && *host != hostCodex {
		fmt.Fprintln(a.stderr, "agent-hooks wire: --host must be claude or codex")
		return 2
	}
	if !filepath.IsAbs(*bin) && !strings.HasPrefix(*bin, portableHome) {
		fmt.Fprintln(a.stderr, "agent-hooks wire: --bin must be an absolute path or start with $HOME/")
		return 2
	}
	path := a.hostHookFile(*host)
	original, err := os.ReadFile(path)
	switch {
	case isNotExist(err):
		original = []byte("{}")
	case err != nil:
		fmt.Fprintf(a.stderr, "agent-hooks wire: %v\n", err)
		return 1
	}
	tmpl, err := hookTemplate(*host, *bin)
	if err != nil {
		fmt.Fprintf(a.stderr, "agent-hooks wire: %v\n", err)
		return 1
	}
	merged, added, err := mergeHooks(original, []byte(tmpl), *host)
	if err != nil {
		fmt.Fprintf(a.stderr, "agent-hooks wire: %s: %v\n", path, err)
		return 1
	}
	if *dryRun {
		_, _ = a.stdout.Write(merged)
		return 0
	}
	if len(added) == 0 {
		fmt.Fprintf(a.stdout, "%s already wires agent-hooks for every event; unchanged\n", path)
		return 0
	}
	if err := writeAtomic(path, merged, original); err != nil {
		fmt.Fprintf(a.stderr, "agent-hooks wire: %v\n", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "%s: added agent-hooks for %s\n", path, strings.Join(added, ", "))
	if *host == hostCodex {
		fmt.Fprintln(a.stdout, "Codex skips new hooks until they are trusted: approve them once with /hooks.")
	}
	return 0
}

// hostHookFile is the user-level file a host reads hook registrations from.
func (a *app) hostHookFile(host string) string {
	if host == hostCodex {
		return filepath.Join(a.codexHome(), "hooks.json")
	}
	return filepath.Join(a.claudeHome(), "settings.json")
}

// mergeHooks appends each template event's groups to the document's hooks
// unless that event already runs agent-hooks for this host. It returns the
// merged document (indented, key order preserved) and the events it added.
func mergeHooks(doc, tmpl []byte, host string) ([]byte, []string, error) {
	top, err := decodeOrdered(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("not a JSON object: %w", err)
	}
	var want struct {
		Hooks orderedObject `json:"hooks"`
	}
	if err := json.Unmarshal(tmpl, &want); err != nil {
		return nil, nil, err
	}
	var hooks orderedObject
	if raw, ok := top.get("hooks"); ok {
		if hooks, err = decodeOrdered(raw); err != nil {
			return nil, nil, fmt.Errorf("hooks is not a JSON object: %w", err)
		}
	}
	marker := host
	var added []string
	for _, ev := range want.Hooks {
		var groups []json.RawMessage
		if raw, ok := hooks.get(ev.Key); ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return nil, nil, fmt.Errorf("hooks.%s is not an array: %w", ev.Key, err)
			}
		}
		if wiresAgentHooks(groups, marker) {
			continue
		}
		var newGroups []json.RawMessage
		if err := json.Unmarshal(ev.Value, &newGroups); err != nil {
			return nil, nil, err
		}
		merged, err := marshalNoEscape(append(groups, newGroups...))
		if err != nil {
			return nil, nil, err
		}
		hooks.set(ev.Key, merged)
		added = append(added, ev.Key)
	}
	hooksRaw, err := hooks.MarshalJSON()
	if err != nil {
		return nil, nil, err
	}
	top.set("hooks", hooksRaw)
	flat, err := top.MarshalJSON()
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, flat, "", "  "); err != nil {
		return nil, nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), added, nil
}

// wiresAgentHooks reports whether any handler in the groups runs agent-hooks
// for the host named by marker.
func wiresAgentHooks(groups []json.RawMessage, marker string) bool {
	for _, g := range groups {
		var group struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		}
		if json.Unmarshal(g, &group) != nil {
			continue
		}
		for _, h := range group.Hooks {
			if runsAgentHooks(h.Command, marker) {
				return true
			}
		}
	}
	return false
}

// writeAtomic replaces path with data through a temp file in the same
// directory, keeping the original's permission bits (0600 for a new file),
// after saving a one-time backup of the original next to it.
func writeAtomic(path string, data, original []byte) error {
	if target, err := filepath.EvalSymlinks(path); err == nil && target != path {
		path = target // replace the managed file, not the link that points at it
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
		backup := path + ".agent-hooks.bak"
		if _, err := os.Stat(backup); isNotExist(err) {
			if err := os.WriteFile(backup, original, mode); err != nil {
				return fmt.Errorf("backup %s: %w", backup, err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// orderedObject is a JSON object that keeps its key order through a
// decode/encode round trip, so a merge only changes what it means to.
type orderedObject []struct {
	Key   string
	Value json.RawMessage
}

func (o orderedObject) get(key string) (json.RawMessage, bool) {
	for _, kv := range o {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return nil, false
}

func (o *orderedObject) set(key string, value json.RawMessage) {
	for i := range *o {
		if (*o)[i].Key == key {
			(*o)[i].Value = value
			return
		}
	}
	*o = append(*o, struct {
		Key   string
		Value json.RawMessage
	}{key, value})
}

// UnmarshalJSON decodes an object, keeping key order.
func (o *orderedObject) UnmarshalJSON(raw []byte) error {
	obj, err := decodeOrdered(raw)
	*o = obj
	return err
}

// MarshalJSON encodes the object in its stored key order.
func (o orderedObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshalNoEscape(kv.Key)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(kv.Value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func decodeOrdered(raw []byte) (orderedObject, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("expected an object")
	}
	var obj orderedObject
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		obj.set(key, value)
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("trailing data after the object")
	}
	return obj, nil
}
