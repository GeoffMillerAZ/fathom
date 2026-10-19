package testrun

// Native Unity Test Framework output is NUnit3, not JUnit. Parsing records
// observations only; even a complete passing XML is never an attestation.
import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const RunnerUnityNUnit Runner = "unity-nunit"
const maxNUnitBytes = 16 << 20
const maxNUnitCases = 100000

// NUnitCase retains the producer's identity, including parameter values and
// custom names. MethodName is explicit native metadata, never name stripping.
type NUnitCase struct {
	ID, Name, FullName, Assembly, ClassName, MethodName string
	Result, Label, Duration, Message, StackTrace        string
	Cohort                                              string
}

// NUnitCohort is a native method's selected variants. A missing declared variant
// is unknown even when every returned leaf passed. Failure is never hidden.
type NUnitCohort struct {
	ID, Assembly, ClassName, MethodName string
	Expected, Observed                  int
	Complete                            bool
	ObservedVerdict                     Verdict // parsed outcome, never attestation
}

// NUnitRun is a bounded native parse, not a claim about source or execution.
type NUnitRun struct {
	ID, Result, Label, EngineVersion, StartedAt, FinishedAt, Duration string
	Total, Passed, Failed, Skipped, Inconclusive, Discovered          int
	Cases                                                             []NUnitCase
	Cohorts                                                           []NUnitCohort
}

type nunitElement struct {
	name     string
	attrs    map[string]string
	children []*nunitElement
	text     strings.Builder
}

func nunitTree(raw []byte) (*nunitElement, error) {
	if len(raw) == 0 || len(raw) > maxNUnitBytes || !utf8.Valid(raw) {
		return nil, fmt.Errorf("NUnit XML is empty, oversized or invalid UTF8")
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	var root *nunitElement
	var stack []*nunitElement
	count := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("NUnit XML: %w", err)
		}
		switch v := tok.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("NUnit XML directives/entities are refused")
		case xml.StartElement:
			count++
			if count > maxNUnitCases*10 || len(stack) >= 64 {
				return nil, fmt.Errorf("NUnit XML element/depth limit exceeded")
			}
			if v.Name.Space != "" {
				return nil, fmt.Errorf("NUnit XML namespace is unsupported")
			}
			node := &nunitElement{name: v.Name.Local, attrs: map[string]string{}}
			for _, a := range v.Attr {
				if a.Name.Space != "" {
					return nil, fmt.Errorf("NUnit XML namespaced attribute is unsupported")
				}
				if _, ok := node.attrs[a.Name.Local]; ok {
					return nil, fmt.Errorf("NUnit XML duplicate attribute %s", a.Name.Local)
				}
				node.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("NUnit XML has multiple roots")
				}
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("NUnit XML has unmatched close")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(v)) != "" {
					return nil, fmt.Errorf("NUnit XML trailing content")
				}
			} else {
				stack[len(stack)-1].text.Write(v)
			}
		}
	}
	if root == nil || root.name != "test-run" || len(stack) != 0 {
		return nil, fmt.Errorf("expected one complete native NUnit test-run")
	}
	return root, nil
}

func nunitCount(node *nunitElement, key string, required bool) (int, error) {
	text, ok := node.attrs[key]
	if !ok && !required {
		return -1, nil
	}
	if text == "" {
		return 0, fmt.Errorf("NUnit %s missing %s", node.name, key)
	}
	for _, b := range []byte(text) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("NUnit invalid %s", key)
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 || n > maxNUnitCases {
		return 0, fmt.Errorf("NUnit %s exceeds case limit", key)
	}
	return n, nil
}

// NUnitDuration parses native seconds without treating NaN/overflow as zero.
func NUnitDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds >= float64(math.MaxInt64)/1e9 {
		return 0, fmt.Errorf("invalid NUnit duration %q", value)
	}
	return time.Duration(math.Round(seconds * 1e9)), nil
}

// NUnitTime handles the native whole-second UTC spelling and RFC3339 variants.
func NUnitTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05Z", "2006-01-02 15:04:05.999999999Z"} {
		if at, err := time.Parse(layout, value); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid NUnit timestamp %q", value)
}

func nunitBody(node *nunitElement, name, child string) string {
	for _, n := range node.children {
		if n.name == name {
			for _, c := range n.children {
				if c.name == child {
					return c.text.String()
				}
			}
		}
	}
	return ""
}

// ParseUnityNUnitXML strictly parses a single native document. It does not
// execute commands, resolve methods, read files, or write test_runs.
func ParseUnityNUnitXML(raw []byte) (NUnitRun, error) {
	root, err := nunitTree(raw)
	if err != nil {
		return NUnitRun{}, err
	}
	r := NUnitRun{ID: root.attrs["id"], Result: root.attrs["result"], Label: root.attrs["label"], EngineVersion: root.attrs["engine-version"], StartedAt: root.attrs["start-time"], FinishedAt: root.attrs["end-time"], Duration: root.attrs["duration"]}
	counts := []struct {
		name string
		ptr  *int
	}{{"total", &r.Total}, {"passed", &r.Passed}, {"failed", &r.Failed}, {"skipped", &r.Skipped}, {"inconclusive", &r.Inconclusive}, {"testcasecount", &r.Discovered}}
	for _, c := range counts {
		n, e := nunitCount(root, c.name, true)
		if e != nil {
			return NUnitRun{}, e
		}
		*c.ptr = n
	}
	if r.Duration == "" {
		return NUnitRun{}, fmt.Errorf("NUnit run duration missing")
	}
	if _, err = NUnitDuration(r.Duration); err != nil {
		return NUnitRun{}, err
	}
	start, err := NUnitTime(r.StartedAt)
	if err != nil {
		return NUnitRun{}, err
	}
	finish, err := NUnitTime(r.FinishedAt)
	if err != nil || finish.Before(start) {
		return NUnitRun{}, fmt.Errorf("NUnit run interval is invalid")
	}
	if r.ID == "" {
		return NUnitRun{}, fmt.Errorf("NUnit run id missing")
	}
	ids, names := map[string]bool{}, map[string]bool{}
	groups := map[string]*NUnitCohort{}
	var walk func(*nunitElement, string, string) error
	walk = func(node *nunitElement, assembly, group string) error {
		if node.name == "test-suite" && node.attrs["type"] == "Assembly" {
			assembly = node.attrs["name"]
			if i := strings.LastIndexAny(assembly, "/\\"); i >= 0 {
				assembly = assembly[i+1:]
			}
			assembly = strings.TrimSuffix(assembly, ".dll")
			if assembly == "" {
				return fmt.Errorf("NUnit assembly name missing")
			}
		}
		if node.name == "test-suite" && node.attrs["type"] == "ParameterizedMethod" {
			group = assembly + "::" + node.attrs["id"]
			if node.attrs["id"] == "" || groups[group] != nil {
				return fmt.Errorf("NUnit parameterized suite id missing/duplicate")
			}
			count, e := nunitCount(node, "testcasecount", false)
			if e != nil {
				return e
			}
			groups[group] = &NUnitCohort{ID: group, Assembly: assembly, Expected: count}
		}
		if node.name == "test-case" {
			var inspect func(*nunitElement) bool
			inspect = func(n *nunitElement) bool {
				for _, child := range n.children {
					if child.name == "test-case" || child.name == "test-suite" || child.name == "test-run" || inspect(child) {
						return true
					}
				}
				return false
			}
			if inspect(node) {
				return fmt.Errorf("NUnit test-case contains nested test structure")
			}
			c := NUnitCase{ID: node.attrs["id"], Name: node.attrs["name"], FullName: node.attrs["fullname"], Assembly: assembly, ClassName: node.attrs["classname"], MethodName: node.attrs["methodname"], Result: node.attrs["result"], Label: node.attrs["label"], Duration: node.attrs["duration"], Cohort: group, Message: nunitBody(node, "failure", "message"), StackTrace: nunitBody(node, "failure", "stack-trace")}
			if c.ID == "" || c.Name == "" || c.FullName == "" || c.ClassName == "" || c.MethodName == "" || assembly == "" {
				return fmt.Errorf("NUnit case identity/assembly missing")
			}
			identity := assembly + "::" + c.FullName
			if ids[c.ID] || names[identity] {
				return fmt.Errorf("NUnit duplicate case id or full identity")
			}
			ids[c.ID] = true
			names[identity] = true
			switch c.Result {
			case "Passed", "Failed", "Skipped", "Inconclusive":
			default:
				return fmt.Errorf("NUnit unsupported case outcome %q", c.Result)
			}
			if c.Duration == "" {
				return fmt.Errorf("NUnit case duration missing")
			}
			if _, e := NUnitDuration(c.Duration); e != nil {
				return e
			}
			if c.Message == "" {
				c.Message = nunitBody(node, "reason", "message")
			}
			if group == "" {
				group = assembly + "::method:" + c.ClassName + "." + c.MethodName
				c.Cohort = group
				if groups[group] == nil {
					groups[group] = &NUnitCohort{ID: group, Assembly: assembly, ClassName: c.ClassName, MethodName: c.MethodName, Expected: 1, Complete: c.FullName == c.ClassName+"."+c.MethodName}
				} else {
					groups[group].Complete = false
					groups[group].Expected = -1
				}
			}
			g := groups[group]
			if g.ClassName != "" && (g.ClassName != c.ClassName || g.MethodName != c.MethodName) {
				return fmt.Errorf("NUnit cohort mixes methods")
			}
			g.ClassName = c.ClassName
			g.MethodName = c.MethodName
			g.Observed++
			r.Cases = append(r.Cases, c)
			if len(r.Cases) > maxNUnitCases {
				return fmt.Errorf("NUnit case limit exceeded")
			}
			return nil
		}
		for _, child := range node.children {
			if err := walk(child, assembly, group); err != nil {
				return err
			}
		}
		return nil
	}
	if err = walk(root, "", ""); err != nil {
		return NUnitRun{}, err
	}
	actual := map[string]int{}
	for _, c := range r.Cases {
		actual[c.Result]++
	}
	if len(r.Cases) != r.Total || r.Passed != actual["Passed"] || r.Failed != actual["Failed"] || r.Skipped != actual["Skipped"] || r.Inconclusive != actual["Inconclusive"] || r.Discovered < r.Total {
		return NUnitRun{}, fmt.Errorf("NUnit root counts contradict native cases")
	}
	switch r.Result {
	case "Passed":
		if r.Failed+r.Skipped+r.Inconclusive > 0 || r.Total == 0 {
			return NUnitRun{}, fmt.Errorf("NUnit Passed contradicts nonpassing/empty cases")
		}
	case "Failed", "Failed(Child)":
		if r.Failed == 0 {
			return NUnitRun{}, fmt.Errorf("NUnit Failed has no failed cases")
		}
	case "Skipped", "Inconclusive":
		if r.Failed > 0 {
			return NUnitRun{}, fmt.Errorf("NUnit skipped/inconclusive hides failed case")
		}
	default:
		return NUnitRun{}, fmt.Errorf("NUnit unsupported run outcome %q", r.Result)
	}
	observed := map[string]map[string]int{}
	for _, c := range r.Cases {
		if observed[c.Cohort] == nil {
			observed[c.Cohort] = map[string]int{}
		}
		observed[c.Cohort][c.Result]++
	}
	for _, g := range groups {
		if g.Expected >= 0 && g.Observed > g.Expected {
			return NUnitRun{}, fmt.Errorf("NUnit cohort exceeds declared count")
		}

		if strings.Contains(g.ID, "::method:") {
			g.Complete = g.Complete && g.Observed == 1
		} else {
			g.Complete = g.Expected > 0 && g.Observed == g.Expected
		}
		g.ObservedVerdict = Unknown
		if observed[g.ID]["Failed"] > 0 {
			g.ObservedVerdict = Fail
		} else if g.Complete && observed[g.ID]["Passed"] == g.Observed {
			g.ObservedVerdict = Pass
		}
		r.Cohorts = append(r.Cohorts, *g)
	}
	sort.Slice(r.Cases, func(i, j int) bool {
		if r.Cases[i].Assembly != r.Cases[j].Assembly {
			return r.Cases[i].Assembly < r.Cases[j].Assembly
		}
		return r.Cases[i].FullName < r.Cases[j].FullName
	})
	sort.Slice(r.Cohorts, func(i, j int) bool { return r.Cohorts[i].ID < r.Cohorts[j].ID })
	return r, nil
}

func nativeNUnitDocument(output string) bool {
	if len(output) > maxNUnitBytes {
		return false
	}
	d := xml.NewDecoder(strings.NewReader(output))
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.Directive:
			return false
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return false
			}
		case xml.StartElement:
			return t.Name.Space == "" && t.Name.Local == "test-run"
		}
	}
}

func parseUnityNUnit(output string) Run {
	native, err := ParseUnityNUnitXML([]byte(output))
	r := Run{Runner: RunnerUnityNUnit}
	if err != nil {
		return r
	}
	r.Complete = true
	for _, cohort := range native.Cohorts {
		if !cohort.Complete {
			r.Complete = false
		}
	}
	r.Summary = &Summary{Tests: native.Total, Failures: native.Failed, Skipped: native.Skipped + native.Inconclusive}
	packages := map[string]Status{}
	for _, c := range native.Cases {
		if packages[c.Assembly] == "" {
			packages[c.Assembly] = StatusOK
		}
		if c.Result == "Failed" {
			packages[c.Assembly] = StatusFail
			r.Failures = append(r.Failures, Failure{Package: c.ClassName, Test: strings.TrimPrefix(c.FullName, c.ClassName+".")})
		}
	}
	for name, status := range packages {
		r.Packages = append(r.Packages, PackageResult{Package: name, Status: status})
	}
	sort.Slice(r.Packages, func(i, j int) bool { return r.Packages[i].Package < r.Packages[j].Package })
	return r
}
