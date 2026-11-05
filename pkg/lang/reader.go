package lang

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/langclass"
	"github.com/geoffmilleraz/fathom/internal/langlink"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// ErrClosed is returned by Extract on a Reader that was closed.
var ErrClosed = errors.New("lang: reader is closed")

// DefaultMaxFileBytes is the largest file a Reader reads: the product's own
// limit, 2 MiB. A larger file is skipped as oversize. The value is a literal;
// TestPublicConstantValues holds it to the product's.
const DefaultMaxFileBytes = 2 * 1024 * 1024

// maxSiblings bounds the sibling list one call may carry.
const maxSiblings = 1 << 20

// serviceID is the placeholder the extractors key symbols under; it is
// stripped from every key the package returns.
const serviceID = "_"

// Limits are the bounds a Reader works under. A zero field keeps the default.
type Limits struct {
	// MaxFileBytes lowers the file size limit (DefaultMaxFileBytes).
	MaxFileBytes int
}

// Option configures New.
type Option func(*config) error

type config struct {
	maxFile  int
	packFS   fs.FS
	natives  []nativeSpec
	families []string
}

// WithLimits lowers the Reader's limits. A limit above the product's, or a
// negative one, is an error: no option raises a limit, and no option raises
// a grammar module's memory cap.
func WithLimits(l Limits) Option {
	return func(c *config) error {
		switch {
		case l.MaxFileBytes < 0:
			return fmt.Errorf("lang: MaxFileBytes %d is negative", l.MaxFileBytes)
		case l.MaxFileBytes > DefaultMaxFileBytes:
			return fmt.Errorf("lang: MaxFileBytes %d is above the limit %d; limits may only be lowered", l.MaxFileBytes, DefaultMaxFileBytes)
		case l.MaxFileBytes > 0:
			c.maxFile = l.MaxFileBytes
		}
		return nil
	}
}

// Reader reads files into facts. Build one with New and Close it. A Reader
// is safe for concurrent use.
type Reader struct {
	mu       sync.RWMutex
	closed   bool
	router   *langroute.Router
	maxFile  int
	levels   map[string]levelRow
	dialects []DialectInfo
	pack     *extractor.Pack      // the pack layer's dialects; nil for none
	natives  map[string]Extractor // Go extractors, by dialect id
}

// New builds a Reader. It returns the error a bad option or a bad built-in
// dialect gives, and never panics. The Reader links every built-in dialect
// except CUE and Timoni, which a consumer adds by importing pkg/lang/cue.
func New(opts ...Option) (r *Reader, err error) {
	defer func() {
		if p := recover(); p != nil {
			r, err = nil, fmt.Errorf("lang: building the reader: %v", p)
		}
	}()
	cfg := config{maxFile: DefaultMaxFileBytes}
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o(&cfg); err != nil {
			return nil, err
		}
	}
	levels, err := loadLevels()
	if err != nil {
		return nil, err
	}
	linked := map[string]bool{}
	var routerOpts []langroute.Option
	for _, name := range cfg.families {
		if linked[name] {
			continue
		}
		fam, ok := langlink.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("lang: family %q is not linked: import its package (for CUE, github.com/geoffmilleraz/fathom/pkg/lang/cue)", name)
		}
		linked[name] = true
		routerOpts = append(routerOpts, fam.Options()...)
	}
	dialects, err := dialectInfos(levels, linked)
	if err != nil {
		return nil, err
	}
	router, err := langroute.NewRouter(routerOpts...)
	if err != nil {
		return nil, fmt.Errorf("lang: %w", err)
	}
	rd := &Reader{router: router, maxFile: cfg.maxFile, levels: levels, dialects: dialects}
	if cfg.packFS != nil || len(cfg.natives) > 0 {
		if err := rd.attachPack(cfg); err != nil {
			router.Close()
			return nil, err
		}
	}
	return rd, nil
}

// attachPack validates the pack and the Go extractors together, compiles the
// pack's queries and attaches the layer to the Reader's router. It changes
// nothing on the Reader unless every dialect is accepted.
func (r *Reader) attachPack(cfg config) error {
	pack, err := extractor.NewPack(cfg.packFS)
	if err != nil {
		return publicPackError(err)
	}
	natives := map[string]Extractor{}
	for _, n := range cfg.natives {
		if !extractor.ValidNativeID(n.m.ID) {
			return &PackError{Reason: PackManifest, Dialect: n.m.ID, Detail: fmt.Sprintf("id %q is not [a-z0-9-]+", n.m.ID)}
		}
		if err := pack.AddNative(n.dialect()); err != nil {
			return publicPackError(err)
		}
		natives[n.m.ID] = n.e
	}
	built, err := pack.Build(context.Background())
	if err != nil {
		return publicPackError(err)
	}
	for id := range natives {
		built[id] = nativeStub{}
	}
	if err := r.router.AttachPack(pack, built); err != nil {
		for _, e := range built {
			if ts, ok := e.(*extractor.TSExtractor); ok {
				ts.Close()
			}
		}
		return publicPackError(err)
	}
	r.pack, r.natives = pack, natives
	r.dialects = append(r.dialects, packDialectInfos(r.levels, pack)...)
	sort.Slice(r.dialects, func(i, j int) bool { return r.dialects[i].ID < r.dialects[j].ID })
	return nil
}

// Close releases the Reader's pooled grammar modules. It is idempotent, and
// waits for extractions in flight.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	r.router.Close()
	return nil
}

// Dialects lists every dialect the Reader reads, sorted by id, with the
// level and grade that readings measured for it.
func (r *Reader) Dialects() []DialectInfo {
	out := make([]DialectInfo, len(r.dialects))
	for i, d := range r.dialects {
		d.Matches = append([]string{}, d.Matches...)
		out[i] = d
	}
	return out
}

// Extract reads one file. A file no dialect reads is not an error: the
// Result says why in Skipped. The error is for misuse (an empty path, a
// closed Reader, a done context). A file an extractor could not fully read
// returns what it did read, and Notes says what failed.
func (r *Reader) Extract(ctx context.Context, f File) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if f.Path == "" {
		return Result{}, errors.New("lang: File.Path is empty")
	}
	if len(f.Siblings) > maxSiblings {
		return Result{}, fmt.Errorf("lang: %d siblings is over the limit %d", len(f.Siblings), maxSiblings)
	}
	path := filepath.ToSlash(f.Path)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return Result{}, ErrClosed
	}
	res := newResult(path)

	if len(f.Src) > r.maxFile && r.maxFile < DefaultMaxFileBytes {
		if langroute.BinaryExcluded(path, f.Src) {
			res.Skipped = SkipBinary
		} else {
			res.Skipped = SkipOversize
		}
		return res, nil
	}
	ex, err := r.route(ctx, path, f)
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return Result{}, err // the context ended between two stages of this file
	}
	if ex.Skipped != "" {
		res.Skipped = skipReason(ex.Skipped, path)
		if res.Skipped == SkipNotLinked {
			res.Dialect = langroute.DialectOf(nil, "", path, nil)
		}
		res.addSiblingNote(path, f.Siblings)
		return res, nil
	}
	if ex.PackDialect != "" {
		if _, native := r.natives[ex.PackDialect]; native {
			return r.extractNative(ctx, ex.PackDialect, File{Path: path, Src: f.Src, Siblings: f.Siblings}, res)
		}
		res.Dialect = ex.PackDialect
		res.Level = LevelUnmeasured
	} else {
		res.Dialect = langroute.DialectOfContent(ex.Extractor, ex.ToolKind, path, f.Src, ex.Nodes)
		if res.Dialect == "" {
			res.Dialect = ex.RouteDialect // the route knows a file with nothing to emit (a sniffed .cfg)
		}
		res.Level, _ = levelOf(r.levels, res.Dialect)
	}
	if res.Dialect == "" && err != nil {
		res.Dialect = langroute.DialectOf(nil, "", path, nil) // an extractor that failed has no content to name it by
		res.Level, _ = levelOf(r.levels, res.Dialect)
	}
	if err != nil {
		res.Notes = append(res.Notes, errorNote(err))
		res.Incomplete = true
	}
	if ex.Incomplete != "" {
		res.Notes = append(res.Notes, "incomplete: "+ex.Incomplete)
		res.Incomplete = true
	}
	res.fill(ex, r.canonicalKind(ex.PackDialect != ""))
	res.withhold()
	if _, isGo := ex.Extractor.(*extractor.GoExtractor); isGo {
		res.refineGoTypes(f.Src)
	}
	res.resolveTargets()
	res.addSiblingNote(path, f.Siblings)
	res.sort()
	return res, nil
}

// route reads f through the router. A panic in a built-in extractor is the
// file's failure, not the caller's: it comes back as the error an extractor
// returns, which Extract reports as an Incomplete reading, so one file's input
// cannot end the batch it is in. (A stack overflow is fatal and cannot be
// caught, so the extractors that recurse refuse a nesting past maxNesting
// before they reach the parser.)
func (r *Reader) route(ctx context.Context, path string, f File) (ex langroute.Extraction, err error) {
	defer func() {
		if p := recover(); p != nil {
			ex, err = langroute.Extraction{}, fmt.Errorf("the reader failed on this input: %v", p)
		}
	}()
	index := func() *langroute.SiblingIndex { return siblingIndex(path, f.Siblings) }
	return r.router.ExtractFileDocSteps(ctx.Err, serviceID, index, path, f.Src)
}

// refineGoTypes sets the Canonical of each GoType symbol from its declaration:
// the extractor emits one kind for a struct, a defined type (`type N int`) and
// an alias, and the closed vocabulary tells them apart (struct, interface,
// type_alias, type).
func (res *Result) refineGoTypes(src []byte) {
	canon := extractor.GoTypeCanonicals(src)
	for i := range res.Symbols {
		if res.Symbols[i].Kind != "GoType" {
			continue
		}
		if c, ok := canon[res.Symbols[i].Name]; ok {
			res.Symbols[i].Canonical = c
		}
	}
}

// errorNote is the one-line Notes entry for an extractor's error: the engine
// budget it spent, when it spent one, else the error's first line. A wasm trap
// carries a multi-line stack, which is no use to a caller and never printed.
func errorNote(err error) string {
	var be *tsengine.BudgetError
	if errors.As(err, &be) {
		return "extractor error: " + be.Reason()
	}
	line := err.Error()
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) > 200 {
		line = line[:200]
	}
	return "extractor error: " + strings.TrimSpace(line)
}

func newResult(path string) Result {
	return Result{
		Language:   langclass.LanguageForPath(path),
		Symbols:    []Symbol{},
		References: []Reference{},
		Unresolved: []Unresolved{},
		Facts:      []Fact{},
		Notes:      []string{},
	}
}

// canonicalKind is the closed-vocabulary kind of a source kind. A pack's kind
// table applies only to the nodes a pack dialect read (packRead), so a pack
// that declares a built-in source kind cannot change what the built-ins read
// as; every other node is the built-in registry's.
func (r *Reader) canonicalKind(packRead bool) func(string) (string, bool) {
	return func(sk string) (string, bool) {
		if packRead && r.pack != nil {
			if c, ok := r.pack.CanonicalKind(sk); ok {
				return c, true
			}
		}
		return extractor.DoorCanonicalKind(sk)
	}
}

// skipReason maps the router's reason onto the closed set. A file the router
// has no extractor for is not-linked when the dialect that owns its path needs
// an extractor this Reader does not link (CUE), otherwise unclassified.
func skipReason(s, path string) string {
	switch s {
	case "binary":
		return SkipBinary
	case "oversize":
		return SkipOversize
	case "generated":
		return SkipGenerated
	case "ambiguous":
		return SkipAmbiguous
	case "no-extractor":
		if strings.EqualFold(filepath.Ext(path), ".cue") {
			return SkipNotLinked
		}
	}
	return SkipUnclassified
}

func symbolKey(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// edgeCall is the Edge of a call; edgeReference the Edge of a same-file
// reference an engine gave no relation for.
const (
	edgeCall      = "call"
	edgeReference = "reference"
)

// callsAreCalls reports whether the node.Calls of e's extractor are calls. It
// is a closed list: the Go and tree-sitter readers, the templ, Rego and
// notebook readers record calls. Every other engine's Calls field is a
// reference (a CMake target link, a Markdown link, a Kubernetes embed's
// entity, a workflow's needs), which reads "reference" unless the engine says
// more through DocResult.Refs. TestCallEdgesComeFromCallEngines holds the list.
func callsAreCalls(e extractor.Extractor) bool {
	switch e.(type) {
	case *extractor.GoExtractor, *extractor.TSExtractor, *extractor.TagsDocExtractor,
		*extractor.CHeaderExtractor, *extractor.TemplExtractor, *extractor.RegoExtractor,
		*extractor.NotebookExtractor:
		return true
	}
	return false
}

// fill converts the router's extraction into the public types.
func (res *Result) fill(ex langroute.Extraction, canonicalKind func(string) (string, bool)) {
	keyOf := map[string]string{} // an entity's name -> its symbol key
	for _, n := range ex.Nodes {
		key := symbolKey(n.Package, n.Name)
		if _, seen := keyOf[n.Name]; !seen {
			keyOf[n.Name] = key
		}
		canonical, _ := canonicalKind(string(n.Kind))
		res.Symbols = append(res.Symbols, Symbol{
			Key: key, Name: n.Name, Package: n.Package, Kind: string(n.Kind), Canonical: canonical,
			Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Tier: string(n.Tier),
		})
	}
	// A document dialect's same-file references carry the edge it declared and
	// the line they are written on (DocResult.Refs). Anything else in a node's
	// Calls is a call, or a plain reference when a document dialect gave no detail.
	type refDetail struct {
		edge string
		line int
	}
	detail := map[[2]string][]refDetail{}
	if ex.Doc != nil {
		for _, r := range ex.Doc.Refs {
			k := [2]string{r.From, r.To}
			detail[k] = append(detail[k], refDetail{r.Edge, r.Line})
		}
	}
	for _, n := range ex.Nodes {
		key := symbolKey(n.Package, n.Name)
		for _, c := range n.Calls {
			to := strings.TrimPrefix(c, serviceID+"::")
			if ds := detail[[2]string{n.Name, c}]; len(ds) > 0 {
				for _, d := range ds {
					res.References = append(res.References, Reference{From: key, To: to, Edge: d.edge, Line: d.line})
				}
				continue
			}
			edge := edgeReference
			if ex.Doc == nil && callsAreCalls(ex.Extractor) {
				edge = edgeCall
			}
			res.References = append(res.References, Reference{From: key, To: to, Edge: edge})
		}
	}
	d := ex.Doc
	if d == nil {
		return
	}
	from := func(name string) string {
		if k, ok := keyOf[name]; ok {
			return k
		}
		return name
	}
	for _, c := range d.CrossFile {
		if c.Target == "" {
			res.Unresolved = append(res.Unresolved, Unresolved{From: from(c.From), Edge: c.Edge, Raw: c.Raw, Reason: "no-target", Line: c.Line})
			continue
		}
		raw := c.Raw
		if raw == c.Target {
			raw = ""
		}
		res.References = append(res.References, Reference{From: from(c.From), To: c.Target, Edge: c.Edge, Raw: raw, Strategy: c.Strategy, Line: c.Line})
	}
	for _, u := range d.Unresolved {
		res.Unresolved = append(res.Unresolved, Unresolved{From: from(u.From), Edge: u.Edge, Raw: u.Raw, Reason: u.Reason, Line: u.Line})
	}
	for _, f := range d.Facts {
		res.Facts = append(res.Facts, Fact{Entity: from(f.Entity), Key: f.Key, Value: f.Value, Line: f.Line})
	}
}

// resolveTargets sets each same-file reference's ToKey: the Key of the one
// symbol its To equals by Key or by Name, empty when none or several do.
func (res *Result) resolveTargets() {
	byKey := map[string][]int{}
	byName := map[string][]int{}
	for i, s := range res.Symbols {
		byKey[s.Key] = append(byKey[s.Key], i)
		byName[s.Name] = append(byName[s.Name], i)
	}
	for i := range res.References {
		r := &res.References[i]
		r.ToKey = ""
		if r.Strategy != "" {
			continue // a cross-file reference names something outside this file
		}
		match := map[int]bool{}
		for _, j := range byKey[r.To] {
			match[j] = true
		}
		for _, j := range byName[r.To] {
			match[j] = true
		}
		if len(match) == 1 {
			for j := range match {
				r.ToKey = res.Symbols[j].Key
			}
		}
	}
}

// sort orders every slice, so equal inputs give equal output whatever order
// an extractor walked in.
func (res *Result) sort() {
	sort.SliceStable(res.Symbols, func(i, j int) bool {
		a, b := res.Symbols[i], res.Symbols[j]
		switch {
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Key != b.Key:
			return a.Key < b.Key
		case a.Kind != b.Kind:
			return a.Kind < b.Kind
		}
		return a.EndLine < b.EndLine
	})
	sort.SliceStable(res.References, func(i, j int) bool {
		a, b := res.References[i], res.References[j]
		switch {
		case a.From != b.From:
			return a.From < b.From
		case a.To != b.To:
			return a.To < b.To
		case a.Edge != b.Edge:
			return a.Edge < b.Edge
		case a.Line != b.Line:
			return a.Line < b.Line
		}
		return a.Raw < b.Raw
	})
	sort.SliceStable(res.Unresolved, func(i, j int) bool {
		a, b := res.Unresolved[i], res.Unresolved[j]
		switch {
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.From != b.From:
			return a.From < b.From
		case a.Edge != b.Edge:
			return a.Edge < b.Edge
		}
		return a.Raw < b.Raw
	})
	sort.SliceStable(res.Facts, func(i, j int) bool {
		a, b := res.Facts[i], res.Facts[j]
		switch {
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Entity != b.Entity:
			return a.Entity < b.Entity
		case a.Key != b.Key:
			return a.Key < b.Key
		}
		return a.Value < b.Value
	})
	sort.Strings(res.Notes)
}

// siblingIndex is the classification index for path and its siblings: the
// repository as far as the caller described it.
func siblingIndex(path string, siblings []string) *langroute.SiblingIndex {
	refs := make([]core.SourceRef, 0, len(siblings)+1)
	refs = append(refs, core.SourceRef{Path: path})
	for _, s := range siblings {
		refs = append(refs, core.SourceRef{Path: filepath.ToSlash(s)})
	}
	return langroute.BuildSiblingIndex(refs)
}

// siblingDirs are the directory names whose YAML or CUE files classify by
// what sits beside or above them.
var siblingDirs = map[string]bool{
	"templates": true, "tasks": true, "handlers": true, "defaults": true, "vars": true, "meta": true,
	"roles": true, "group_vars": true, "host_vars": true, "inventory": true,
}

// addSiblingNote discloses an empty Siblings when it could have changed the
// verdict: a YAML or CUE file the classification could not settle alone, or
// one under a directory that sibling facts key on.
func (res *Result) addSiblingNote(path string, siblings []string) {
	if len(siblings) > 0 {
		return
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".cue":
	default:
		return
	}
	dependent := res.Skipped == SkipUnclassified || res.Skipped == SkipNotLinked
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		dependent = dependent || siblingDirs[seg]
	}
	if strings.EqualFold(filepath.Ext(path), ".cue") {
		dependent = true
	}
	if dependent {
		res.Notes = append(res.Notes, "siblings-empty: this file's classification reads sibling paths (chart, role or module roots), and none were given")
	}
}

// dialectInfos lists the manifest dialects, the native Go extractor, the
// generic JSON and TOML readers and the templ reader.
func dialectInfos(levels map[string]levelRow, linked map[string]bool) ([]DialectInfo, error) {
	ds, err := extractor.LoadDialects()
	if err != nil {
		return nil, fmt.Errorf("lang: %w", err)
	}
	unlinked := map[string]bool{}
	for family, ids := range langlink.FamilyDialects {
		if !linked[family] {
			for _, id := range ids {
				unlinked[id] = true
			}
		}
	}
	out := make([]DialectInfo, 0, len(ds)+1)
	for _, d := range ds {
		if unlinked[d.ID] {
			continue // a family this Reader did not link reads nothing: Dialects lists what it can read
		}
		if !langroute.DoorReadable(d.ID) {
			continue // refused by its plan row, or read only by the product's router: Extract skips its files
		}
		set := map[string]bool{}
		for _, e := range d.Match.Extensions {
			set[strings.ToLower(e)] = true
		}
		for _, v := range d.Variants {
			for _, e := range v.Extensions {
				set[strings.ToLower(e)] = true
			}
		}
		for _, b := range d.Match.Basenames {
			set[b] = true
		}
		for _, g := range d.Match.Globs {
			set[g] = true
		}
		if d.Match.Sniff != nil {
			for _, e := range d.Match.Sniff.Extensions {
				set[strings.ToLower(e)] = true
			}
		}
		out = append(out, newDialectInfo(levels, d.ID, d.Kind, d.Engine, set))
	}
	out = append(out, newDialectInfo(levels, "go", "language", "native", map[string]bool{".go": true}))
	// The generic readers read the .json and .toml files no dialect claims, and
	// templ has no manifest, so they are listed, unmeasured, and a file they read
	// has a Dialect.
	out = append(out,
		newDialectInfo(levels, langroute.GenericJSON, "tooling", "native", map[string]bool{".json": true}),
		newDialectInfo(levels, langroute.GenericTOML, "tooling", "native", map[string]bool{".toml": true}),
		newDialectInfo(levels, langroute.Templ, "language", "native", map[string]bool{".templ": true}))
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func newDialectInfo(levels map[string]levelRow, id, kind, engine string, matches map[string]bool) DialectInfo {
	m := make([]string, 0, len(matches))
	for k := range matches {
		m = append(m, k)
	}
	sort.Strings(m)
	level, grade := levelOf(levels, id)
	return DialectInfo{ID: id, Kind: kind, Engine: engine, Matches: m, Level: level, Grade: grade}
}
