package specdialect

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// taskMasterDialect reads Task Master's .taskmaster/tasks/tasks.json in
// place (design §3): a top-level "tasks" array of task objects, each
// optionally carrying "subtasks" (the same shape, one level deep — Task
// Master does not nest further); a tag-scoped export (a tasks.json whose
// top level is tag name -> {tasks: […]} rather than a bare "tasks" array)
// is a needs-strategy case this adapter does not read. status: "done" is
// the only claimed state; every other status ("pending", "in-progress",
// "deferred", "cancelled", "blocked", "review", …) reads unclaimed. A
// task's anchor comes from an inline fathom annotation on its own title,
// else the first repo-relative path named across its details and
// testStrategy text (design §3's own "anchor sources" column for this
// dialect) — the title and description are read for the annotation and
// for display only, never scanned for anchors, since a Task Master title
// commonly names files only in passing ("Add retry to the payment
// client").
type taskMasterDialect struct{}

// TaskMaster returns the Task Master adapter.
func TaskMaster() SpecDialect { return taskMasterDialect{} }

func (taskMasterDialect) ID() string { return "taskmaster" }

func taskMasterPath(p string) bool {
	return hasSegmentsSuffix(p, ".taskmaster", "tasks", "tasks.json")
}

// taskMasterShape reports whether content parses as JSON carrying a
// top-level "tasks" array — the flat shape this adapter reads.
func taskMasterShape(content []byte) bool {
	var doc struct {
		Tasks *[]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(content, &doc); err != nil {
		return false
	}
	return doc.Tasks != nil
}

func (taskMasterDialect) Detect(root string, files []string) []string {
	return detectBy(root, files, taskMasterPath, taskMasterShape)
}

func (taskMasterDialect) Claims(path string, content []byte) bool {
	return taskMasterPath(path) && taskMasterShape(content)
}

func (taskMasterDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	return taskMasterParse(path, content)
}

// tmTask is one Task Master task or subtask object — the fields this
// adapter reads; a real tasks.json may carry others fathom ignores.
type tmTask struct {
	ID           json.Number `json:"id"`
	Title        string      `json:"title"`
	Details      string      `json:"details"`
	TestStrategy string      `json:"testStrategy"`
	Status       string      `json:"status"`
	Subtasks     []tmTask    `json:"subtasks"`
}

// tmIDKeyRe matches a JSON "id" key's own line — the best-effort line
// pointer this adapter gives a verdict (JSON carries no checkbox column
// to point a Span at); it depends on the file's own "id" keys appearing
// one per line, true of every tasks.json this adapter has been shown and
// of json.MarshalIndent's own output, but not guaranteed for a
// hand-minified file, in which case every task's Line reads 0.
var tmIDKeyRe = regexp.MustCompile(`^\s*"id"\s*:`)

// taskMasterLineNumbers returns, in document order, the 1-based line of
// every "id" key in content.
func taskMasterLineNumbers(content []byte) []int {
	var out []int
	for i, line := range strings.Split(string(content), "\n") {
		if tmIDKeyRe.MatchString(line) {
			out = append(out, i+1)
		}
	}
	return out
}

// taskMasterParse maps a Task Master tasks.json into the canonical
// model, in document order, each subtask immediately after its parent —
// a subtask's id is "<parent>.<own>" (Task Master's own dotted
// convention). Pure: the "id"-key line scan reads the same bytes handed
// in, never touches disk.
func taskMasterParse(path string, content []byte) (verify.Plan, []Span, error) {
	var doc struct {
		Tasks []tmTask `json:"tasks"`
	}
	if err := json.Unmarshal(content, &doc); err != nil {
		return verify.Plan{}, nil, fmt.Errorf("taskmaster: %s: %w", path, err)
	}
	lines := taskMasterLineNumbers(content)
	li := 0
	nextLine := func() int {
		if li < len(lines) {
			l := lines[li]
			li++
			return l
		}
		return 0
	}

	var (
		plan  verify.Plan
		spans []Span
	)
	var walk func(t tmTask, parentID string)
	walk = func(t tmTask, parentID string) {
		id := strings.TrimSpace(t.ID.String())
		if parentID != "" {
			id = parentID + "." + id
		}
		line := nextLine()

		// Re-frame the title through the native grammar's own tokenizer
		// (the same trick foreignTask uses), so an inline fathom
		// annotation on a Task Master title means exactly what it means
		// natively.
		parsed := verify.ParseMarkdownPlan("", []byte("- [ ] "+t.Title+"\n"))
		var vt verify.Task
		if len(parsed.Tasks) == 1 {
			vt = parsed.Tasks[0]
		} else {
			vt = verify.Task{Weight: 1}
		}
		vt.ID = id
		vt.Claimed = t.Status == "done"
		vt.Line = line
		vt.SpecIDs = nil

		if !hasContract(vt) {
			for i, p := range namedPaths(t.Details + " " + t.TestStrategy) {
				if i == 0 && !strings.HasSuffix(strings.ToLower(p), ".md") {
					vt.Oracle = "lexical"
					vt.OraclePath = p
					continue
				}
				vt.Ctx = append(vt.Ctx, p)
			}
		}

		plan.Tasks = append(plan.Tasks, vt)
		spans = append(spans, Span{TaskID: id, Line: line, Col: 1})
		for _, sub := range t.Subtasks {
			walk(sub, id)
		}
	}
	for _, t := range doc.Tasks {
		walk(t, "")
	}
	return plan, spans, nil
}
