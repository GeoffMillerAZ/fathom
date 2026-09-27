package tenancy

// producer.go — the producer governor (design §2.1, RT-153). The ladder
// publishes its level in one atomic; every byte producer — the connector's
// Fetch, (*Client).Sources, corpus reads, store row decoding every 1,000
// rows, the ingest fetch workers — reads it before each unit of work and
// stops with ErrOverBudget when ProducerAllows says no: tree, fleet and
// ingest producers at refuse, every producer at the exit mark. A runaway
// loop therefore halts within one file or 1,000 rows of the rung,
// independent of sampling latency. This is what bounds memory in waves
// A–B, before per-call Charge exists in most producers.

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrOverBudget is the producer governor's (and later the per-call
// meter's) refusal. Callers return what they have, named as partial (§3.4).
var ErrOverBudget = errors.New("over budget")

// overBudgetErrs are preallocated, so a refused producer does not allocate.
var overBudgetErrs = func() (out [numLevels][numClasses]error) {
	for l := range numLevels {
		for c := range numClasses {
			out[l][c] = &OverBudgetError{Level: Level(l), Class: Class(c)}
		}
	}
	return out
}()

// OverBudgetError names the rung and the producer's class.
type OverBudgetError struct {
	Level Level
	Class Class
}

func (e *OverBudgetError) Error() string {
	return "over budget: " + e.Class.String() + " producer halted at " + e.Level.String() + " (fathom resources)"
}

func (e *OverBudgetError) Is(target error) bool { return target == ErrOverBudget }

var currentLevel atomic.Uint32

// PublishLevel sets the process-wide ladder level the producers read. The
// governor loop is its one writer in a daemon; tests restore LevelNormal.
func PublishLevel(l Level) { currentLevel.Store(uint32(l)) }

// CurrentLevel is the published ladder level.
func CurrentLevel() Level { return Level(currentLevel.Load()) }

// ProducerAllows reports whether a producer of class c may do its next
// unit of work: nil, or an *OverBudgetError (errors.Is ErrOverBudget).
func ProducerAllows(c Class) error {
	l := CurrentLevel()
	switch {
	case l >= LevelExit:
	case l >= LevelRefuse && (c == ClassTree || c == ClassFleet || c == ClassIngest):
	default:
		return nil
	}
	if l >= numLevels || c >= numClasses {
		return ErrOverBudget
	}
	return overBudgetErrs[l][c]
}

type producerClassKey struct{}

// WithProducerClass marks ctx's byte producers as class c (the request's
// cost class, or ClassIngest for an ingest).
func WithProducerClass(ctx context.Context, c Class) context.Context {
	return context.WithValue(ctx, producerClassKey{}, c)
}

// ProducerClassOf is ctx's producer class, or def when none was marked.
func ProducerClassOf(ctx context.Context, def Class) Class {
	if c, ok := ctx.Value(producerClassKey{}).(Class); ok {
		return c
	}
	return def
}

// ProducerAllowsCtx is ProducerAllows for ctx's class (def when unmarked).
func ProducerAllowsCtx(ctx context.Context, def Class) error {
	if CurrentLevel() == LevelNormal {
		return nil
	}
	return ProducerAllows(ProducerClassOf(ctx, def))
}

// RowGovernorEvery is how many decoded rows a store scan reads between
// checks.
const RowGovernorEvery = 1000

// RowGovernor checks the ladder every RowGovernorEvery rows of a scan.
type RowGovernor struct {
	ctx context.Context
	def Class
	n   int
}

// NewRowGovernor governs one scan under ctx (def: the class when ctx names
// none).
func NewRowGovernor(ctx context.Context, def Class) RowGovernor {
	return RowGovernor{ctx: ctx, def: def}
}

// Row counts one decoded row; every RowGovernorEvery rows it returns the
// producer governor's verdict.
func (g *RowGovernor) Row() error {
	g.n++
	if g.n%RowGovernorEvery != 0 {
		return nil
	}
	return ProducerAllowsCtx(g.ctx, g.def)
}
