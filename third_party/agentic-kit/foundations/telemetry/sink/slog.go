package sink

import (
	"context"
	"log/slog"
	"sort"

	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// SlogSink bridges telemetry events into a *slog.Logger, letting existing
// structured-logging pipelines (JSON handlers, log shippers) carry telemetry
// without a dedicated backend.
//
// Every event is logged with message "telemetry_event" and the fixed
// top-level attrs event_id, kind, source, and timestamp (the event's own
// timestamp — handlers add their record time separately). Event Attributes,
// including the reserved trace.* correlation keys, are nested under an
// "attributes" group with keys emitted in sorted order so handler output is
// deterministic. Kind maps to level: event.KindError logs at
// slog.LevelError, everything else at slog.LevelInfo.
//
// SlogSink is safe for concurrent use to the same degree as the wrapped
// *slog.Logger (slog loggers are concurrency-safe by contract).
type SlogSink struct {
	logger *slog.Logger
}

// NewSlogSink returns a SlogSink writing to logger. Pass nil to use
// slog.Default().
func NewSlogSink(logger *slog.Logger) *SlogSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogSink{logger: logger}
}

// Emit logs e on the wrapped logger. It always returns nil: slog handlers do
// not surface write errors to callers, so there is nothing to propagate.
func (s *SlogSink) Emit(e ev.Event) error {
	level := slog.LevelInfo
	if e.Kind == ev.KindError {
		level = slog.LevelError
	}

	attrs := []slog.Attr{
		slog.String("event_id", e.ID),
		slog.String("kind", string(e.Kind)),
		slog.String("source", e.Source),
		slog.Time("timestamp", e.Timestamp),
	}
	if len(e.Attributes) > 0 {
		keys := make([]string, 0, len(e.Attributes))
		for k := range e.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		group := make([]any, 0, len(keys))
		for _, k := range keys {
			group = append(group, slog.String(k, e.Attributes[k]))
		}
		attrs = append(attrs, slog.Group("attributes", group...))
	}

	s.logger.LogAttrs(context.Background(), level, "telemetry_event", attrs...)
	return nil
}
