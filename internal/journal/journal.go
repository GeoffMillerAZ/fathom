// Package journal provides a write path from fathom findings into an
// agentic-kit investigation journal.
package journal

import (
	"fmt"
	"time"

	ij "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
)

// Finding is a single fathom finding to be recorded in a journal.
type Finding struct {
	Step string
	Note string
}

// RecordFindings appends each Finding in findings to j as a journal Entry.
// The At timestamp is set to time.Now() for each entry so that entries remain
// ordered even when findings is non-empty.
//
// It returns the first error encountered; on error the journal may have been
// partially written (entries appended before the failing one are retained).
func RecordFindings(j *ij.Journal, findings []Finding) error {
	for i, f := range findings {
		err := j.Append(ij.Entry{
			Step: f.Step,
			Note: f.Note,
			At:   time.Now(),
		})
		if err != nil {
			return fmt.Errorf("journal: finding[%d] (%q): %w", i, f.Step, err)
		}
	}
	return nil
}
