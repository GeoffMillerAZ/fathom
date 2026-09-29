package checkcatalog

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Mismatch is one id declared by both the catalog and an implementation whose
// declared fields differ.
type Mismatch struct {
	// ID is the check both sides claim.
	ID string
	// Field is which declared field differs: name, severity, means or rejects.
	Field string
	// Catalog is the catalog's value.
	Catalog string
	// Impl is the implementation's value.
	Impl string
}

// String renders the mismatch as one line.
func (m Mismatch) String() string {
	return fmt.Sprintf("%s: %s is %q in code, %q in catalog", m.ID, m.Field, m.Impl, m.Catalog)
}

// Drift is what Reconcile found: every way the catalog and the
// implementations disagree. The zero value is clean.
type Drift struct {
	// Unimplemented lists catalog ids with no check — promises the code does
	// not keep.
	Unimplemented []string
	// Undeclared lists check ids with no catalog row — undocumented gates.
	Undeclared []string
	// Mismatched lists ids on both sides whose declared fields differ.
	Mismatched []Mismatch
	// DuplicateRows lists ids the catalog declares more than once.
	DuplicateRows []string
	// DuplicateChecks lists ids more than one check claims.
	DuplicateChecks []string
	// NoFn lists checks declared with a nil Fn — they would silently never
	// run.
	NoFn []string
	// Invalid lists rows, on either side, that fail Row.Validate, each with
	// its reason.
	Invalid []string
}

// Clean reports whether the catalog and the implementations agree exactly.
func (d Drift) Clean() bool {
	return len(d.Unimplemented) == 0 && len(d.Undeclared) == 0 && len(d.Mismatched) == 0 &&
		len(d.DuplicateRows) == 0 && len(d.DuplicateChecks) == 0 && len(d.NoFn) == 0 && len(d.Invalid) == 0
}

// Err returns nil when the drift is clean, otherwise an error whose text is
// String — so a consumer's bidirectional test is one call and one assertion.
func (d Drift) Err() error {
	if d.Clean() {
		return nil
	}
	return errors.New(d.String())
}

// String lists every line of drift, one per line, in a fixed order.
func (d Drift) String() string {
	if d.Clean() {
		return "catalog and implementations agree"
	}
	var b strings.Builder
	for _, id := range d.Unimplemented {
		fmt.Fprintf(&b, "catalog row %s has NO implementation — the catalog promises a check the code does not run\n", id)
	}
	for _, id := range d.Undeclared {
		fmt.Fprintf(&b, "check %s runs but is NOT in the catalog — an undocumented gate\n", id)
	}
	for _, m := range d.Mismatched {
		b.WriteString(m.String() + "\n")
	}
	for _, id := range d.DuplicateRows {
		fmt.Fprintf(&b, "catalog declares %s twice\n", id)
	}
	for _, id := range d.DuplicateChecks {
		fmt.Fprintf(&b, "two implementations claim id %s\n", id)
	}
	for _, id := range d.NoFn {
		fmt.Fprintf(&b, "check %s is declared with no function — it would silently never run\n", id)
	}
	for _, s := range d.Invalid {
		b.WriteString(s + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Reconcile is the bidirectional test: it compares the declared catalog with
// the checks that actually run and returns everything on which they
// disagree. A clean Drift means "what this gate checks" has exactly one
// answer. Ids are compared exactly; every list in the result is sorted so
// the output diffs cleanly.
func Reconcile[In any](catalog []Row, checks []Check[In]) Drift {
	var d Drift

	byID := make(map[string]Row, len(catalog))
	for _, r := range catalog {
		if err := r.Validate(); err != nil {
			d.Invalid = append(d.Invalid, "catalog: "+err.Error())
		}
		if _, dup := byID[r.ID]; dup {
			d.DuplicateRows = append(d.DuplicateRows, r.ID)
			continue
		}
		byID[r.ID] = r
	}

	impls := make(map[string]Check[In], len(checks))
	for _, c := range checks {
		if err := c.Validate(); err != nil {
			d.Invalid = append(d.Invalid, "code: "+err.Error())
		}
		if c.Fn == nil {
			d.NoFn = append(d.NoFn, c.ID)
		}
		if _, dup := impls[c.ID]; dup {
			d.DuplicateChecks = append(d.DuplicateChecks, c.ID)
			continue
		}
		impls[c.ID] = c
	}

	for id, row := range byID {
		c, ok := impls[id]
		if !ok {
			d.Unimplemented = append(d.Unimplemented, id)
			continue
		}
		if c.Name != row.Name {
			d.Mismatched = append(d.Mismatched, Mismatch{ID: id, Field: "name", Catalog: row.Name, Impl: c.Name})
		}
		if c.Severity != row.Severity {
			d.Mismatched = append(d.Mismatched, Mismatch{ID: id, Field: "severity", Catalog: string(row.Severity), Impl: string(c.Severity)})
		}
		if c.Means != row.Means {
			d.Mismatched = append(d.Mismatched, Mismatch{ID: id, Field: "means", Catalog: string(row.Means), Impl: string(c.Means)})
		}
		if c.Rejects != row.Rejects {
			d.Mismatched = append(d.Mismatched, Mismatch{ID: id, Field: "rejects", Catalog: row.Rejects, Impl: c.Rejects})
		}
	}
	for id := range impls {
		if _, ok := byID[id]; !ok {
			d.Undeclared = append(d.Undeclared, id)
		}
	}

	sort.Strings(d.Unimplemented)
	sort.Strings(d.Undeclared)
	sort.Strings(d.DuplicateRows)
	sort.Strings(d.DuplicateChecks)
	sort.Strings(d.NoFn)
	sort.Strings(d.Invalid)
	sort.Slice(d.Mismatched, func(i, j int) bool {
		if d.Mismatched[i].ID != d.Mismatched[j].ID {
			return d.Mismatched[i].ID < d.Mismatched[j].ID
		}
		return d.Mismatched[i].Field < d.Mismatched[j].Field
	})
	return d
}

// Report is one run of a set of checks over one input.
type Report struct {
	// Subject names what was audited.
	Subject string
	// Findings is every finding, in check-id order.
	Findings []Finding
	// Ran lists the check ids executed, in order.
	Ran []string
	// Skipped lists the check ids that reported they could not run. A
	// skipped check is in Ran too: it was invoked, it just could not do its
	// job, and its finding says so.
	Skipped []string
}

// Blocking returns the findings that should stop a publish.
func (r Report) Blocking() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity.Blocking() {
			out = append(out, f)
		}
	}
	return out
}

// Passed reports whether nothing blocking was found. A skipped blocking check
// is a blocking finding, so a gate that could not run does not pass.
func (r Report) Passed() bool { return len(r.Blocking()) == 0 }

// String renders the report grouped by severity, most severe first, with the
// skipped count stated.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %d check(s) run, %d skipped, %d finding(s)\n", r.Subject, len(r.Ran), len(r.Skipped), len(r.Findings))
	if len(r.Findings) == 0 {
		b.WriteString("  clean\n")
		return b.String()
	}
	for _, sev := range Severities {
		var group []Finding
		for _, f := range r.Findings {
			if f.Severity == sev {
				group = append(group, f)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", strings.ToUpper(string(sev)), len(group))
		for _, f := range group {
			b.WriteString("  " + f.String() + "\n")
		}
	}
	return b.String()
}

// Run executes checks in id order over in and collects their findings. Every
// finding is stamped with its check's id and, where the check left it empty,
// the check's severity. A check with a nil Fn is skipped, not silently
// passed: it yields a Skipped finding like any check that cannot run.
//
// Order is by id so a report diffs cleanly between runs — an audit whose
// output reorders is an audit nobody diffs.
func Run[In any](subject string, in In, checks []Check[In]) Report {
	sorted := append([]Check[In](nil), checks...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	rep := Report{Subject: subject}
	for _, c := range sorted {
		rep.Ran = append(rep.Ran, c.ID)
		var findings []Finding
		if c.Fn == nil {
			findings = Skipped("no implementation")
		} else {
			findings = c.Fn(in)
		}
		skipped := false
		for _, f := range findings {
			f.CheckID = c.ID
			if f.Severity == "" {
				f.Severity = c.Severity
			}
			if f.Skipped {
				skipped = true
			}
			rep.Findings = append(rep.Findings, f)
		}
		if skipped {
			rep.Skipped = append(rep.Skipped, c.ID)
		}
	}
	return rep
}

// DeterministicShare counts the rows that need no model. Print it in the
// report header so "a model is never the last line of defense" stays a
// measurement rather than a memory.
func DeterministicShare(rows []Row) (deterministic, total int) {
	for _, r := range rows {
		total++
		if r.Means == MeansDeterministic {
			deterministic++
		}
	}
	return deterministic, total
}
