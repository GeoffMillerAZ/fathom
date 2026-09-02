package publicloc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// FuncSpan is one top-level function or method at base_commit with its
// 1-based line span. Name is qualified as Recv.Method for methods (the
// receiver's base type name, pointer stripped) and bare for functions —
// the form the task prompt asks the arm to answer in.
type FuncSpan struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// FuncSpans parses one Go file and lists its function declarations in
// source order.
func FuncSpans(src []byte, filename string) ([]FuncSpan, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var spans []FuncSpan
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		spans = append(spans, FuncSpan{
			Name:  QualifiedFuncName(fd),
			Start: fset.Position(fd.Pos()).Line,
			End:   fset.Position(fd.End()).Line,
		})
	}
	return spans, nil
}

// QualifiedFuncName is Recv.Method for a method, Name for a function.
func QualifiedFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return receiverBase(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func receiverBase(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverBase(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverBase(t.X)
	case *ast.IndexListExpr:
		return receiverBase(t.X)
	case *ast.ParenExpr:
		return receiverBase(t.X)
	}
	return "?"
}

// EnclosingFuncs applies the registered attribution rule to touches
// against the spans of one file: a deleted line belongs to the function
// whose span contains it; an inserted run belongs to a function only when
// both of its base_commit neighbours are inside that same function. The
// result is sorted by start line and unique.
func EnclosingFuncs(spans []FuncSpan, touches []Touch) []FuncSpan {
	at := func(line int) *FuncSpan {
		for i := range spans {
			if line >= spans[i].Start && line <= spans[i].End {
				return &spans[i]
			}
		}
		return nil
	}
	hit := map[string]FuncSpan{}
	for _, t := range touches {
		for _, l := range t.Deleted {
			if s := at(l); s != nil {
				hit[s.Name] = *s
			}
		}
		for _, before := range t.Inserted {
			prev, next := at(before-1), at(before)
			if prev != nil && next != nil && prev.Name == next.Name {
				hit[prev.Name] = *prev
			}
		}
	}
	out := make([]FuncSpan, 0, len(hit))
	for _, s := range hit {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// EnclosingFuncsForHunks is the function_localization derivation for one
// file: parse src (the file as it is at base_commit) with go/parser and
// attribute each hunk's touched lines to the functions declared there.
func EnclosingFuncsForHunks(src []byte, filename string, hunks []Hunk) ([]FuncSpan, error) {
	spans, err := FuncSpans(src, filename)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	touches := make([]Touch, 0, len(hunks))
	for _, h := range hunks {
		touches = append(touches, TouchedOldLines(h))
	}
	return EnclosingFuncs(spans, touches), nil
}

// EnclosingFuncsForRegions is the same derivation from line-level truth
// (a region is treated as a run of changed lines).
func EnclosingFuncsForRegions(src []byte, filename string, regions []Region) ([]FuncSpan, error) {
	spans, err := FuncSpans(src, filename)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	var t Touch
	for _, r := range regions {
		for l := r.Start; l <= r.End; l++ {
			t.Deleted = append(t.Deleted, l)
		}
	}
	return EnclosingFuncs(spans, []Touch{t}), nil
}

// FuncRef names one gold function by declaring file and qualified name —
// the same key cmd/loctruth reports callers under.
type FuncRef struct {
	File string `json:"file"`
	Func string `json:"func"`
	Line int    `json:"line,omitempty"`
}

// CallEdge is one static call recorded by cmd/loctruth: Caller calls
// Callee at Line of CallerFile. Both ends are repo-relative.
type CallEdge struct {
	CalleeFile string `json:"callee_file"`
	Callee     string `json:"callee"`
	CallerFile string `json:"caller_file"`
	Caller     string `json:"caller"`
	Line       int    `json:"line"`
}

// CallersFile is cmd/loctruth's output for one module at one commit.
type CallersFile struct {
	Tool               string     `json:"tool"`
	Module             string     `json:"module"`
	Packages           int        `json:"packages"`
	PackagesWithErrors int        `json:"packages_with_errors"`
	Edges              []CallEdge `json:"edges"`
}

// CallersOf lists the distinct callers of one function in the truth file,
// excluding the function itself (recursion is not a caller an agent must
// find) and excluding test files, sorted by name.
func (cf CallersFile) CallersOf(ref FuncRef) []FuncRef {
	seen := map[string]FuncRef{}
	for _, e := range cf.Edges {
		if e.CalleeFile != ref.File || e.Callee != ref.Func {
			continue
		}
		if IsTestPath(e.CallerFile) || (e.CallerFile == ref.File && e.Caller == ref.Func) {
			continue
		}
		k := e.CallerFile + ":" + e.Caller
		if _, ok := seen[k]; !ok {
			seen[k] = FuncRef{File: e.CallerFile, Func: e.Caller}
		}
	}
	out := make([]FuncRef, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Func != out[j].Func {
			return out[i].Func < out[j].Func
		}
		return out[i].File < out[j].File
	})
	return out
}

// Env is how derivation reaches a subject's base_commit snapshot. ReadFile
// returns a file as it is at base_commit; Callers returns cmd/loctruth's
// truth for the instance. A nil Callers derives no find_callers tasks
// (recorded in the corpus, never silent).
type Env struct {
	ReadFile func(instanceID, relPath string) ([]byte, error)
	Callers  func(instanceID string) (CallersFile, error)
}

// MaxCallers caps a find_callers task: a function with more static callers
// than this is infrastructure, not a localization target, and the task
// would grade recall over a list nobody would type.
const MaxCallers = 8

// MaxPredicted is the H2-2 cap on the predicted set: max(5, 2·|gold|).
func MaxPredicted(gold int) int {
	if 2*gold > 5 {
		return 2 * gold
	}
	return 5
}

// Subject is where a task's truth lives: the repo and commit the runner
// snapshots into a scratch store under the given service name.
type Subject struct {
	RepoURL    string `json:"repo_url"`
	BaseCommit string `json:"base_commit"`
	Service    string `json:"service"`
}

// Derivation is the audit trail of one task: enough for a stranger to
// recompute its ground truth from the pinned inputs without this package.
type Derivation struct {
	Source     string    `json:"source"`
	InstanceID string    `json:"instance_id"`
	Rule       string    `json:"rule"`
	Tool       string    `json:"tool"`
	Files      []string  `json:"files,omitempty"`
	Functions  []FuncRef `json:"functions,omitempty"`
	Callee     *FuncRef  `json:"callee,omitempty"`
	Callers    []FuncRef `json:"callers,omitempty"`
	Regions    []Region  `json:"regions,omitempty"`
	PatchSHA   string    `json:"patch_sha256,omitempty"`
	Notes      []string  `json:"notes,omitempty"`
}

// Task is one public-corpus row. Its JSON is a superset of
// internal/contextbench.Task (id, class, category, prompt, ground_truth,
// scoring_rule, source, notes), so the existing runner loads it unchanged;
// the extra fields carry H2-2 (max_predicted), the split (requires), the
// subject, and the derivation.
type Task struct {
	ID           string     `json:"id"`
	Class        string     `json:"class"`
	Category     string     `json:"category"`
	Prompt       string     `json:"prompt"`
	GroundTruth  []string   `json:"ground_truth"`
	ScoringRule  string     `json:"scoring_rule"`
	MaxPredicted int        `json:"max_predicted"`
	Requires     []string   `json:"requires"`
	Source       string     `json:"source"`
	Subject      Subject    `json:"subject"`
	Derivation   Derivation `json:"derivation"`
}

// Requires vocabulary. subject-snapshot: the runner must ingest the
// subject at base_commit into a scratch store (PB-5 --scratch-daemon);
// typed-call-edges: the index must carry go/types-resolved call edges for
// the answer to be reachable through find_callers.
const (
	RequiresSubjectSnapshot = "subject-snapshot"
	RequiresTypedCallEdges  = "typed-call-edges"
)

// InstanceDerivation is what DeriveLocalizationTasks produced for one
// instance, with the reasons any class was not derived.
type InstanceDerivation struct {
	InstanceID string   `json:"instance_id"`
	Tasks      int      `json:"tasks"`
	Omitted    []string `json:"omitted,omitempty"`
}

// DeriveLocalizationTasks derives every task for the selected instances.
// Instances are processed in instance_id order and tasks within an
// instance in class order (file, function, callers by function name), so
// the output is independent of the input order.
func DeriveLocalizationTasks(instances []Instance, env Env) ([]Task, []InstanceDerivation, error) {
	sorted := append([]Instance(nil), instances...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].InstanceID < sorted[j].InstanceID })

	var tasks []Task
	var report []InstanceDerivation
	for _, in := range sorted {
		its, rep, err := deriveInstance(in, env)
		if err != nil {
			return nil, nil, fmt.Errorf("instance %s: %w", in.InstanceID, err)
		}
		tasks = append(tasks, its...)
		report = append(report, rep)
	}
	return tasks, report, nil
}

func deriveInstance(in Instance, env Env) ([]Task, InstanceDerivation, error) {
	rep := InstanceDerivation{InstanceID: in.InstanceID}
	files, err := GoldFiles(in)
	if err != nil {
		return nil, rep, err
	}
	if len(files) == 0 {
		return nil, rep, fmt.Errorf("no gold files (selection should have excluded it)")
	}
	subject := Subject{RepoURL: in.RepoURL(), BaseCommit: in.BaseCommit, Service: in.Service()}
	source := fmt.Sprintf("%s %s @ %s (%s)", in.Source, in.InstanceID, in.BaseCommit, Sources[in.Source].License)
	base := Derivation{Source: in.Source, InstanceID: in.InstanceID, Regions: in.Regions}
	if in.Patch != "" {
		base.PatchSHA = SHA256Hex([]byte(in.Patch))
	}
	var tasks []Task

	// file_localization
	{
		d := base
		d.Rule = "non-test .go paths the gold patch modifies or deletes (added files are not gold); all-of within max_predicted"
		d.Tool = "publicloc.GoldFiles"
		if in.Patch == "" {
			d.Rule = "non-test .go paths of the line-level regions; all-of within max_predicted"
		}
		d.Files = files
		tasks = append(tasks, Task{
			ID:           "PG-" + in.SafeID() + "-file",
			Class:        "file_localization",
			Category:     "file_localization",
			Prompt:       filePrompt(in),
			GroundTruth:  files,
			ScoringRule:  "all-of",
			MaxPredicted: MaxPredicted(len(files)),
			Requires:     []string{RequiresSubjectSnapshot},
			Source:       source,
			Subject:      subject,
			Derivation:   d,
		})
	}

	// function_localization
	if env.ReadFile == nil {
		rep.Omitted = append(rep.Omitted, "function_localization: no base_commit snapshot reader", "find_callers: no base_commit snapshot reader")
		rep.Tasks = len(tasks)
		return tasks, rep, nil
	}
	var funcs []FuncRef
	var notes []string
	var patchFiles []PatchFile
	if in.Patch != "" {
		patchFiles, err = ParsePatch(in.Patch)
		if err != nil {
			return nil, rep, err
		}
	}
	for _, path := range files {
		src, err := env.ReadFile(in.InstanceID, path)
		if err != nil {
			return nil, rep, fmt.Errorf("reading %s at base_commit: %w", path, err)
		}
		var spans []FuncSpan
		if in.Patch != "" {
			var hunks []Hunk
			for _, pf := range patchFiles {
				if pf.OldPath == path && !pf.Added {
					hunks = append(hunks, pf.Hunks...)
				}
			}
			spans, err = EnclosingFuncsForHunks(src, path, hunks)
		} else {
			var regs []Region
			for _, r := range in.Regions {
				if r.Path == path {
					regs = append(regs, r)
				}
			}
			spans, err = EnclosingFuncsForRegions(src, path, regs)
		}
		if err != nil {
			return nil, rep, err
		}
		if len(spans) == 0 {
			notes = append(notes, path+": no function encloses a changed line (top-level change)")
		}
		for _, s := range spans {
			funcs = append(funcs, FuncRef{File: path, Func: s.Name, Line: s.Start})
		}
	}
	sort.Slice(funcs, func(i, j int) bool {
		if funcs[i].Func != funcs[j].Func {
			return funcs[i].Func < funcs[j].Func
		}
		return funcs[i].File < funcs[j].File
	})
	if len(funcs) == 0 {
		rep.Omitted = append(rep.Omitted, "function_localization: the change touches no function body")
	} else {
		d := base
		d.Rule = "functions whose span at base_commit contains a deleted line, or both neighbours of an inserted run; all-of within max_predicted"
		d.Tool = "publicloc.EnclosingFuncsForHunks (go/parser)"
		if in.Patch == "" {
			d.Rule = "functions whose span at base_commit intersects a line-level region; all-of within max_predicted"
			d.Tool = "publicloc.EnclosingFuncsForRegions (go/parser)"
		}
		d.Functions = funcs
		d.Notes = notes
		gold := make([]string, 0, len(funcs))
		seen := map[string]bool{}
		for _, f := range funcs {
			if !seen[f.Func] {
				seen[f.Func] = true
				gold = append(gold, f.Func)
			}
		}
		tasks = append(tasks, Task{
			ID:           "PG-" + in.SafeID() + "-func",
			Class:        "function_localization",
			Category:     "function_localization",
			Prompt:       funcPrompt(in),
			GroundTruth:  gold,
			ScoringRule:  "all-of",
			MaxPredicted: MaxPredicted(len(gold)),
			Requires:     []string{RequiresSubjectSnapshot},
			Source:       source,
			Subject:      subject,
			Derivation:   d,
		})
	}

	// find_callers
	if env.Callers == nil {
		rep.Omitted = append(rep.Omitted, "find_callers: no callers truth (run cmd/loctruth and pass -callers-dir)")
		rep.Tasks = len(tasks)
		return tasks, rep, nil
	}
	if len(funcs) > 0 {
		truth, err := env.Callers(in.InstanceID)
		if err != nil {
			return nil, rep, fmt.Errorf("callers truth: %w", err)
		}
		for _, f := range funcs {
			callers := truth.CallersOf(f)
			switch {
			case len(callers) == 0:
				rep.Omitted = append(rep.Omitted, "find_callers "+f.Func+": no non-test static callers")
				continue
			case len(callers) > MaxCallers:
				rep.Omitted = append(rep.Omitted, fmt.Sprintf("find_callers %s: %d callers exceeds the cap of %d", f.Func, len(callers), MaxCallers))
				continue
			}
			d := base
			d.Rule = fmt.Sprintf("non-test static callers of the function at base_commit as listed by cmd/loctruth (go/types); self-calls excluded; 1–%d callers; all-of within max_predicted", MaxCallers)
			d.Tool = "cmd/loctruth"
			callee := f
			d.Callee = &callee
			d.Callers = callers
			gold := make([]string, 0, len(callers))
			seen := map[string]bool{}
			for _, c := range callers {
				if !seen[c.Func] {
					seen[c.Func] = true
					gold = append(gold, c.Func)
				}
			}
			tasks = append(tasks, Task{
				ID:           "PG-" + in.SafeID() + "-callers-" + strings.ReplaceAll(f.Func, ".", "_"),
				Class:        "find_callers",
				Category:     "find_callers",
				Prompt:       callersPrompt(in, f),
				GroundTruth:  gold,
				ScoringRule:  "all-of",
				MaxPredicted: MaxPredicted(len(gold)),
				Requires:     []string{RequiresSubjectSnapshot, RequiresTypedCallEdges},
				Source:       source,
				Subject:      subject,
				Derivation:   d,
			})
		}
	}
	rep.Tasks = len(tasks)
	return tasks, rep, nil
}

const answerFormat = "Put your final answer inside a single fenced code block, one entry per line and nothing else in the block; entries beyond the task's max_predicted are not scored."

func problemBlock(in Instance) string {
	return "Issue (verbatim):\n\n" + strings.TrimSpace(in.Problem)
}

func filePrompt(in Instance) string {
	return fmt.Sprintf("The following issue was filed against %s/%s at commit %s. Which non-test Go source files must change to fix it? List each repository-relative path. %s\n\n%s",
		in.Org, in.Repo, in.BaseCommit, answerFormat, problemBlock(in))
}

func funcPrompt(in Instance) string {
	return fmt.Sprintf("The following issue was filed against %s/%s at commit %s. Which functions or methods must change to fix it? Write a function as Name and a method as Type.Method (receiver base type, no pointer). %s\n\n%s",
		in.Org, in.Repo, in.BaseCommit, answerFormat, problemBlock(in))
}

func callersPrompt(in Instance, f FuncRef) string {
	return fmt.Sprintf("In %s/%s at commit %s, which functions or methods call `%s` (declared in %s)? Exclude test files and the function itself; count only direct static calls. Write a function as Name and a method as Type.Method (receiver base type, no pointer). %s",
		in.Org, in.Repo, in.BaseCommit, f.Func, f.File, answerFormat)
}
