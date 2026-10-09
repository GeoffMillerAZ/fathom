// Package stackframe reads stack frames out of a pasted trace or an error
// payload (TL-1, docs/plans/telemetry-links.md; ADR-062; design
// docs/design/telemetry-links.md section 3). It is the one parser localize
// (internal/mcp) seeds from and the prompt hook (cmd/fathom) detects with,
// so the two can no longer drift apart.
//
// Frame tuples only (ADR-058 decision 3): a frame is a path, a line and the
// name of the function at that place, never message text. A payload is
// parsed in the call and discarded; the payload shapes are data
// (shapes.json), and a field the registry does not name (a message, a
// breadcrumb, the user, metaData) is never read.
package stackframe

import (
	"regexp"
	"strconv"
	"strings"
)

// Frame is one stack frame tuple. Depth 0 is the throw site (the innermost
// frame of the root cause); deeper frames are its callers, in order.
type Frame struct {
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	// Func is the function enclosing File:Line as the trace names it
	// (qualified as written; ShortName gives the bare name).
	Func string `json:"func,omitempty"`
	// PathHint is a longer repository-relative suffix the frame implies:
	// a JVM frame's package names its directory, so
	// `com.acme.billing.Util.round(Util.java:12)` hints
	// com/acme/billing/Util.java, which a duplicate basename elsewhere does
	// not match.
	PathHint string `json:"path_hint,omitempty"`
	Depth    int    `json:"depth"`
}

// Result is one extraction: the frames in depth order, whether the text
// was a payload of a registered shape, and which shapes matched.
type Result struct {
	Frames  []Frame  `json:"frames"`
	Payload bool     `json:"payload"`
	Shapes  []string `json:"shapes,omitempty"`
}

// Extract reads text as a payload of a registered shape when it is one
// (ExtractPayload), and as a raw trace otherwise (Parse).
func Extract(text string) Result {
	if r, ok := ExtractPayload(text); ok {
		return r
	}
	return Result{Frames: Parse(text)}
}

// ─── raw traces ─────────────────────────────────────────────────────────────

var (
	// Python: `File "path", line N, in fn` (oldest call first).
	pyFrameRE = regexp.MustCompile(`^\s*File "([^"]+)", line (\d+)(?:, in (\S+))?`)
	// PHP: `#N path(line): callee(args)`; `#N {main}` closes the trace.
	phpFrameRE = regexp.MustCompile(`^\s*#(\d+) (\S.*?)\((\d+)\): (.+)$`)
	phpMainRE  = regexp.MustCompile(`^\s*#(\d+) \{main\}\s*$`)
	// PHP header: "... in /path/file.php:42" ending the message line, and
	// the uncaught footer "thrown in /path/file.php on line 42".
	phpHeaderRE = regexp.MustCompile(`\bin (\S+\.php):(\d+)\s*$`)
	phpThrownRE = regexp.MustCompile(`^\s*thrown in (\S+\.php) on line (\d+)\s*$`)
	// PHP's chained exceptions: "Next Outer: msg in file:line" follows the
	// previous exception's own trace.
	phpNextRE = regexp.MustCompile(`^Next [\w\\]+(?::|$)`)
	// JVM: `at [module/]pkg.Class.method(File.java:12)`; Lambda's Java
	// runtime writes the same frame without "at ".
	jvmFrameRE = regexp.MustCompile(`^\s*(?:at\s+)?(?:[\w.@-]*/+)?([A-Za-z_$][\w$]*(?:\.[\w$<>-]+)+)\(([\w$.-]+\.(?:java|kt|kts|scala|groovy|clj)):(\d+)\)\s*$`)
	// V8: `at fn (path:line:col)`, `at path:line:col`.
	v8FrameRE = regexp.MustCompile(`^\s*at (?:(.+?) \()?(\S+?):(\d+):(\d+)\)?\s*$`)
	// Ruby: `path.rb:12:in 'meth'` (with or without "from ").
	rubyFrameRE = regexp.MustCompile("^\\s*(?:from )?(\\S+\\.rb):(\\d+):in [`'](?:[^'`]*[#.])?([^'`]+)'")
	// Go: a function line followed by `\tpath.go:N +0x1d`.
	goFuncRE = regexp.MustCompile(`^([\w./*()-]+)\((?:.*)\)$`)
	// Generic `path.ext:line` anywhere (Go, Rust, a log line).
	genericRE = regexp.MustCompile(`([\w./-]+\.[A-Za-z0-9]+):(\d+)`)
	// Python's chained-exception separators: the cause prints first.
	pyChainRE = regexp.MustCompile(`^(The above exception was the direct cause of the following exception:|During handling of the above exception, another exception occurred:)\s*$`)
)

// rawFrame is a frame before depth is assigned.
type rawFrame struct {
	f       Frame
	section int
}

// Parse reads a raw trace (Python, PHP, JVM, V8, Ruby, Go, or any
// `path.ext:line`) into frame tuples in depth order. Python prints the
// oldest call first and its root cause first; the JVM prints the root
// cause last ("Caused by:"); PHP names, on frame #N, the place #N+1's
// function called #N's, so its tuples are realigned to the function that
// encloses each place.
func Parse(text string) []Frame {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var (
		py, jvm    []rawFrame
		other      []rawFrame
		pySection  int
		jvmSection int
		phpHeader  *Frame
		phpCallees = map[int]string{}
		phpPlaces  = map[int]Frame{}
		phpMax     = -1
		phpDone    bool // a "Next X:" line ends PHP's root-cause section
	)
	for i, ln := range lines {
		if phpMax >= 0 && phpNextRE.MatchString(ln) {
			phpDone = true
		}
		if phpDone && (phpFrameRE.MatchString(ln) || phpMainRE.MatchString(ln) || phpThrownRE.MatchString(ln) || phpHeaderRE.MatchString(ln)) {
			continue // PHP prints the previous (root-cause) exception first
		}
		switch {
		case pyChainRE.MatchString(strings.TrimSpace(ln)):
			pySection++
			continue
		case strings.HasPrefix(strings.TrimSpace(ln), "Caused by:"):
			jvmSection++
			continue
		}
		if m := pyFrameRE.FindStringSubmatch(ln); m != nil {
			py = append(py, rawFrame{Frame{File: m[1], Line: atoi(m[2]), Func: m[3]}, pySection})
			continue
		}
		if m := phpMainRE.FindStringSubmatch(ln); m != nil {
			n := atoi(m[1])
			phpCallees[n] = ""
			if n > phpMax {
				phpMax = n
			}
			continue
		}
		if m := phpFrameRE.FindStringSubmatch(ln); m != nil {
			n := atoi(m[1])
			phpPlaces[n] = Frame{File: m[2], Line: atoi(m[3])}
			phpCallees[n] = phpCallee(m[4])
			if n > phpMax {
				phpMax = n
			}
			continue
		}
		if m := phpThrownRE.FindStringSubmatch(ln); m != nil {
			if phpHeader == nil {
				phpHeader = &Frame{File: m[1], Line: atoi(m[2])}
			}
			continue
		}
		if m := jvmFrameRE.FindStringSubmatch(ln); m != nil {
			jvm = append(jvm, rawFrame{jvmFrame(m[1], m[2], atoi(m[3])), jvmSection})
			continue
		}
		if m := v8FrameRE.FindStringSubmatch(ln); m != nil {
			if f, ok := v8Frame(m[1], m[2], atoi(m[3])); ok {
				other = append(other, rawFrame{f, jvmSection})
			}
			continue
		}
		if m := rubyFrameRE.FindStringSubmatch(ln); m != nil {
			other = append(other, rawFrame{Frame{File: m[1], Line: atoi(m[2]), Func: m[3]}, jvmSection})
			continue
		}
		if phpHeader == nil && phpMax < 0 {
			if m := phpHeaderRE.FindStringSubmatch(ln); m != nil {
				phpHeader = &Frame{File: m[1], Line: atoi(m[2])}
				continue
			}
		}
		for _, m := range genericRE.FindAllStringSubmatch(ln, -1) {
			f := Frame{File: m[1], Line: atoi(m[2])}
			// Go: the line above a location names its function.
			if i > 0 && strings.HasPrefix(ln, "\t") {
				if fm := goFuncRE.FindStringSubmatch(strings.TrimSpace(lines[i-1])); fm != nil {
					f.Func = fm[1]
				}
			}
			other = append(other, rawFrame{f, jvmSection})
		}
	}

	var out []Frame
	// Python: sections in print order (the cause first), each newest-first.
	for s := 0; s <= pySection; s++ {
		for i := len(py) - 1; i >= 0; i-- {
			if py[i].section == s {
				out = append(out, py[i].f)
			}
		}
	}
	// JVM: the last "Caused by:" section is the root cause.
	for s := jvmSection; s >= 0; s-- {
		for _, r := range jvm {
			if r.section == s {
				out = append(out, r.f)
			}
		}
	}
	out = append(out, phpFrames(phpHeader, phpPlaces, phpCallees, phpMax)...)
	// Any other grammar under "Caused by:" sections (a Java-style
	// formatter in another runtime): the last section first, too.
	for s := jvmSection; s >= 0; s-- {
		for _, r := range other {
			if r.section == s {
				out = append(out, r.f)
			}
		}
	}
	return finish(out)
}

// phpFrames realigns PHP's frames: frame #N's place sits inside #N+1's
// callee, and the header's place (the throw) inside #0's callee.
func phpFrames(header *Frame, places map[int]Frame, callees map[int]string, max int) []Frame {
	if max < 0 {
		if header != nil {
			return []Frame{*header}
		}
		return nil
	}
	var out []Frame
	if header != nil {
		h := *header
		h.Func = callees[0]
		out = append(out, h)
	} else if c := callees[0]; c != "" {
		out = append(out, Frame{Func: c})
	}
	for n := 0; n <= max; n++ {
		p, ok := places[n]
		if !ok {
			continue
		}
		p.Func = callees[n+1]
		out = append(out, p)
	}
	return out
}

// phpCallee is the callee of a PHP frame with its arguments cut off: the
// arguments are values, never read.
func phpCallee(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// jvmFrame builds a JVM frame and its package path hint.
func jvmFrame(qualified, file string, line int) Frame {
	f := Frame{File: file, Line: line, Func: qualified}
	parts := strings.Split(qualified, ".")
	if len(parts) >= 3 {
		pkg := parts[:len(parts)-2] // drop the class and the method
		f.PathHint = strings.Join(pkg, "/") + "/" + file
	}
	return f
}

// v8Frame builds a V8 frame; a runtime-internal location (`node:`,
// `<anonymous>`, `native`) is no file in a repository.
func v8Frame(fn, loc string, line int) (Frame, bool) {
	if strings.HasPrefix(loc, "node:") || strings.HasPrefix(loc, "<") || loc == "native" || strings.HasPrefix(loc, "internal/") {
		return Frame{}, false
	}
	fn = strings.TrimSpace(fn)
	if i := strings.Index(fn, " [as "); i >= 0 {
		fn = fn[:i]
	}
	return Frame{File: normalizePath(loc), Line: line, Func: fn}, true
}

// normalizePath strips a URL or bundler scheme so a frame's path can be
// matched by suffix against the repository.
func normalizePath(p string) string {
	for _, pre := range []string{"file://", "webpack:///", "webpack-internal:///"} {
		if strings.HasPrefix(p, pre) {
			p = strings.TrimPrefix(p, pre)
			if pre == "file://" {
				return p
			}
		}
	}
	if i := strings.Index(p, "://"); i > 0 {
		rest := p[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			p = rest[j+1:]
		}
	}
	return strings.TrimPrefix(p, "./")
}

// finish drops repeated (file, line, func) frames and empty ones, keeping
// the first, and numbers depth in order. Two functions at one place (a JVM
// lambda and the method whose line called it) stay two frames: each names a
// function.
func finish(in []Frame) []Frame {
	seen := map[string]bool{}
	var out []Frame
	for _, f := range in {
		if f.File == "" && f.Func == "" {
			continue
		}
		if f.File != "" && f.Line <= 0 {
			continue
		}
		k := f.File + "\x00" + strconv.Itoa(f.Line) + "\x00" + f.Func
		if seen[k] {
			continue
		}
		seen[k] = true
		f.Depth = len(out)
		out = append(out, f)
	}
	return out
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// ShortName is a frame function's bare name, the one an index lookup
// takes: the last segment after `.`, `->`, `::`, `\`, `#` or `/`, with a
// V8 `new `/`async ` prefix dropped. A name that is no identifier
// (`<module>`, `Object.<anonymous>`, `{main}`) is "".
func ShortName(fn string) string {
	fn = strings.TrimSpace(fn)
	for _, pre := range []string{"new ", "async "} {
		fn = strings.TrimPrefix(fn, pre)
	}
	if i := strings.IndexByte(fn, ' '); i >= 0 {
		fn = fn[:i]
	}
	cut := 0
	for _, sep := range []string{".", "->", "::", "\\", "#", "/"} {
		if i := strings.LastIndex(fn, sep); i >= 0 && i+len(sep) > cut {
			cut = i + len(sep)
		}
	}
	fn = fn[cut:]
	if fn == "" || !isIdent(fn) {
		return ""
	}
	return fn
}

func isIdent(s string) bool {
	for i, r := range s {
		ok := r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') || r > 127
		if !ok {
			return false
		}
	}
	return true
}

// FrameLineRE matches one line that is wholly a stack frame, in any raw
// grammar Parse reads: the prompt hook's issue-shape test counts these.
var FrameLineRE = regexp.MustCompile(`(?m)^\s*(?:` +
	`(?:at\s+)?(?:[\w.@-]*/+)?[\w$]+(?:\.[\w$<>-]+)+\([\w$.-]+\.(?:java|kt|kts|scala|groovy|clj):\d+\)` + // JVM
	`|at\s+[\w.$]+\(.*\)` + // JVM-like (Native Method, Unknown Source)
	`|at\s+(?:async\s+)?(?:new\s+)?[^\s()]+(?:\s+\[as\s+[^\]]+\])?\s+\(\S+:\d+:\d+\)` + // V8 at fn (path:l:c)
	`|at\s+\S+:\d+:\d+` + // V8 at path:l:c
	`|#\d+\s+\S.*\(\d+\):\s+\S.*` + // PHP #N path(line): fn
	`|#\d+\s+\{main\}` + // PHP #N {main}
	`|File "[^"]+", line \d+(?:, in \S+)?` + // Python
	`|(?:from\s+)?\S+\.rb:\d+:in\s+\S.*` + // Ruby
	`|[\w./-]+\.[A-Za-z0-9]+:\d+(?::\d+)?\s*(?:\+0x[0-9a-f]+)?` + // generic, Go
	`)\s*$`)

// CountFrameLines counts lines of text that are wholly a stack frame,
// stopping at max (max <= 0 counts all).
func CountFrameLines(text string, max int) int {
	return len(FrameLineRE.FindAllStringIndex(text, max))
}
