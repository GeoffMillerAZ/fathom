package routes

// registry.go — the route seam (LB-F321, wave c2; docs/design/system-graph.md
// §5 "Framework routes (wave c2)"). Every framework reader registers itself
// from an init() in its own file; ExtractFile runs the Go AST reader plus
// every reader registered for the file's language. A reader returns the
// routes it reads and the mounts it sees: a mount says "every route of the
// file(s) Target names is served under Prefix", which no single file can
// resolve, so Compose (compose.go) applies mounts across a service's files.

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Mount is one file mounting another's routes under a path prefix
// (Express `app.use('/api', require('./api'))`, Django
// `path('polls/', include('polls.urls'))`). Target is one of the three
// forms Compose resolves: "./x" or "../x" relative to File's directory,
// "/x" repo-relative, or a dotted module "a.b.urls". A path form ending in
// "/" names a directory's direct files, in "/**" every file under it (a
// recursive import such as Symfony's), and in "/**?dirs" every file under
// it with its folders joined to Prefix (@fastify/autoload). Every form is
// stored as is in the MOUNT service_routes row (serviceref.ExtractFile).
// File and Line locate the mount itself.
type Mount struct {
	Prefix string
	Target string
	File   string
	Line   int
}

// Result is one file's reading: its routes, at the paths the file itself
// writes, and its mounts.
type Result struct {
	Routes []Route
	Mounts []Mount
}

// Reader is one framework's route reader. Framework names it (unique;
// readers run in Framework order); Langs are the connector.LanguageForPath
// tokens it reads (go, java, php, py, js, ts, ...); Read reads one file and
// must be pure and deterministic.
type Reader struct {
	Framework string
	Langs     []string
	Read      func(path string, src []byte) Result
}

var (
	readersMu sync.RWMutex
	readers   []Reader // sorted by Framework
)

// Register adds a framework reader; it is called from an init() in the
// reader's own file. It panics on a duplicate Framework or an incomplete
// Reader, a programming error caught by the first test run.
func Register(r Reader) {
	if r.Framework == "" || len(r.Langs) == 0 || r.Read == nil {
		panic(fmt.Sprintf("routes: Register: incomplete reader %q", r.Framework))
	}
	langs := make([]string, 0, len(r.Langs))
	for _, l := range r.Langs {
		langs = append(langs, canonLang(l))
	}
	r.Langs = langs
	readersMu.Lock()
	defer readersMu.Unlock()
	for _, have := range readers {
		if have.Framework == r.Framework {
			panic(fmt.Sprintf("routes: Register: duplicate framework %q", r.Framework))
		}
	}
	readers = append(readers, r)
	sort.Slice(readers, func(i, j int) bool { return readers[i].Framework < readers[j].Framework })
}

// canonLang folds the long spellings Extract has always accepted onto
// connector.LanguageForPath's tokens.
func canonLang(lang string) string {
	switch l := strings.ToLower(lang); l {
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	case "python":
		return "py"
	default:
		return l
	}
}

// readersFor returns the registered readers of lang, in Framework order.
func readersFor(lang string) []Reader {
	readersMu.RLock()
	defer readersMu.RUnlock()
	var out []Reader
	for _, r := range readers {
		for _, l := range r.Langs {
			if l == lang {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// HasReader reports whether any reader reads lang: the Go AST reader, or a
// registered framework reader.
func HasReader(lang string) bool {
	l := canonLang(lang)
	return l == "go" || len(readersFor(l)) > 0
}

// ExtractFile reads one file with the Go AST reader (lang "go") plus every
// reader registered for lang. Routes are sorted (sortRoutes) and
// deduplicated on (Method, Path, Handler, Line); mounts are sorted. A route
// or mount a reader left without a File is filed under path.
func ExtractFile(lang, path string, src []byte) Result {
	l := canonLang(lang)
	var res Result
	if l == "go" {
		res.Routes = extractGo(path, src)
	}
	for _, r := range readersFor(l) {
		x := r.Read(path, src)
		res.Routes = append(res.Routes, x.Routes...)
		res.Mounts = append(res.Mounts, x.Mounts...)
	}
	for i := range res.Routes {
		if res.Routes[i].File == "" {
			res.Routes[i].File = path
		}
	}
	for i := range res.Mounts {
		if res.Mounts[i].File == "" {
			res.Mounts[i].File = path
		}
	}
	res.Routes = dedupeRoutes(res.Routes)
	sortMounts(res.Mounts)
	return res
}

// Extract scans src for HTTP route registrations and returns them sorted:
// ExtractFile's routes, without the mounts (a caller that composes across
// files uses ExtractFile and Compose).
func Extract(lang, path string, src []byte) []Route {
	return ExtractFile(lang, path, src).Routes
}

// dedupeRoutes sorts rs and drops a route equal to an earlier one on
// (Method, Path, Handler, Line); nil stays nil.
func dedupeRoutes(rs []Route) []Route {
	if len(rs) == 0 {
		return rs
	}
	sortRoutes(rs)
	type key struct {
		method, path, handler string
		line                  int
	}
	seen := make(map[key]bool, len(rs))
	out := rs[:0]
	for _, r := range rs {
		k := key{r.Method, r.Path, r.Handler, r.Line}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

// sortMounts orders mounts by (File, Line, Prefix, Target).
func sortMounts(ms []Mount) {
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Prefix != b.Prefix {
			return a.Prefix < b.Prefix
		}
		return a.Target < b.Target
	})
}
