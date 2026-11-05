package langroute

// pack.go -- the per-Router pack layer (ADR-092, design §6). The built-in
// tables are process-wide and immutable, so a pack never edits them: each
// Router holds its own layer of classifier rules, tried after the specific
// built-in dispatches and before an extension's generic fallback. A pack
// dialect claims by basename, glob, sniff or an extension no built-in owns; it
// cannot take a file a built-in reads by extension, nor one a built-in's
// sniff, glob or shebang reads (the sniffed built-ins are asked first).

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langclass"
)

// packToolPrefix marks the ToolKind a pack dialect's file carries: the prefix,
// then the dialect id. It never reaches a store: the product loads no pack.
const packToolPrefix = "pack:"

// PackDialectOf is the id of the pack dialect behind tk, or "" when tk is not
// a pack's.
func PackDialectOf(tk ToolKind) string {
	if s := string(tk); strings.HasPrefix(s, packToolPrefix) {
		return strings.TrimPrefix(s, packToolPrefix)
	}
	return ""
}

// genericExts are the extensions the router reads with a generic extractor
// (JSON, TOML, YAML): a pack may claim a file of one by basename, glob or
// sniff, never the extension itself, which would take every file.
var genericExts = map[string]bool{".json": true, ".toml": true, ".yaml": true, ".yml": true}

// packLayer is one Router's pack: classifier rules in id order and the
// extractor of each dialect.
type packLayer struct {
	rules []langclass.DialectRule
	ex    map[string]extractor.Extractor
}

// AttachPack adds p as this Router's pack layer. ex holds the extractor of
// every pack dialect (extractor.Pack.Build's, and the ones Go extractors
// supply). A dialect that claims an extension or basename the router owns
// outright refuses the whole pack with extractor.PackRoute, and nothing is
// attached. A Router takes one pack, attached before it resolves a file.
func (r *Router) AttachPack(p *extractor.Pack, ex map[string]extractor.Extractor) error {
	if r.pack != nil {
		return fmt.Errorf("langroute: a pack is already attached")
	}
	ds := p.Dialects()
	pl := &packLayer{ex: map[string]extractor.Extractor{}}
	for _, d := range ds {
		e, ok := ex[d.ID]
		if !ok || e == nil {
			return &extractor.PackError{Reason: extractor.PackManifest, Dialect: d.ID, Err: fmt.Errorf("no extractor for the dialect")}
		}
		if err := r.checkPackRoute(d); err != nil {
			return err
		}
		pl.ex[d.ID] = e
		rule := langclass.DialectRule{ID: d.ID, Basenames: d.Match.Basenames, Globs: d.Match.Globs}
		rule.Extensions = append(rule.Extensions, d.Match.Extensions...)
		for _, v := range d.Variants {
			rule.Extensions = append(rule.Extensions, v.Extensions...)
		}
		if s := d.Match.Sniff; s != nil {
			rule.Sniff = &langclass.DialectSniffRule{Extensions: s.Extensions, Pattern: s.Pattern, Priority: s.Priority, ScanBytes: s.ScanBytes}
		}
		pl.rules = append(pl.rules, rule) // p.Dialects() is in id order
	}
	r.pack = pl
	return nil
}

// owned reports whether the built-in router reads files of the extension ext
// (lowercase, with its dot) with an extractor of its own.
func (r *Router) owned(ext string) bool {
	if genericExts[ext] {
		return false
	}
	return r.exts[ext] != nil || r.sniffExt[ext]
}

// checkPackRoute refuses a dialect that claims what the router owns outright.
func (r *Router) checkPackRoute(d extractor.Dialect) error {
	refuse := func(format string, args ...any) error {
		return &extractor.PackError{Reason: extractor.PackRoute, Dialect: d.ID, Err: fmt.Errorf(format, args...)}
	}
	exts := append([]string(nil), d.Match.Extensions...)
	for _, v := range d.Variants {
		exts = append(exts, v.Extensions...)
	}
	for _, x := range exts {
		x = strings.ToLower(x)
		if genericExts[x] {
			return refuse("extension %s is read by the built-in %s reader; claim files by basename, glob or sniff", x, strings.TrimPrefix(x, "."))
		}
		if r.owned(x) {
			return refuse("extension %s is owned by a built-in dialect", x)
		}
	}
	if s := d.Match.Sniff; s != nil {
		for _, x := range s.Extensions {
			if r.owned(strings.ToLower(x)) {
				return refuse("sniffed extension %s is owned by a built-in dialect", x)
			}
		}
	}
	for _, b := range d.Match.Basenames {
		if r.exts[b] != nil {
			return refuse("basename %s is owned by a built-in dialect", b)
		}
	}
	for _, g := range d.Match.Globs {
		x := strings.ToLower(filepath.Ext(g))
		if x != "" && !strings.ContainsAny(x, "*?[") && r.owned(x) {
			return refuse("glob %s ends in extension %s, owned by a built-in dialect", g, x)
		}
	}
	return nil
}

// packDispatch is the pack layer's claim on relPath: the extractor of the
// first pack dialect (in id order) whose rules settle the file. A file two
// pack dialects could both read and no sniff settles is claimed by neither.
func (r *Router) packDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	if r.pack == nil {
		return nil, "", false
	}
	v := langclass.ClassifyDialect(r.pack.rules, relPath, content)
	if v.Dialect == "" {
		return nil, "", false
	}
	e, ok := r.pack.ex[v.Dialect]
	if !ok {
		return nil, "", false
	}
	return e, ToolKind(packToolPrefix + v.Dialect), true
}

// closePack releases the pack's pooled grammar modules.
func (r *Router) closePack() {
	if r.pack == nil {
		return
	}
	for _, e := range r.pack.ex {
		if ts, ok := e.(*extractor.TSExtractor); ok {
			ts.Close()
		}
	}
}
