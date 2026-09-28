package langbench

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// CntAdjudicated counts dialect-only definitions admitted to a reading's
// adjudicated set (owner decision D3): they leave sym_precision's
// denominator.
const CntAdjudicated = "adjudicated"

// Adjudication is an oracle-deficiency disclosure file (D3): classes of
// verified oracle misses, each with why and an optional structural pattern
// ({name} stands for the definition's name), and the dialect-only
// definitions claimed under them.
type Adjudication struct {
	Classes     map[string]AdjudicationClass `json:"classes"`
	Definitions []AdjudicatedDef             `json:"definitions"`
}

// AdjudicationClass is one kind of oracle deficiency.
type AdjudicationClass struct {
	Why     string `json:"why"`
	Pattern string `json:"pattern,omitempty"`
}

// AdjudicatedDef is one claimed dialect-only definition.
type AdjudicatedDef struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// Adjudicate admits each claimed definition by a deterministic check only —
// never anyone's judgment: fathom emitted a definition of that name at
// exactly that file:line inside the kind intersection, the L1 oracle
// matched nothing to it (it is dialect-only), and the definition's source
// text (its first line through at most two more, never past its end)
// contains the name as a whole word and, when the class gives one, a match
// of the class pattern. Admitted entries are counted in l1.Counts under
// CntAdjudicated and returned as "path:line name (class)"; refused ones
// are returned with the reason.
func Adjudicate(l1 *L1Result, nodes []core.SymbolNode, adj Adjudication, src func(path string) []byte) (accepted, refused []string) {
	type key struct {
		path, name string
		line       int
	}
	dialectOnly := map[key]int{} // -> end line
	for _, d := range l1.DialectOnly {
		dialectOnly[key{d.Path, d.Name, d.Line}] = d.End
	}
	emitted := map[key]bool{}
	for _, n := range nodes {
		emitted[key{n.FilePath, BareName(n.Name), n.Line}] = true
		emitted[key{n.FilePath, n.Name, n.Line}] = true
	}
	patterns := map[string]*regexp.Regexp{}
	seen := map[key]bool{}
	for _, e := range adj.Definitions {
		label := fmt.Sprintf("%s:%d %s (%s)", e.Path, e.Line, e.Name, e.Class)
		refuse := func(why string) { refused = append(refused, label+": "+why) }
		class, ok := adj.Classes[e.Class]
		if !ok || strings.TrimSpace(class.Why) == "" {
			refuse("no such class, or a class without a why")
			continue
		}
		k := key{e.Path, e.Name, e.Line}
		end, only := dialectOnly[k]
		switch {
		case seen[k]:
			refuse("listed twice")
			continue
		case !only && emitted[k]:
			refuse("the oracle matched it, or its kind is outside the intersection")
			continue
		case !only:
			refuse("fathom emitted no such definition at this line")
			continue
		}
		lines := strings.Split(string(src(e.Path)), "\n")
		last := min(e.Line+2, len(lines))
		if end >= e.Line && end < last {
			last = end
		}
		if e.Line < 1 || e.Line > len(lines) {
			refuse("line outside the file")
			continue
		}
		text := strings.Join(lines[e.Line-1:last], "\n")
		word := regexp.MustCompile(`(^|[^\pL\pN_$])` + regexp.QuoteMeta(e.Name) + `($|[^\pL\pN_$])`)
		if !word.MatchString(text) {
			refuse("the definition's text does not contain its name")
			continue
		}
		if class.Pattern != "" {
			re, ok := patterns[e.Class+"\x00"+e.Name]
			if !ok {
				var err error
				re, err = regexp.Compile(strings.ReplaceAll(class.Pattern, "{name}", regexp.QuoteMeta(e.Name)))
				if err != nil {
					refuse("class pattern does not compile: " + err.Error())
					continue
				}
				patterns[e.Class+"\x00"+e.Name] = re
			}
			if !re.MatchString(text) {
				refuse("the definition's text does not match the class pattern")
				continue
			}
		}
		seen[k] = true
		accepted = append(accepted, label)
		l1.Counts[CntAdjudicated]++
	}
	sort.Strings(accepted)
	sort.Strings(refused)
	return accepted, refused
}
