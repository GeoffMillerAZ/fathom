package ask

import (
	"fmt"
	"regexp"
	"strings"
)

// RequestState is the State every E1 request carries, verbatim; each
// Question's Instructions is QuestionText's.
const RequestState = "Each question below is a statement about one Go function, shown in the question. Judge each statement independently and give the probability that it is true."

// MaxQuestionBytes bounds one question's text: a longer span is excluded
// from its template's pool (ExcludedTooLong), well inside the provider's
// limit for the state plus the longest question.
const MaxQuestionBytes = 16 * 1024

// ExcludedTooLong: the span's question would pass MaxQuestionBytes.
const ExcludedTooLong = "too_long"

// Template is one of E1's four judged predicates: the statement sent
// verbatim, the fact that is its truth, and its lexical baseline, the grep
// a careful engineer would write (Baseline, verbatim regular expressions in
// Go's RE2 syntax, read by BaselineRule).
type Template struct {
	ID           string
	Question     string
	Baseline     []string
	BaselineRule string

	baseline func(src string) bool
	truth    func(Facts) (applies, value bool)
	guard    bool
}

// Truth is the template's computed truth over f: applies is false when the
// span has nothing the question is about.
func (t Template) Truth(f Facts) (applies, value bool) { return t.truth(f) }

// BaselineAnswer is the lexical baseline's answer over a span's source.
func (t Template) BaselineAnswer(src string) bool { return t.baseline(src) }

func anyMatch(res []*regexp.Regexp, src string) bool {
	for _, re := range res {
		if re.MatchString(src) {
			return true
		}
	}
	return false
}

func compileAll(exprs []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(exprs))
	for i, e := range exprs {
		out[i] = regexp.MustCompile(e)
	}
	return out
}

var (
	errcheckBaseline = []string{
		`(?m)^\s*_\s*=\s*[\w.]+\(`,
		`,\s*_\s*:?=\s*[\w.]+\(`,
		`(?m)^\s*fmt\.(Print|Fprint)\w*\(`,
		`(?m)^\s*[\w.]+\.(Close|Write|WriteString|Flush|Remove|RemoveAll|Encode|Sync|Shutdown|Rollback)\(.*\)\s*$`,
	}
	wrapBaseline = []string{
		`fmt\.Errorf\("(?:[^"\\%]|\\.|%[^w"])*"\s*,[^\n]*\berr\w*\b`,
	}
	assertBaseline = []string{
		`\bt\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow|Skip|Skipf|SkipNow)\(`,
		`[(,]\s*t\s*[,)]`,
	}
	lockBaseline = []string{
		`\.Lock\(\)`,
		`(?m)^\s*[A-Za-z_]\w*\.\w+(\[[^\]]*\])?\s*([-+*/%&|^]?=[^=]|\+\+|--)`,
	}
)

// Templates are E1's four templates in their pre-registered order.
var Templates = func() []Template {
	errRes, wrapRes, assertRes, lockRes := compileAll(errcheckBaseline), compileAll(wrapBaseline), compileAll(assertBaseline), compileAll(lockBaseline)
	return []Template{
		{
			ID:           "errcheck",
			Question:     "Every error this Go function receives from a call is checked: no call whose result list includes the built-in error type is used as a bare expression statement, and no such call has its error result assigned to the blank identifier _. Calls inside function literals count; the call a defer or go statement makes is not a bare expression statement; fmt.Print, fmt.Fprintf and Write-style calls count like any other call.",
			Baseline:     errcheckBaseline,
			BaselineRule: "false when any expression matches the function's text, true otherwise",
			baseline:     func(src string) bool { return !anyMatch(errRes, src) },
			truth: func(f Facts) (bool, bool) {
				return f.UncheckedError.Applies, !f.UncheckedError.Holds
			},
		},
		{
			ID:           "wrap",
			Question:     "Every error value this Go function formats with fmt.Errorf is formatted with the %w verb: no argument whose type implements error is formatted with %v, %s or any other verb.",
			Baseline:     wrapBaseline,
			BaselineRule: "false when the expression matches the function's text, true otherwise",
			baseline:     func(src string) bool { return !anyMatch(wrapRes, src) },
			truth: func(f Facts) (bool, bool) {
				return f.Wrapped.Applies, f.Wrapped.Holds
			},
		},
		{
			ID:           "assert",
			Question:     "This Go test asserts: its body, function literals in it included, calls a testing method named Error, Errorf, Fatal, Fatalf, Fail, FailNow, Skip, Skipf or SkipNow, or passes its *testing.T (or a testing.TB) to a function.",
			Baseline:     assertBaseline,
			BaselineRule: "true when any expression matches the function's text, false otherwise",
			baseline:     func(src string) bool { return anyMatch(assertRes, src) },
			truth: func(f Facts) (bool, bool) {
				return f.ExecutesOnly.Applies, !f.ExecutesOnly.Holds
			},
		},
		{
			ID:           "lock",
			Question:     "Every write in this method to a receiver field that the receiver type declares after a sync.Mutex or sync.RWMutex field (guarded by the nearest such mutex declared before it) is preceded, in statement order within the method, by a Lock call on that mutex through the receiver with no Unlock call on it in between. RLock is not a Lock, a deferred call is not in between, and function literals in the method are ignored.",
			Baseline:     lockBaseline,
			BaselineRule: "true when the first expression (a Lock call) matches before the second (a field write) first matches, or matches and the second never does; false otherwise",
			baseline: func(src string) bool {
				lock := lockRes[0].FindStringIndex(src)
				if lock == nil {
					return false
				}
				write := lockRes[1].FindStringIndex(src)
				return write == nil || lock[0] < write[0]
			},
			truth: func(f Facts) (bool, bool) {
				return f.LockedWrite.Applies, f.LockedWrite.Holds
			},
			guard: true,
		},
	}
}()

// TemplateByID is the template with id, or ok=false.
func TemplateByID(id string) (Template, bool) {
	for _, t := range Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// QuestionText is the exact Instructions sent for c under t: the
// statement, the receiver type's declaration for the lock template, then
// the function.
func QuestionText(t Template, c Computed) string {
	var b strings.Builder
	b.WriteString(t.Question)
	if t.guard && c.Guard != "" {
		b.WriteString("\n\nReceiver type:\n```go\n")
		b.WriteString(c.Guard)
		b.WriteString("\n```")
	}
	fmt.Fprintf(&b, "\n\nFunction %s/%s, line %d:\n```go\n%s\n```", c.Span.Repo, c.Span.File, c.Line, c.Source)
	return b.String()
}

// Pool is t's candidate pool over cs: every computed span the template
// applies to, in input order, less those whose question passes
// MaxQuestionBytes (counted in tooLong).
func Pool(t Template, cs []Computed) (pool []Computed, tooLong int) {
	for _, c := range cs {
		if c.Excluded != "" {
			continue
		}
		if applies, _ := t.Truth(c.Facts); !applies {
			continue
		}
		if len(QuestionText(t, c)) > MaxQuestionBytes {
			tooLong++
			continue
		}
		pool = append(pool, c)
	}
	return pool, tooLong
}
