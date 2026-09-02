// Package widgetgizmo is a scaffolded kit package: replace this sentence with
// what it does, in one line, because it becomes the docs/catalog.md synopsis.
//
// Shape contract: schema.cue is authoritative; the Go types in types.go
// mirror it, and any drift between the two is a bug (CUE owns the shape,
// Go owns the behavior).
//
// Determinism: the clock is injected (New takes a now func() time.Time);
// this package never reads wall time on its own and never sleeps.
//
// Concurrency: a *WidgetGizmo holds only state fixed at New and is safe for
// concurrent use; Config.Validate and every other function here is pure and
// needs no synchronization. Revise this paragraph the moment mutable state
// is added — the concurrency contract is a gate check, not decoration.
//
// Non-guarantees (state them as plainly as the guarantees — VISION belief 6):
//   - it persists nothing and survives no process restart;
//   - it enforces no bound beyond what Config.Validate reports;
//   - it is not a security boundary of any kind.
//
// Verified against: go 1.26, cue v0.16.1 (at scaffold time; re-run task validate and update this line on every toolchain bump).
package widgetgizmo
