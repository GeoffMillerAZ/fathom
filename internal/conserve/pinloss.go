package conserve

// pinloss.go — PR-1 (ADR-057 decision 4, docs/design/served-not-kept.md
// section 3): PinLosses, the storeless pin-loss check the commit pin line
// (PR-3) and `fathom conserve --strict-plane integrity` (PR-9) share.
//
// A pin is a test a governing source names: an invariant's enforcement
// column, a `tests/*.cue` manifest entry, a compatibility-surface evidence
// line, a `// certifies:` marker, or a guard note's `done needs:` segment
// (PR-2, ADR-057 decision 3). PinLosses runs integrity.Check from base
// to head over the changed files with that CV-7 guarded set and reach off (a
// hook opens no store, and a pin is never inferred from reach: PR-X11), then
// keeps only the loss kinds: deleted, renamed, weakened, skipped,
// golden_changed and marker_moved. `edited` is never returned (ADR-056
// decision 8), and a test moved unchanged to another changed file (Check's
// "renamed" to the same name) is not a loss.
//
// Every read goes through PinLossInput.Read, so PR-4 can back it with one
// `git cat-file --batch` process; the only other I/O is the base-side
// manifest listing manifestCertifiesAt already does (`git ls-tree`, skipped
// when Root is ""). The guarded set comes from this package's own base-side
// helpers (scopeEvidenceAt, manifestCertifiesAt, invariantTestsAt,
// guardDoneNeedsAt), the same ones the integrity plane uses, so the two can never disagree about what is
// pinned. Output is sorted and deterministic. A retirement (no naming source
// still names the lost test at head) carries its decision cite, if the range
// has one (PR-12, pinloss_cite.go); PinLoss.Blocks is the pre-push verdict.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/integrity"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
)

const (
	pinInvariantsPath = "docs/intent/invariants.md"
	pinScopePath      = "docs/intent/scope.md"
)

// pinLossKinds are the integrity kinds that lose a pin (design section 3).
var pinLossKinds = map[integrity.Kind]bool{
	integrity.KindDeleted:       true,
	integrity.KindRenamed:       true,
	integrity.KindWeakened:      true,
	integrity.KindSkipped:       true,
	integrity.KindGoldenChanged: true,
	integrity.KindMarkerMoved:   true,
}

// PinLossInput is PinLosses' whole input: a base..head range and a blob reader.
type PinLossInput struct {
	// Root is the repository root, used only to list tests/*.cue at Base
	// (manifestCertifiesAt); "" leaves manifest pins out.
	Root string
	// Base and Head are the revisions Read is asked for: a commit sha, or
	// whatever else the caller's reader understands (conserve's worktree head).
	Base, Head string
	// ChangedFiles is the base..head changed-file set, repo-relative.
	ChangedFiles []string
	// Read returns path's content at rev; ok is false when path does not
	// exist there. An error is operational and fails the whole check.
	Read func(rev, path string) (content []byte, ok bool, err error)
}

// PinSource is one governing source that names a pinned test at base.
type PinSource struct {
	// Reason is the integrity.GuardReason: certifies_marker, manifest_entry,
	// invariant_enforcement, compatibility_surface or guard_done_needs.
	Reason string `json:"reason"`
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
}

// Cite renders s as "reason path:line".
func (s PinSource) Cite() string {
	if s.Line > 0 {
		return fmt.Sprintf("%s %s:%d", s.Reason, s.Path, s.Line)
	}
	return s.Reason + " " + s.Path
}

// PinLoss is one lost pin over base..head.
type PinLoss struct {
	Test   string `json:"test"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	// Sources is every governing source that named the test at base, sorted.
	Sources  []PinSource `json:"sources"`
	BaseFile string      `json:"base_file"`
	BaseLine int         `json:"base_line"`
	// HeadFile/HeadLine is where to look at head: the test itself for
	// weakened, skipped, golden_changed and marker_moved; the new name for
	// renamed; for deleted, the lone Test function new to the same file at
	// head (ReplacedBy), else the file with line 0, else empty when the file
	// is gone.
	HeadFile   string `json:"head_file,omitempty"`
	HeadLine   int    `json:"head_line,omitempty"`
	RenamedTo  string `json:"renamed_to,omitempty"`
	ReplacedBy string `json:"replaced_by,omitempty"`
	// StillNamedAtHead: some naming source still names the test at head, so
	// the loss is silent (PR-9 blocks on it). False means every naming source
	// dropped the name in the range: a retired pin, printed for review.
	StillNamedAtHead bool `json:"still_named_at_head"`
	// SourceEdited: a naming source was edited in the range (its file is in
	// ChangedFiles, or a marker no longer sits on the test).
	SourceEdited bool `json:"source_edited"`
	// DecisionCite is "path:line" of a line added in the range that names the
	// test as a decision (PR-12, pinloss_cite.go): in a decision record, or a
	// dated decision line; never a docs/plans line. Looked for only on a
	// retirement (StillNamedAtHead false); "" when there is none.
	DecisionCite string `json:"decision_cite,omitempty"`
}

// Silent reports whether l is a silent loss (ADR-057's PR-8 decision): the
// test is lost while its naming source still names it at head.
func (l PinLoss) Silent() bool { return l.StillNamedAtHead }

// Blocks reports whether l blocks at pre-push: a silent loss, or a retirement
// with no decision cite in the range (PR-12, ADR-057's K4 revisit clause,
// O161). Only a cited retirement is printed for review and passes.
func (l PinLoss) Blocks() bool { return l.StillNamedAtHead || l.DecisionCite == "" }

// PinLosses returns every lost pin over in's range, sorted by test, kind and
// base location. A read error is returned, never swallowed: a check that
// could not read its input has not proven anything kept.
func PinLosses(ctx context.Context, in PinLossInput) ([]PinLoss, error) {
	if in.Read == nil {
		return nil, fmt.Errorf("pin loss: no blob reader")
	}
	var readErr error
	reader := func(rev string) func(string) ([]byte, bool, error) {
		return func(p string) ([]byte, bool, error) {
			b, ok, err := in.Read(rev, p)
			if err != nil {
				if readErr == nil {
					readErr = fmt.Errorf("pin loss: read %s at %s: %w", p, rev, err)
				}
				return nil, false, err
			}
			return b, ok, nil
		}
	}
	baseFile, headFile := reader(in.Base), reader(in.Head)

	base := Input{Root: in.Root, Base: in.Base, BaseFile: baseFile}
	guard := integrity.GuardedInput{
		ManifestCertifies:    manifestCertifiesAt(ctx, base),
		InvariantTests:       invariantTestsAt(baseFile),
		SurfaceEvidenceTests: scopeEvidenceAt(baseFile),
		GuardDoneNeedsTests:  guardDoneNeedsAt(baseFile),
	}
	findings := integrity.Check(integrity.Input{
		ChangedFiles: in.ChangedFiles,
		BaseFile:     baseFile,
		HeadFile:     headFile,
		Guard:        guard,
	})
	if readErr != nil {
		return nil, readErr
	}

	changed := make(map[string]bool, len(in.ChangedFiles))
	for _, f := range in.ChangedFiles {
		changed[f] = true
	}
	n := pinNaming{ctx: ctx, in: in, base: baseFile, head: headFile, changed: changed}

	var out []PinLoss
	for _, f := range findings {
		if !pinLossKinds[f.Kind] {
			continue
		}
		if f.Kind == integrity.KindRenamed && f.RenamedTo == f.Key {
			continue // moved unchanged to another changed file: the same test, still runnable by name
		}
		l := PinLoss{
			Test: f.Key, Kind: string(f.Kind), Detail: f.Detail,
			BaseFile: f.BaseFile, BaseLine: f.BaseLine,
			HeadFile: f.HeadFile, HeadLine: f.HeadLine,
			RenamedTo: f.RenamedTo,
		}
		if f.Kind == integrity.KindDeleted {
			l.HeadFile, l.HeadLine, l.ReplacedBy = n.deletedHeadLocation(f.BaseFile)
		}
		for _, r := range f.Reasons {
			src, still, edited := n.source(r, f)
			if src.Path == "" {
				continue
			}
			l.Sources = append(l.Sources, src)
			l.StillNamedAtHead = l.StillNamedAtHead || still
			l.SourceEdited = l.SourceEdited || edited
		}
		if readErr != nil {
			return nil, readErr
		}
		sort.Slice(l.Sources, func(i, j int) bool {
			a, b := l.Sources[i], l.Sources[j]
			if a.Reason != b.Reason {
				return a.Reason < b.Reason
			}
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			return a.Line < b.Line
		})
		out = append(out, l)
	}
	var retired []string
	for _, l := range out {
		if !l.StillNamedAtHead {
			retired = append(retired, l.Test)
		}
	}
	if len(retired) > 0 {
		cites := decisionCites(baseFile, headFile, in.ChangedFiles, retired)
		if readErr != nil {
			return nil, readErr
		}
		for i := range out {
			if !out[i].StillNamedAtHead {
				out[i].DecisionCite = cites[out[i].Test]
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.BaseFile != b.BaseFile {
			return a.BaseFile < b.BaseFile
		}
		return a.BaseLine < b.BaseLine
	})
	return out, nil
}

// PinLossesOf runs PinLosses over a conserve Input's own range (NewInput's
// base sha to its worktree head), reading through in.BaseFile/in.HeadFile —
// `fathom conserve --strict-plane integrity` (PR-9).
func PinLossesOf(ctx context.Context, in Input) ([]PinLoss, error) {
	if in.BaseFile == nil || in.HeadFile == nil {
		return nil, fmt.Errorf("pin loss: the conserve input has no base or head reader")
	}
	return PinLosses(ctx, PinLossInput{
		Root:         in.Root,
		Base:         in.Base,
		Head:         in.Head,
		ChangedFiles: in.ChangedFiles,
		Read: func(rev, p string) ([]byte, bool, error) {
			switch rev {
			case in.Base:
				return in.BaseFile(p)
			case in.Head:
				return in.HeadFile(p)
			}
			return nil, false, fmt.Errorf("revision %q is neither base %q nor head %q", rev, in.Base, in.Head)
		},
	})
}

// pinNaming answers, per guard reason, where a source names a test at base,
// whether it still names it at head, and whether it was edited in the range.
type pinNaming struct {
	ctx        context.Context
	in         PinLossInput
	base, head func(string) ([]byte, bool, error)
	changed    map[string]bool

	manifestsLoaded bool
	manifestPaths   []string        // tests/*.cue at base, plus changed ones
	headCertifies   map[string]bool // every manifest-certified test name at head
}

func (n *pinNaming) source(r integrity.GuardReason, f integrity.Finding) (src PinSource, stillNamed, edited bool) {
	switch r {
	case integrity.ReasonInvariant:
		return n.docSource(r, pinInvariantsPath, f.Key, intent.ExtractInvariantEnforcementTests)
	case integrity.ReasonSurface:
		return n.docSource(r, pinScopePath, f.Key, integrity.ExtractScopeEvidenceTests)
	case integrity.ReasonGuardDoneNeeds:
		return n.docSource(r, guardFilePath, f.Key, guardDoneNeedsIn)
	case integrity.ReasonManifest:
		return n.manifestSource(f.Key)
	case integrity.ReasonMarker:
		return n.markerSource(f)
	}
	return PinSource{}, false, false // reach is off here, and names no source
}

func (n *pinNaming) docSource(r integrity.GuardReason, p, test string, extract func([]byte) []string) (PinSource, bool, bool) {
	src := PinSource{Reason: string(r), Path: p}
	if b, ok, _ := n.base(p); ok {
		src.Line = lineNaming(b, test)
	}
	still := false
	if b, ok, _ := n.head(p); ok {
		for _, name := range extract(b) {
			if name == test {
				still = true
				break
			}
		}
	}
	return src, still, n.changed[p]
}

func (n *pinNaming) loadManifests() {
	if n.manifestsLoaded {
		return
	}
	n.manifestsLoaded = true
	seen := map[string]bool{}
	if n.in.Root != "" {
		if paths, err := gitLsTree(n.ctx, n.in.Root, n.in.Base); err == nil {
			for _, p := range paths {
				if isManifestPath(p) {
					seen[p] = true
				}
			}
		}
	}
	for p := range n.changed {
		if isManifestPath(p) {
			seen[p] = true
		}
	}
	n.manifestPaths = setToSortedSlice(seen)
	files := map[string][]byte{}
	for _, p := range n.manifestPaths {
		if b, ok, err := n.head(p); err == nil && ok {
			files[p] = b
		}
	}
	n.headCertifies = map[string]bool{}
	if manifests, _, err := testevidence.LoadManifestsFromContents(files); err == nil {
		for _, m := range manifests {
			for _, su := range m.Suites {
				for _, c := range su.Certifies {
					for _, t := range c.Tests {
						n.headCertifies[t] = true
					}
				}
			}
		}
	}
}

func (n *pinNaming) manifestSource(test string) (PinSource, bool, bool) {
	n.loadManifests()
	src := PinSource{Reason: string(integrity.ReasonManifest)}
	edited := false
	for _, p := range n.manifestPaths {
		b, ok, _ := n.base(p)
		if !ok {
			continue
		}
		if line := lineNaming(b, test); line > 0 {
			if src.Path == "" {
				src.Path, src.Line = p, line
			}
			edited = edited || n.changed[p]
		}
	}
	if src.Path == "" {
		src.Path = "tests/*.cue" // named at base (the guard said so), listing unavailable
	}
	return src, n.headCertifies[test], edited
}

// markerSource: a `// certifies:` marker names its test by sitting on it, so
// it still names the test at head only when a head test of the same name
// carries the same marker text; anything else (the test or the marker gone,
// the marker moved or reworded) is an edit to the source.
func (n *pinNaming) markerSource(f integrity.Finding) (PinSource, bool, bool) {
	src := PinSource{Reason: string(integrity.ReasonMarker), Path: f.BaseFile, Line: f.BaseLine}
	baseMarker := markerOn(n.base, f.BaseFile, f.Key)
	still := false
	for _, p := range sortedKeys(n.changed) {
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		if m := markerOn(n.head, p, f.Key); m != "" && m == baseMarker {
			still = true
			break
		}
	}
	return src, still, !still
}

// markerOn returns the certifies marker text on the test named test in file
// p read through read ("" when the file, the test or the marker is absent).
func markerOn(read func(string) ([]byte, bool, error), p, test string) string {
	b, ok, err := read(p)
	if err != nil || !ok {
		return ""
	}
	for _, g := range integrity.GuardedTests(integrity.GuardedInput{Sources: []integrity.Source{{Path: p, Content: b}}}) {
		if g.Name == test {
			return g.CertifiesMarker
		}
	}
	return ""
}

// deletedHeadLocation places a deleted test at head: the lone Test function
// new to its file (a rename with a new body reads as deleted), else the file
// itself, else nothing when the file is gone too.
func (n *pinNaming) deletedHeadLocation(file string) (string, int, string) {
	h, hok, _ := n.head(file)
	if !hok {
		return "", 0, ""
	}
	b, _, _ := n.base(file)
	baseTests := map[string]bool{}
	for _, t := range topLevelTests(file, b) {
		baseTests[t.name] = true
	}
	var fresh []goTest
	for _, t := range topLevelTests(file, h) {
		if !baseTests[t.name] {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) == 1 {
		return file, fresh[0].line, fresh[0].name
	}
	return file, 0, ""
}

type goTest struct {
	name string
	line int
}

// topLevelTests lists the receiverless Test* functions in src, in source order.
func topLevelTests(file string, src []byte) []goTest {
	if len(src) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var out []goTest
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "Test") {
			continue
		}
		out = append(out, goTest{name: fd.Name.Name, line: fset.Position(fd.Pos()).Line})
	}
	return out
}

// lineNaming returns the 1-based line of the first whole-word mention of name
// in b, or 0.
func lineNaming(b []byte, name string) int {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	for i, line := range strings.Split(string(b), "\n") {
		if re.MatchString(line) {
			return i + 1
		}
	}
	return 0
}

func isManifestPath(p string) bool {
	return path.Dir(p) == "tests" && strings.HasSuffix(p, ".cue")
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
