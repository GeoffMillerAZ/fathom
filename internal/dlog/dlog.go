// Package dlog is fathom's leveled, timestamped daemon logger.
//
// Before this package, every daemon diagnostic line was a bare
// fmt.Fprintf(stderr/stdout, ...) with no timestamp and no level (confirmed:
// zero hits for log.New(/log.SetFlags/log/slog outside third_party's
// unrelated telemetry bridge). That made every daemon log line
// unattributable to wall-clock time, forcing incident review to fall back
// to file line order and cross-reference a separately-timestamped
// mcp-calls.jsonl (diagnostics sweep DG-1).
//
// Logger wraps log/slog with a text handler over an injected io.Writer and
// an injected clock (internal/AGENTS.md's determinism contract: never call
// time.Now() directly in logging/analysis logic — inject it). Operational
// logs are not part of fathom's deterministic-OUTPUT contract (two runs may
// legitimately log at different wall-clock times), but tests still inject
// the clock so assertions on emitted lines are not flaky.
package dlog

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Level is a log severity, numerically compatible with log/slog.Level so
// LevelFromEnv's result can be handed straight to slog.HandlerOptions.
type Level int

const (
	LevelDebug Level = Level(slog.LevelDebug)
	LevelInfo  Level = Level(slog.LevelInfo)
	LevelWarn  Level = Level(slog.LevelWarn)
	LevelError Level = Level(slog.LevelError)
)

func (l Level) slog() slog.Level { return slog.Level(l) }

// String names the level the way FATHOM_LOG_LEVEL spells it.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// LevelFromEnv resolves the daemon's log level from FATHOM_LOG_LEVEL
// (debug/info/warn/error, case-insensitive, via the injected getenv — never
// os.Getenv directly, so callers can test this without t.Setenv).
//
// Unset or unrecognized values default to LevelInfo. This is never a
// startup error — an operational tuning knob, matching the existing
// FATHOM_SCAVENGE_INTERVAL convention (cmd/fathom/memscavenge.go).
func LevelFromEnv(getenv func(string) string) Level {
	switch strings.ToLower(strings.TrimSpace(getenv("FATHOM_LOG_LEVEL"))) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger is a leveled, timestamped logger over an injected io.Writer and
// clock. The zero value is not usable; construct with New.
type Logger struct {
	handler slog.Handler
	now     func() time.Time

	dedupe *dedupeState

	// writeMu is the SAME lock lockedWriter takes around every Write to
	// the underlying io.Writer (see New and WithWriteLock's doc comment).
	writeMu *sync.Mutex
}

// lockedWriter serializes every Write through mu — installed as every
// Logger's underlying writer so WithWriteLock can hand an external caller
// that same lock (see below).
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

// New constructs a Logger writing slog's text format (RFC3339 timestamps,
// level, message, key=value attrs) to w, filtered to level and above, with
// now supplying every record's timestamp (injected, per the determinism
// contract — production callers pass time.Now, tests pass a fixed clock).
func New(w io.Writer, level Level, now func() time.Time) *Logger {
	mu := &sync.Mutex{}
	h := slog.NewTextHandler(&lockedWriter{mu: mu, w: w}, &slog.HandlerOptions{
		Level: level.slog(),
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.UTC().Format(time.RFC3339))
				}
			}
			return a
		},
	})
	return &Logger{handler: h, now: now, dedupe: newDedupeState(), writeMu: mu}
}

// WithWriteLock runs fn while holding the SAME lock this Logger uses to
// serialize its own writes to the underlying io.Writer. A caller that must
// manipulate the writer's underlying file directly — log rotation's
// Truncate+Seek, in particular — takes this lock around that critical
// section so it can never interleave with a concurrent Debug/Info/Warn/
// Error call's Write landing between the Truncate and the Seek (which
// would otherwise leave a sparse hole of NUL bytes: finding [1]/DG-6+DG-1).
func (l *Logger) WithWriteLock(fn func()) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	fn()
}

func (l *Logger) emit(level Level, msg string, args ...any) {
	l.emitAt(l.now(), level, msg, args...)
}

// emitAt is emit with an explicit timestamp, bypassing the injected clock —
// FlushDedupe's caller supplies "now" as a parameter rather than through
// l.now, and this keeps that path from racily swapping l.now under
// concurrent Dedupe/Info/etc. calls.
func (l *Logger) emitAt(t time.Time, level Level, msg string, args ...any) {
	sl := level.slog()
	if !l.handler.Enabled(context.Background(), sl) {
		return
	}
	r := slog.NewRecord(t, sl, msg, 0)
	r.Add(args...)
	_ = l.handler.Handle(context.Background(), r)
}

// Debug logs msg at LevelDebug with slog key/value args.
func (l *Logger) Debug(msg string, args ...any) { l.emit(LevelDebug, msg, args...) }

// Info logs msg at LevelInfo with slog key/value args.
func (l *Logger) Info(msg string, args ...any) { l.emit(LevelInfo, msg, args...) }

// Warn logs msg at LevelWarn with slog key/value args.
func (l *Logger) Warn(msg string, args ...any) { l.emit(LevelWarn, msg, args...) }

// Error logs msg at LevelError with slog key/value args.
func (l *Logger) Error(msg string, args ...any) { l.emit(LevelError, msg, args...) }
