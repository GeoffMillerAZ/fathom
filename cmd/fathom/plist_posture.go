package main

// plist_posture.go — launchd plist schema 2, its posture check and the
// in-place reconcile (design §8.1, §8.3; RT-13, RT-14, RT-15).
//
// INC-23: the installed plist carried `KeepAlive true` and no throttle, so
// every exit (a kill, a crash, an over-budget exit) was relaunched within 10
// s, in the foreground scheduling class, forever. Schema 2 makes exit 0 mean
// "stay down", throttles relaunches to 60 s and runs the daemon in the
// Background class. An installed plist is never regenerated wholesale to get
// there: `--reconcile` rewrites only the tenancy keys and keeps every byte an
// operator owns (label, paths, store, call log, WorkingDirectory, env,
// comments).

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// plistSchemaCurrent is the schema plistBody renders.
const plistSchemaCurrent = 2

// plistSchemaMarker is the comment that names a fathom-written plist's schema.
const plistSchemaMarker = "<!-- fathom-plist-schema: 2 -->"

var plistMarkerRe = regexp.MustCompile(`<!--\s*fathom-plist-schema:\s*(\d+)\s*-->`)

// plistTenancyKey is one key --reconcile owns: its schema-2 value as XML and
// as the posture check renders it.
type plistTenancyKey struct {
	Key, XML, Want string
}

// plistTenancyKeys is the ordered set of top-level keys schema 2 owns.
// Everything else in the plist belongs to the operator.
var plistTenancyKeys = []plistTenancyKey{
	{"KeepAlive", "<dict><key>SuccessfulExit</key><false/></dict>", "{SuccessfulExit: false}"},
	{"ThrottleInterval", "<integer>60</integer>", "60"},
	{"ProcessType", "<string>Background</string>", "Background"},
	{"LowPriorityIO", "<true/>", "true"},
	{"LowPriorityBackgroundIO", "<true/>", "true"},
	{"Nice", "<integer>5</integer>", "5"},
	{"ExitTimeOut", "<integer>20</integer>", "20"},
}

// plistNode is one parsed plist value with its byte span in the source.
type plistNode struct {
	Kind       string // true false integer real string date data dict array
	Text       string
	Start, End int64
	Keys       []string     // dict: child keys, in order
	Children   []*plistNode // dict: values (parallel to Keys); array: elements
}

// render is a short, stable spelling of the value for posture diffs.
func (n *plistNode) render() string {
	switch n.Kind {
	case "true", "false":
		return n.Kind
	case "dict":
		parts := make([]string, len(n.Keys))
		for i, k := range n.Keys {
			parts[i] = k + ": " + n.Children[i].render()
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case "array":
		parts := make([]string, len(n.Children))
		for i, c := range n.Children {
			parts[i] = c.render()
		}
		return "[" + strings.Join(parts, " ") + "]"
	default:
		return n.Text
	}
}

// plistDoc is a parsed plist: the top-level dict with source offsets.
type plistDoc struct {
	raw      []byte
	top      *plistNode
	keyStart []int64 // offset of each top-level <key>
	dictOpen int64   // offset just after the top <dict>
	dictEnd  int64   // offset of the top </dict>
}

func (p *plistDoc) entry(key string) (*plistNode, bool) {
	for i, k := range p.top.Keys {
		if k == key {
			return p.top.Children[i], true
		}
	}
	return nil, false
}

// parsePlist parses an XML plist whose root is a dict, recording byte spans.
func parsePlist(raw []byte) (*plistDoc, error) {
	d := xml.NewDecoder(bytes.NewReader(raw))
	d.Strict = true
	doc := &plistDoc{raw: raw}
	// Find <plist> then the root <dict>.
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("plist: no root dict: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "plist" {
				continue
			}
			if se.Name.Local != "dict" {
				return nil, fmt.Errorf("plist: root is <%s>, want <dict>", se.Name.Local)
			}
			doc.dictOpen = d.InputOffset()
			top, err := parsePlistDict(d, doc, true)
			if err != nil {
				return nil, err
			}
			doc.top = top
			return doc, nil
		}
	}
}

// parsePlistDict reads a dict body up to its </dict>; the <dict> start
// token has been consumed.
func parsePlistDict(d *xml.Decoder, doc *plistDoc, isTop bool) (*plistNode, error) {
	n := &plistNode{Kind: "dict"}
	var pendingKey *string
	for {
		off := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("plist: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if pendingKey != nil {
				return nil, fmt.Errorf("plist: key %q has no value", *pendingKey)
			}
			n.End = d.InputOffset()
			if isTop {
				doc.dictEnd = off
			}
			return n, nil
		case xml.StartElement:
			if pendingKey == nil {
				if t.Name.Local != "key" {
					return nil, fmt.Errorf("plist: <%s> where a <key> was expected", t.Name.Local)
				}
				text, err := plistText(d)
				if err != nil {
					return nil, err
				}
				k := text
				pendingKey = &k
				if isTop {
					doc.keyStart = append(doc.keyStart, off)
				}
				continue
			}
			v, err := parsePlistValue(d, doc, t, off)
			if err != nil {
				return nil, err
			}
			n.Keys = append(n.Keys, *pendingKey)
			n.Children = append(n.Children, v)
			pendingKey = nil
		}
	}
}

// parsePlistValue reads one value whose start token (at off) was consumed.
func parsePlistValue(d *xml.Decoder, doc *plistDoc, se xml.StartElement, off int64) (*plistNode, error) {
	switch se.Name.Local {
	case "dict":
		n, err := parsePlistDict(d, doc, false)
		if err != nil {
			return nil, err
		}
		n.Start = off
		return n, nil
	case "array":
		n := &plistNode{Kind: "array", Start: off}
		for {
			eoff := d.InputOffset()
			tok, err := d.Token()
			if err != nil {
				return nil, fmt.Errorf("plist: %w", err)
			}
			switch t := tok.(type) {
			case xml.EndElement:
				n.End = d.InputOffset()
				return n, nil
			case xml.StartElement:
				c, err := parsePlistValue(d, doc, t, eoff)
				if err != nil {
					return nil, err
				}
				n.Children = append(n.Children, c)
			}
		}
	default:
		text, err := plistText(d)
		if err != nil {
			return nil, err
		}
		return &plistNode{Kind: se.Name.Local, Text: strings.TrimSpace(text), Start: off, End: d.InputOffset()}, nil
	}
}

// plistText reads character data up to the matching end element.
func plistText(d *xml.Decoder) (string, error) {
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("plist: unexpected end")
			}
			return "", fmt.Errorf("plist: %w", err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return b.String(), nil
		case xml.StartElement:
			return "", fmt.Errorf("plist: unexpected <%s> inside a scalar", t.Name.Local)
		}
	}
}

// plistPostureReport is the posture of one installed plist against schema 2.
type plistPostureReport struct {
	// Schema is the marker's schema, or 1 for a plist without one.
	Schema int
	// FathomWritten is true when fathom rendered this plist (a schema
	// marker, or the v1 template's own BP-37 comment); otherwise
	// --reconcile needs --adopt.
	FathomWritten bool
	// UnconditionalKeepAlive: `KeepAlive true` — every exit is relaunched.
	UnconditionalKeepAlive bool
	// ThrottleSeconds is launchd's relaunch interval (10 when unset).
	ThrottleSeconds int
	// Binary is ProgramArguments[0].
	Binary string
	// Drift lists each tenancy key that differs from schema 2, sorted.
	Drift []string
}

// plistPosture parses raw and diffs its tenancy keys against schema 2.
func plistPosture(raw []byte) (plistPostureReport, error) {
	p, err := parsePlist(raw)
	if err != nil {
		return plistPostureReport{}, err
	}
	rep := plistPostureReport{Schema: 1, ThrottleSeconds: 10}
	if m := plistMarkerRe.FindSubmatch(raw); m != nil {
		rep.Schema, _ = strconv.Atoi(string(m[1]))
		rep.FathomWritten = true
	} else if bytes.Contains(raw, []byte("BP-37")) && bytes.Contains(raw, []byte("serve-http")) {
		rep.FathomWritten = true // the v1 template's own comment
	}
	if ka, ok := p.entry("KeepAlive"); ok && ka.Kind == "true" {
		rep.UnconditionalKeepAlive = true
	}
	if ti, ok := p.entry("ThrottleInterval"); ok && ti.Kind == "integer" {
		if n, err := strconv.Atoi(ti.Text); err == nil {
			rep.ThrottleSeconds = n
		}
	}
	for _, k := range plistTenancyKeys {
		got := "absent"
		if e, ok := p.entry(k.Key); ok {
			got = e.render()
		}
		if got != k.Want {
			rep.Drift = append(rep.Drift, fmt.Sprintf("%s: %s (want %s)", k.Key, got, k.Want))
		}
	}
	if args, ok := p.entry("ProgramArguments"); ok && args.Kind == "array" {
		if len(args.Children) > 0 {
			rep.Binary = args.Children[0].Text
		}
		if !plistArgsHaveManaged(args) {
			rep.Drift = append(rep.Drift, "ProgramArguments: no --managed (want serve-http --managed)")
		}
	} else {
		rep.Drift = append(rep.Drift, "ProgramArguments: absent")
	}
	if rep.Schema < plistSchemaCurrent {
		rep.Drift = append(rep.Drift, fmt.Sprintf("schema marker: %d (want %d)", rep.Schema, plistSchemaCurrent))
	}
	sort.Strings(rep.Drift)
	return rep, nil
}

func plistArgsHaveManaged(args *plistNode) bool {
	for _, c := range args.Children {
		if c.Kind == "string" && (c.Text == "--managed" || c.Text == "-managed" || c.Text == "--managed=true") {
			return true
		}
	}
	return false
}

// errPlistNeedsAdopt: the plist was not written by fathom.
var errPlistNeedsAdopt = errors.New("this plist was not written by fathom — re-run with --adopt to reconcile it anyway")

// plistEdit replaces raw[start:end] with text.
type plistEdit struct {
	start, end int64
	text       string
}

// reconcilePlist rewrites only the tenancy keys of raw to schema 2 and
// returns the new bytes and one line per change. Every other byte is kept.
// A plist fathom did not write needs adopt. A plist already at schema 2 is
// returned unchanged with no changes.
func reconcilePlist(raw []byte, adopt bool) ([]byte, []string, error) {
	post, err := plistPosture(raw)
	if err != nil {
		return nil, nil, err
	}
	if !post.FathomWritten && !adopt {
		return nil, nil, errPlistNeedsAdopt
	}
	if len(post.Drift) == 0 {
		return raw, nil, nil
	}
	p, err := parsePlist(raw)
	if err != nil {
		return nil, nil, err
	}
	indent := plistIndent(raw, p)
	var edits []plistEdit
	var changes []string
	var inserts strings.Builder
	for _, k := range plistTenancyKeys {
		e, ok := p.entry(k.Key)
		switch {
		case !ok:
			fmt.Fprintf(&inserts, "%s<key>%s</key>%s\n", indent, k.Key, k.XML)
			changes = append(changes, fmt.Sprintf("%s: absent → %s", k.Key, k.Want))
		case e.render() != k.Want:
			edits = append(edits, plistEdit{e.Start, e.End, k.XML})
			changes = append(changes, fmt.Sprintf("%s: %s → %s", k.Key, e.render(), k.Want))
		}
	}
	if args, ok := p.entry("ProgramArguments"); ok && args.Kind == "array" && !plistArgsHaveManaged(args) {
		at := int64(-1)
		for _, c := range args.Children {
			if c.Kind == "string" && c.Text == "serve-http" {
				at = c.End
				break
			}
		}
		if at < 0 {
			return nil, nil, errors.New("ProgramArguments carries no serve-http — not a fathom daemon plist")
		}
		// Keep the operator's layout: the new argument copies the
		// whitespace that precedes serve-http's own element.
		ws := plistLeadingSpace(raw, args, "serve-http")
		edits = append(edits, plistEdit{at, at, ws + "<string>--managed</string>"})
		changes = append(changes, "ProgramArguments: + --managed after serve-http")
	}
	if loc := plistMarkerRe.FindIndex(raw); loc != nil {
		edits = append(edits, plistEdit{int64(loc[0]), int64(loc[1]), plistSchemaMarker})
	} else {
		edits = append(edits, plistEdit{p.dictOpen, p.dictOpen, "\n" + indent + plistSchemaMarker})
	}
	changes = append(changes, fmt.Sprintf("schema marker: %d → %d", post.Schema, plistSchemaCurrent))
	if inserts.Len() > 0 {
		edits = append(edits, plistEdit{p.dictEnd, p.dictEnd, plistInsertPrefix(raw, p.dictEnd) + inserts.String()})
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), raw...)
	for _, e := range edits {
		out = append(out[:e.start:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	sort.Strings(changes)
	return out, changes, nil
}

// plistIndent is the whitespace before the first top-level key ("  " when
// it cannot be told).
func plistIndent(raw []byte, p *plistDoc) string {
	if len(p.keyStart) == 0 {
		return "  "
	}
	i := p.keyStart[0]
	j := i
	for j > 0 && (raw[j-1] == ' ' || raw[j-1] == '\t') {
		j--
	}
	if j == i {
		return "  "
	}
	return string(raw[j:i])
}

// plistInsertPrefix returns "\n" when the text before the top </dict> does
// not already end a line, so inserted keys start on their own line.
func plistInsertPrefix(raw []byte, at int64) string {
	j := at
	for j > 0 && (raw[j-1] == ' ' || raw[j-1] == '\t') {
		j--
	}
	if j > 0 && raw[j-1] == '\n' {
		return ""
	}
	return "\n"
}

// plistLeadingSpace returns the whitespace (with its newline) before the
// array element whose text is want.
func plistLeadingSpace(raw []byte, arr *plistNode, want string) string {
	for _, c := range arr.Children {
		if c.Text != want {
			continue
		}
		j := c.Start
		for j > 0 && (raw[j-1] == ' ' || raw[j-1] == '\t' || raw[j-1] == '\n' || raw[j-1] == '\r') {
			j--
		}
		if j < c.Start {
			return string(raw[j:c.Start])
		}
	}
	return ""
}
