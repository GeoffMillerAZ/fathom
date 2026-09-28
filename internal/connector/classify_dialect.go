package connector

import (
	"bytes"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// DialectRule is one dialect manifest's match block (docs/design/
// language-breadth.md §2.2 "match", §5), as the connector's classification
// consults it. fusion builds the rules from extractor.LoadDialects; the
// connector does not import the extractor.
type DialectRule struct {
	ID         string
	Extensions []string
	Basenames  []string
	Globs      []string
	Shebangs   []string
	Sniff      *DialectSniffRule
}

// DialectSniffRule is a manifest's match.sniff: a regex over the first
// 1 KiB, evaluated for the listed extensions in ascending Priority across
// every dialect claiming the extension.
type DialectSniffRule struct {
	Extensions []string
	Pattern    string
	Priority   int
}

// How a DialectVerdict was reached (§5's order).
const (
	ClassifiedByBasename  = "basename"
	ClassifiedByGlob      = "glob"
	ClassifiedByExtension = "extension"
	ClassifiedByShebang   = "shebang"
	ClassifiedBySniff     = "sniff"
	// ClassifiedAmbiguous: the extension is claimed by more than one
	// dialect and no sniff settled it — left unclassified, and counted.
	ClassifiedAmbiguous = "ambiguous"
	// ClassifiedNone: no dialect claims the file.
	ClassifiedNone = ""
)

// DialectVerdict is ClassifyDialect's answer: the dialect id ("" when none)
// and how it was reached.
type DialectVerdict struct {
	Dialect string
	By      string
}

// sniffScanBytes is how much of a file a sniff sees (§5).
const sniffScanBytes = 1 << 10

// ClassifyDialect classifies path (repo-relative, slash-separated) against
// the manifests' match rules in §5's order (LB-F9): exact basename, path
// glob, extension (case-insensitive), shebang for a file no extension
// settles, then — only for an extension more than one dialect claims — the
// claimants' sniffs over the first 1 KiB of head, lowest priority first
// (ties by id). An ambiguous extension no sniff settles is By
// ClassifiedAmbiguous with no dialect: never a guess. head may be nil when
// the caller has no content (the sniff and shebang steps then find
// nothing).
func ClassifyDialect(rules []DialectRule, relPath string, head []byte) DialectVerdict {
	relPath = strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(relPath, "\\", "/")), "/")
	base := path.Base(relPath)
	for _, r := range rules {
		for _, b := range r.Basenames {
			if b == base {
				return DialectVerdict{r.ID, ClassifiedByBasename}
			}
		}
	}
	for _, r := range rules {
		for _, g := range r.Globs {
			if globMatch(g, relPath) {
				return DialectVerdict{r.ID, ClassifiedByGlob}
			}
		}
	}
	ext := strings.ToLower(path.Ext(base))
	if ext != "" {
		var plain []string
		var sniffers []DialectRule
		for _, r := range rules {
			if containsFold(r.Extensions, ext) {
				plain = append(plain, r.ID)
			}
			if r.Sniff != nil && containsFold(r.Sniff.Extensions, ext) {
				sniffers = append(sniffers, r)
			}
		}
		claimants := map[string]bool{}
		for _, id := range plain {
			claimants[id] = true
		}
		for _, r := range sniffers {
			claimants[r.ID] = true
		}
		switch {
		case len(claimants) == 1 && len(plain) == 1:
			return DialectVerdict{plain[0], ClassifiedByExtension}
		case len(claimants) > 0:
			if id, ok := sniff(sniffers, head); ok {
				return DialectVerdict{id, ClassifiedBySniff}
			}
			return DialectVerdict{"", ClassifiedAmbiguous}
		}
	}
	if interp := shebangInterpreter(head); interp != "" {
		for _, r := range rules {
			for _, s := range r.Shebangs {
				if interpreterIs(interp, s) {
					return DialectVerdict{r.ID, ClassifiedByShebang}
				}
			}
		}
	}
	return DialectVerdict{"", ClassifiedNone}
}

func containsFold(list []string, ext string) bool {
	for _, e := range list {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

// sniff evaluates the claimants' patterns in (priority, id) order over the
// first 1 KiB.
func sniff(sniffers []DialectRule, head []byte) (string, bool) {
	if len(head) > sniffScanBytes {
		head = head[:sniffScanBytes]
	}
	sort.SliceStable(sniffers, func(i, j int) bool {
		if sniffers[i].Sniff.Priority != sniffers[j].Sniff.Priority {
			return sniffers[i].Sniff.Priority < sniffers[j].Sniff.Priority
		}
		return sniffers[i].ID < sniffers[j].ID
	})
	for _, r := range sniffers {
		if re := sniffRegexp(r.Sniff.Pattern); re != nil && re.Match(head) {
			return r.ID, true
		}
	}
	return "", false
}

var sniffRes sync.Map // pattern -> *regexp.Regexp (nil when it does not compile)

// sniffRegexp compiles a sniff pattern once, multi-line (§5's patterns
// anchor lines with ^). A pattern that does not compile never matches;
// TestDialectManifests refuses one in a shipped manifest.
func sniffRegexp(p string) *regexp.Regexp {
	if v, ok := sniffRes.Load(p); ok {
		re, _ := v.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile("(?m)" + p)
	if err != nil {
		re = nil
	}
	sniffRes.Store(p, re)
	return re
}

// shebangInterpreter is the interpreter a `#!` first line names: the last
// path element of the program, or env's first non-flag argument.
func shebangInterpreter(head []byte) string {
	if !bytes.HasPrefix(head, []byte("#!")) {
		return ""
	}
	line := head[2:]
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(strings.TrimSpace(string(line)))
	if len(fields) == 0 {
		return ""
	}
	prog := path.Base(fields[0])
	if prog == "env" {
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") && !strings.Contains(f, "=") {
				return path.Base(f)
			}
		}
		return ""
	}
	return prog
}

// interpreterIs reports whether interp is name, optionally followed by a
// version ("python3", "perl5.36").
func interpreterIs(interp, name string) bool {
	if !strings.HasPrefix(interp, name) {
		return false
	}
	return strings.Trim(interp[len(name):], "0123456789.") == ""
}

// globMatch matches a slash path against a manifest glob: path.Match per
// segment, with a "**" segment matching zero or more segments.
func globMatch(pattern, p string) bool {
	return globSegs(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func globSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if globSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
