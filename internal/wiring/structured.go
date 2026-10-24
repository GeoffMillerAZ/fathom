package wiring

// structured.go — tier 3: surgical, keyed edits inside files owned by other
// tools.
//
// Two shapes, because two shapes is what fathom actually writes:
//
//   - KindYAMLCommand, the lefthook shape (<group>.commands.<name>). Edited
//     LINE BY LINE, never re-serialized: a YAML round-trip through a marshal
//     loses comments, quoting style and key order, which for someone's CI
//     configuration is indistinguishable from vandalism. Every byte outside
//     the addressed key survives exactly.
//   - KindJSONHook, the Claude Code settings shape
//     (hooks.<event>[matcher].hooks[]). JSON has no comments to lose, so it
//     is parsed and re-emitted with two-space indent; every unrelated key,
//     entry and array order is preserved. Top-level key ORDER is normalized
//     by encoding/json, which is why the receipt carries the full prior
//     bytes: undo restores the original spelling exactly.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// KeyKind selects the editor a Key is addressed with.
type KeyKind string

const (
	// KindYAMLCommand addresses a named command in a lefthook-shaped YAML
	// file: Path is the full key path, e.g. {"pre-push","commands","fathom-verify"}.
	KindYAMLCommand KeyKind = "yaml-command"
	// KindJSONHook addresses one hook entry in a Claude Code settings file:
	// Path is {event, matcher} and Marker identifies fathom's own command.
	KindJSONHook KeyKind = "json-hook"
	// KindJSONValue addresses one value in a JSON settings file by its
	// object-key path ({"context","fileName"}): Body is the value as JSON.
	// Gemini CLI's context.fileName is the case (ADR-059, AG-6).
	KindJSONValue KeyKind = "json-value"
)

// Key is one entry fathom owns inside a structured file owned by someone
// else. Its identity must be statable exactly — that is the price of tier-3
// ownership, and ID() is where the price is paid.
type Key struct {
	File    string   // repo-relative, slash-separated
	Kind    KeyKind  //
	Path    []string // meaning depends on Kind (see the constants)
	Marker  string   // KindJSONHook: the command substring that is fathom's signature
	Body    string   // KindYAMLCommand: the child block; KindJSONHook: the command
	Timeout int      // KindJSONHook: the hook timeout in seconds, omitted when 0
	// Async is, for a KindJSONHook key, `"async": true` on the entry: the
	// harness runs it in the background (RT-169, observe-only bindings).
	Async bool
	// AdoptUntracked is the pre-ledger migration switch (INC-12), the key
	// counterpart of Region.AdoptUntracked: an entry that is PRESENT and
	// identified as fathom's own (KindJSONHook: by Marker) but absent from
	// .fathom/owned.json was written by a fathom that kept no ledger. The
	// caller that sets this asserts the entry's identity is fathom's, so an
	// out-of-date one is updated to the template and recorded rather than
	// left as "someone else's". An entry that already matches is recorded
	// without changing its bytes. The engine default (false) stays strict.
	AdoptUntracked bool
	// ShippedTimeout is, for a KindJSONHook key whose Timeout the caller
	// adopted from the file, the timeout fathom's template ships (0: Timeout
	// is already the template's). An entry equal to the shipped rendering is
	// fathom's own even when the per-checkout ledger names an older release.
	ShippedTimeout int
}

// shippedValue is the value fathom's template ships at this key: want,
// unless the caller adopted a timeout that differs from the template's.
func (k Key) shippedValue(want string) string {
	if k.Kind != KindJSONHook || k.ShippedTimeout == 0 || k.ShippedTimeout == k.Timeout {
		return want
	}
	sk := k
	sk.Timeout = k.ShippedTimeout
	v, err := sk.desired()
	if err != nil {
		return want
	}
	return v
}

// ID is the key's stable address, and the string the ownership ledger
// records. It must be derivable from the Key alone so the spec and the
// ledger can never disagree about what "this key" refers to.
func (k Key) ID() string {
	if k.Kind == KindJSONHook {
		return strings.Join(k.Path, "|") + "|" + k.Marker
	}
	return strings.Join(k.Path, ".")
}

// desired is the canonical spelling of the value fathom wants at this key.
// It is what gets digested into the ledger, so it must round-trip: reading
// back a key fathom just wrote must produce this exact string.
func (k Key) desired() (string, error) {
	switch k.Kind {
	case KindYAMLCommand:
		return normalizeBlock(k.Body), nil
	case KindJSONHook:
		b, err := json.Marshal(k.hookEntry())
		if err != nil {
			return "", err
		}
		return string(b), nil
	case KindJSONValue:
		if len(k.Path) == 0 {
			return "", fmt.Errorf("json value key wants a key path")
		}
		var v any
		if err := json.Unmarshal([]byte(k.Body), &v); err != nil {
			return "", fmt.Errorf("json value key body is not JSON: %w", err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unknown key kind %q", k.Kind)
	}
}

func (k Key) hookEntry() map[string]any {
	e := map[string]any{"type": "command", "command": k.Body}
	if k.Timeout > 0 {
		e["timeout"] = float64(k.Timeout)
	}
	if k.Async {
		e["async"] = true
	}
	return e
}

// current reads the value at this key out of content, and reports whether
// the key is there at all. Absent is not empty (law 3).
func (k Key) current(content string) (string, bool, error) {
	switch k.Kind {
	case KindYAMLCommand:
		return k.yamlCurrent(content)
	case KindJSONHook:
		return k.jsonCurrent(content)
	case KindJSONValue:
		return k.jsonValueCurrent(content)
	default:
		return "", false, fmt.Errorf("unknown key kind %q", k.Kind)
	}
}

// place returns content with this key set to fathom's desired value,
// touching nothing else.
func (k Key) place(content string) (string, error) {
	switch k.Kind {
	case KindYAMLCommand:
		return k.yamlPlace(content)
	case KindJSONHook:
		return k.jsonPlace(content)
	case KindJSONValue:
		return k.jsonValuePlace(content)
	default:
		return "", fmt.Errorf("unknown key kind %q", k.Kind)
	}
}

// ─── YAML, line by line ──────────────────────────────────────────────────

// splitKeepEnds splits content into lines with their terminators attached,
// so joining them back is byte-for-byte lossless.
func splitKeepEnds(content string) []string {
	if content == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			out = append(out, content[start:i+1])
			start = i + 1
		}
	}
	if start < len(content) {
		out = append(out, content[start:])
	}
	return out
}

// lineIndent returns a line's indentation, or -1 for a line that carries no
// structure (blank, or a comment) — such lines must not terminate a block,
// because a comment inside someone's YAML belongs to whatever surrounds it.
func lineIndent(l string) int {
	s := strings.TrimRight(l, "\r\n")
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i == len(s) || s[i] == '#' || s[i] == '\t' {
		return -1
	}
	return i
}

// lineKey returns the plain mapping key a line declares, or "" when the
// line is not a plain mapping key (a sequence item, a comment, a blank).
func lineKey(l string) string {
	s := strings.TrimRight(l, "\r\n")
	s = strings.TrimLeft(s, " ")
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "-") {
		return ""
	}
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return ""
	}
	k := s[:i]
	if strings.ContainsAny(k, " \t") {
		return ""
	}
	return strings.Trim(k, `"'`)
}

// lineInlineValue returns the scalar written on the key's own line, if any.
func lineInlineValue(l string) string {
	s := strings.TrimRight(l, "\r\n")
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(s[i+1:])
}

// blockEnd returns the index one past the last line belonging to a block
// whose parent is indented parentIndent — the first structural line at or
// left of the parent's own indent ends it.
func blockEnd(lines []string, lo, hi, parentIndent int) int {
	for i := lo; i < hi; i++ {
		ind := lineIndent(lines[i])
		if ind < 0 {
			continue
		}
		if ind <= parentIndent {
			return i
		}
	}
	return hi
}

// trimTrailingFiller walks an index back past blank and comment lines, so
// an insertion lands immediately after the block's last real content rather
// than after a blank separator or a comment written for the NEXT sibling.
func trimTrailingFiller(lines []string, lo, end int) int {
	for end > lo && lineIndent(lines[end-1]) < 0 {
		end--
	}
	return end
}

type yamlSpot struct {
	found     bool
	keyLine   int      // index of the "<key>:" line
	blockEnd  int      // index one past the key's last line
	indent    int      // the key line's indent
	insertAt  int      // where a missing key should be written
	insertInd int      // at what indent
	missing   []string // the path segments still to be created
}

// yamlLocate walks a key path through a YAML document by indentation. It
// deliberately understands only plain block mappings — the shape lefthook
// files use — and reports "not found, insert here" for anything else rather
// than guessing at flow mappings, anchors or multi-document files.
func yamlLocate(lines []string, path []string) yamlSpot {
	lo, hi, parentIndent := 0, len(lines), -1
	for si := 0; si < len(path); si++ {
		end := blockEnd(lines, lo, hi, parentIndent)
		childIndent, found := -1, -1
		for i := lo; i < end; i++ {
			ind := lineIndent(lines[i])
			if ind < 0 {
				continue
			}
			if childIndent < 0 {
				childIndent = ind
			}
			if ind != childIndent {
				continue
			}
			if lineKey(lines[i]) == path[si] {
				found = i
				break
			}
		}
		if found < 0 {
			ind := childIndent
			if ind < 0 {
				ind = parentIndent + 2
			}
			return yamlSpot{
				insertAt:  trimTrailingFiller(lines, lo, end),
				insertInd: ind,
				missing:   path[si:],
			}
		}
		parentIndent = lineIndent(lines[found])
		lo, hi = found+1, end
		if si == len(path)-1 {
			be := trimTrailingFiller(lines, lo, blockEnd(lines, lo, hi, parentIndent))
			return yamlSpot{found: true, keyLine: found, blockEnd: be, indent: parentIndent}
		}
	}
	return yamlSpot{}
}

// normalizeBlock is the comparison form of a YAML value: trailing
// whitespace per line removed, trailing blank lines dropped, CRLF folded to
// LF. Two spellings that differ only in those respects are the same value,
// and fathom must not "update" a key to change nothing.
func normalizeBlock(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func (k Key) yamlCurrent(content string) (string, bool, error) {
	lines := splitKeepEnds(content)
	spot := yamlLocate(lines, k.Path)
	if !spot.found {
		return "", false, nil
	}
	if inline := lineInlineValue(lines[spot.keyLine]); inline != "" {
		return normalizeBlock(inline), true, nil
	}
	child := lines[spot.keyLine+1 : spot.blockEnd]
	min := -1
	for _, l := range child {
		if ind := lineIndent(l); ind >= 0 && (min < 0 || ind < min) {
			min = ind
		}
	}
	if min < 0 {
		return "", true, nil
	}
	var b strings.Builder
	for _, l := range child {
		s := strings.TrimRight(l, "\r\n")
		if len(s) >= min {
			s = s[min:]
		} else {
			s = strings.TrimLeft(s, " ")
		}
		b.WriteString(s)
		b.WriteString("\n")
	}
	return normalizeBlock(b.String()), true, nil
}

// renderYAMLKey renders "<name>:" plus its child block at the given indent.
func renderYAMLKey(name, body string, indent int, ending string) []string {
	out := []string{strings.Repeat(" ", indent) + name + ":" + ending}
	for _, l := range strings.Split(normalizeBlock(body), "\n") {
		if strings.TrimSpace(l) == "" {
			out = append(out, ending)
			continue
		}
		out = append(out, strings.Repeat(" ", indent+2)+l+ending)
	}
	return out
}

func (k Key) yamlPlace(content string) (string, error) {
	if len(k.Path) == 0 {
		return "", fmt.Errorf("yaml key has no path")
	}
	ending := lineEnding(content)
	lines := splitKeepEnds(content)
	// A file whose last line has no terminator cannot have anything appended
	// after it without one; adding it is the single byte fathom is allowed to
	// change outside its own key, and only when it is appending.
	if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
		lines[n-1] += ending
	}
	spot := yamlLocate(lines, k.Path)

	name := k.Path[len(k.Path)-1]
	if spot.found {
		next := append([]string{}, lines[:spot.keyLine]...)
		next = append(next, renderYAMLKey(name, k.Body, spot.indent, ending)...)
		next = append(next, lines[spot.blockEnd:]...)
		return strings.Join(next, ""), nil
	}

	// Build the missing chain, then the key itself, at the insertion point.
	var block []string
	ind := spot.insertInd
	for i, seg := range spot.missing {
		if i == len(spot.missing)-1 {
			block = append(block, renderYAMLKey(seg, k.Body, ind, ending)...)
			break
		}
		block = append(block, strings.Repeat(" ", ind)+seg+":"+ending)
		ind += 2
	}
	next := append([]string{}, lines[:spot.insertAt]...)
	next = append(next, block...)
	next = append(next, lines[spot.insertAt:]...)
	return strings.Join(next, ""), nil
}

// ─── JSON hook entries ───────────────────────────────────────────────────

func parseJSONObject(content string) (map[string]any, error) {
	root := map[string]any{}
	if strings.TrimSpace(content) == "" {
		return root, nil
	}
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	return root, nil
}

// findHookEntry returns fathom's own entry in the addressed matcher group,
// identified by Marker appearing in its command. Identity by marker (not by
// exact command) is what makes the check survive a repointed binary path.
func (k Key) findHookEntry(root map[string]any) map[string]any {
	if len(k.Path) != 2 {
		return nil
	}
	event, matcher := k.Path[0], k.Path[1]
	hooks, _ := root["hooks"].(map[string]any)
	arr, _ := hooks[event].([]any)
	for _, g := range arr {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		gMatcher, _ := gm["matcher"].(string)
		if gMatcher != matcher {
			continue
		}
		entries, _ := gm["hooks"].([]any)
		for _, e := range entries {
			em, ok := e.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := em["command"].(string)
			if strings.Contains(cmd, k.Marker) {
				return em
			}
		}
	}
	return nil
}

func (k Key) jsonCurrent(content string) (string, bool, error) {
	root, err := parseJSONObject(content)
	if err != nil {
		return "", false, err
	}
	entry := k.findHookEntry(root)
	if entry == nil {
		return "", false, nil
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

func (k Key) jsonPlace(content string) (string, error) {
	if len(k.Path) != 2 {
		return "", fmt.Errorf("json hook key wants {event, matcher}, got %v", k.Path)
	}
	event, matcher := k.Path[0], k.Path[1]
	root, err := parseJSONObject(content)
	if err != nil {
		return "", err
	}

	if entry := k.findHookEntry(root); entry != nil {
		for _, key := range sortedKeys(entry) {
			delete(entry, key)
		}
		for key, v := range k.hookEntry() {
			entry[key] = v
		}
		return marshalJSONFile(root)
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	arr, _ := hooks[event].([]any)
	placed := false
	for i, g := range arr {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		gMatcher, _ := gm["matcher"].(string)
		if gMatcher != matcher {
			continue
		}
		entries, _ := gm["hooks"].([]any)
		gm["hooks"] = append(entries, any(k.hookEntry()))
		arr[i] = gm
		placed = true
		break
	}
	if !placed {
		group := map[string]any{"hooks": []any{any(k.hookEntry())}}
		if matcher != "" {
			group["matcher"] = matcher
		}
		arr = append(arr, any(group))
	}
	hooks[event] = arr
	root["hooks"] = hooks
	return marshalJSONFile(root)
}

func marshalJSONFile(root map[string]any) (string, error) {
	// No HTML escaping: a hook command's `>` and `&` stay readable in the
	// file (the digests are computed elsewhere, over json.Marshal).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(root); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── JSON, one value by key path ─────────────────────────────────────────

func (k Key) jsonValueCurrent(content string) (string, bool, error) {
	root, err := parseJSONObject(content)
	if err != nil {
		return "", false, err
	}
	var cur any = root
	for _, seg := range k.Path {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false, nil
		}
		if cur, ok = m[seg]; !ok {
			return "", false, nil
		}
	}
	b, err := json.Marshal(cur)
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

func (k Key) jsonValuePlace(content string) (string, error) {
	want, err := k.desired()
	if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal([]byte(want), &v); err != nil {
		return "", err
	}
	root, err := parseJSONObject(content)
	if err != nil {
		return "", err
	}
	m := root
	for i, seg := range k.Path {
		if i == len(k.Path)-1 {
			m[seg] = v
			break
		}
		next, present := m[seg]
		if !present {
			child := map[string]any{}
			m[seg] = child
			m = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s is not an object — fathom will not replace it", strings.Join(k.Path[:i+1], "."))
		}
		m = child
	}
	return marshalJSONFile(root)
}
