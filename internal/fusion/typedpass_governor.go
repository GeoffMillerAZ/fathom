package fusion

// typedpass_governor.go — RT-178 (the wave-B live reading's F1,
// docs/bench/readings/rt-wave-b-seatbelt-2026-09-27.md §5): the in-process
// typed pass is an ingest-class producer under the producer governor
// (RT-153, design §2.1). Before this, the governor was asked by the
// connector's Fetch, Sources and the corpus reads but never inside
// go/packages, so an ingest admitted before the ladder climbed type-checked
// for 10 minutes at refuse and was stopped only by the exit mark.
//
// The pass now asks the governor (1) before it starts, (2) in a governed
// ParseFile before every file go/packages parses, (3) from a watcher that
// polls the published level every typedPassGovernorPoll while `go list`
// or a type-check runs, and (4) before every package of the store loop.
// A refusal cancels the load's context with the *tenancy.OverBudgetError as
// its cause: go/packages stops parsing and type-checking every package not
// yet started (it checks ld.Context.Err() before each), and its `go list`
// child is interrupted and then killed by the go command runner, which
// runs it under that context. The pass returns the named error; the Ingest
// around it fails before its final commit, so the ingest stays in_progress
// and the next start resumes it (RT-109).

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// typedPassGovernorPoll is how often the watcher reads the ladder while
// go/packages holds the thread (`go list`, one package's type-check).
const typedPassGovernorPoll = 50 * time.Millisecond

// Test hooks: called before each governed parse / each stored package.
var (
	onTypedPassParse   func(filename string)
	onTypedPassPackage func(pkgID string)
)

// typedPassGovernor is one pass's governed context.
type typedPassGovernor struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	stop   chan struct{}
	done   chan struct{}
}

// governTypedPass derives the pass's context from parent and starts the
// watcher. close must be called when the pass returns.
func governTypedPass(parent context.Context) *typedPassGovernor {
	ctx, cancel := context.WithCancelCause(parent)
	g := &typedPassGovernor{ctx: ctx, cancel: cancel, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(g.done)
		t := time.NewTicker(typedPassGovernorPoll)
		defer t.Stop()
		for {
			select {
			case <-g.stop:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				_ = g.check()
			}
		}
	}()
	return g
}

// check is the governor's verdict for the pass's next unit: nil, or the
// cause that stopped it (the *tenancy.OverBudgetError, or the parent's
// cancellation). A refusal cancels the pass's context.
func (g *typedPassGovernor) check() error {
	if g.ctx.Err() != nil {
		return context.Cause(g.ctx)
	}
	if err := tenancy.ProducerAllowsCtx(g.ctx, tenancy.ClassIngest); err != nil {
		g.cancel(err)
		return err
	}
	return nil
}

// parseFile is go/packages' default ParseFile, asked of the governor first.
func (g *typedPassGovernor) parseFile(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if onTypedPassParse != nil {
		onTypedPassParse(filename)
	}
	if err := g.check(); err != nil {
		return nil, err
	}
	const mode = parser.AllErrors | parser.ParseComments
	return parser.ParseFile(fset, filename, src, mode)
}

func (g *typedPassGovernor) close() {
	close(g.stop)
	<-g.done
	g.cancel(nil)
}
