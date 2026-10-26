package scenariobench

// NullHarness makes no edits and returns no answer, providing the dry-run
// baseline that every kept task must fail under its existing grader.
type NullHarness struct{}

// NewNullHarness returns a stateless, spend-free harness.
func NewNullHarness() *NullHarness { return &NullHarness{} }

func (NullHarness) Name() string    { return "null" }
func (NullHarness) Version() string { return "null-1" }

func (NullHarness) RunSession(req SessionRequest) (SessionResult, error) {
	return SessionResult{Index: req.Index, Role: req.Role, RowID: req.RowID}, nil
}

var _ Harness = NullHarness{}
