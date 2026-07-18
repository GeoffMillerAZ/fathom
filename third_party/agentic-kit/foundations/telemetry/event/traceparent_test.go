package event_test

import (
	"strings"
	"testing"

	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// validTC returns a well-formed wire-representable TraceContext.
func validTC() ev.TraceContext {
	return ev.TraceContext{
		RunID:  "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID: "00f067aa0ba902b7",
	}
}

// ---- Constants ------------------------------------------------------------------

func TestTraceparentConstants(t *testing.T) {
	if ev.TraceparentHeader != "traceparent" {
		t.Errorf("TraceparentHeader = %q, want %q", ev.TraceparentHeader, "traceparent")
	}
	if ev.TraceparentEnv != "TRACEPARENT" {
		t.Errorf("TraceparentEnv = %q, want %q", ev.TraceparentEnv, "TRACEPARENT")
	}
}

// ---- FormatTraceparent ----------------------------------------------------------

func TestFormatTraceparent(t *testing.T) {
	tests := []struct {
		name   string
		tc     ev.TraceContext
		want   string
		wantOK bool
	}{
		{
			name:   "valid ids",
			tc:     validTC(),
			want:   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantOK: true,
		},
		{
			name: "parent span id is not encoded",
			tc: ev.TraceContext{
				RunID:        "4bf92f3577b34da6a3ce929d0e0e4736",
				SpanID:       "00f067aa0ba902b7",
				ParentSpanID: "b7ad6b7169203331",
			},
			want:   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantOK: true,
		},
		{name: "zero trace context", tc: ev.TraceContext{}, wantOK: false},
		{
			name:   "all-zero run id never emitted",
			tc:     ev.TraceContext{RunID: strings.Repeat("0", 32), SpanID: "00f067aa0ba902b7"},
			wantOK: false,
		},
		{
			name:   "all-zero span id never emitted",
			tc:     ev.TraceContext{RunID: "4bf92f3577b34da6a3ce929d0e0e4736", SpanID: strings.Repeat("0", 16)},
			wantOK: false,
		},
		{
			name:   "run id too short",
			tc:     ev.TraceContext{RunID: "4bf92f", SpanID: "00f067aa0ba902b7"},
			wantOK: false,
		},
		{
			name:   "span id too long",
			tc:     ev.TraceContext{RunID: "4bf92f3577b34da6a3ce929d0e0e4736", SpanID: "00f067aa0ba902b7ff"},
			wantOK: false,
		},
		{
			name:   "uppercase hex rejected",
			tc:     ev.TraceContext{RunID: "4BF92F3577B34DA6A3CE929D0E0E4736", SpanID: "00f067aa0ba902b7"},
			wantOK: false,
		},
		{
			name:   "non-hex run id",
			tc:     ev.TraceContext{RunID: "zzf92f3577b34da6a3ce929d0e0e4736", SpanID: "00f067aa0ba902b7"},
			wantOK: false,
		},
		{
			name:   "missing span id",
			tc:     ev.TraceContext{RunID: "4bf92f3577b34da6a3ce929d0e0e4736"},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ev.FormatTraceparent(tt.tc)
			if ok != tt.wantOK {
				t.Fatalf("FormatTraceparent(%+v) ok = %v, want %v", tt.tc, ok, tt.wantOK)
			}
			if !tt.wantOK && got != "" {
				t.Errorf("not-ok format must return empty string; got %q", got)
			}
			if tt.wantOK && got != tt.want {
				t.Errorf("FormatTraceparent = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- ParseTraceparent -----------------------------------------------------------

func TestParseTraceparent(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    ev.TraceContext
		wantErr bool
	}{
		{
			name: "valid version 00",
			in:   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			want: validTC(),
		},
		{
			name: "unsampled flags accepted",
			in:   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
			want: validTC(),
		},
		{
			name: "unknown future version parsed by 00 layout",
			in:   "cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			want: validTC(),
		},
		{
			name: "future version with dash-separated extra data",
			in:   "cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra-fields",
			want: validTC(),
		},
		{
			name:    "future version extra data without dash separator",
			in:      "cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01extra",
			wantErr: true,
		},
		{
			name:    "version ff rejected",
			in:      "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantErr: true,
		},
		{
			name:    "version 00 with trailing data rejected",
			in:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
			wantErr: true,
		},
		{
			name:    "all-zero trace-id rejected",
			in:      "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
			wantErr: true,
		},
		{
			name:    "all-zero parent-id rejected",
			in:      "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
			wantErr: true,
		},
		{
			name:    "uppercase trace-id rejected",
			in:      "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
			wantErr: true,
		},
		{
			name:    "non-hex version rejected",
			in:      "zz-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantErr: true,
		},
		{
			name:    "non-hex flags rejected",
			in:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-zz",
			wantErr: true,
		},
		{
			name:    "wrong separator rejected",
			in:      "00_4bf92f3577b34da6a3ce929d0e0e4736_00f067aa0ba902b7_01",
			wantErr: true,
		},
		{
			name:    "trace-id too short rejected",
			in:      "00-4bf92f3577b34da6a3ce929d0e0e47-00f067aa0ba902b7-01",
			wantErr: true,
		},
		{name: "empty string", in: "", wantErr: true},
		{name: "garbage", in: "not-a-traceparent", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ev.ParseTraceparent(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTraceparent(%q) = %+v, want error", tt.in, got)
				}
				if !got.IsZero() {
					t.Errorf("fail-closed violated: error path returned non-zero TraceContext %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTraceparent(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseTraceparent(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
			if got.ParentSpanID != "" {
				t.Errorf("parsed ParentSpanID must be empty (does not cross the wire); got %q", got.ParentSpanID)
			}
		})
	}
}

// ---- Round trip -----------------------------------------------------------------

// TestTraceparentRoundTrip proves Format→Parse recovers exactly
// {RunID, SpanID} for wire-representable contexts, including fresh ones
// from both deterministic and crypto/rand sources. ParentSpanID never
// survives the trip — receivers derive it via Child.
func TestTraceparentRoundTrip(t *testing.T) {
	rh := fixedRandHex()
	contexts := []ev.TraceContext{
		validTC(),
		ev.NewTrace(rh),
		ev.NewTrace(rh).Child(rh),   // ParentSpanID set: must not survive
		ev.NewTrace(nil),            // crypto/rand
		ev.NewTrace(nil).Child(nil), // crypto/rand child
		{RunID: strings.Repeat("f", 32), SpanID: strings.Repeat("1", 16)},
	}
	for _, tc := range contexts {
		s, ok := ev.FormatTraceparent(tc)
		if !ok {
			t.Fatalf("FormatTraceparent(%+v) not ok for a wire-representable context", tc)
		}
		got, err := ev.ParseTraceparent(s)
		if err != nil {
			t.Fatalf("ParseTraceparent(%q): %v", s, err)
		}
		want := ev.TraceContext{RunID: tc.RunID, SpanID: tc.SpanID}
		if got != want {
			t.Errorf("round trip of %+v via %q = %+v, want %+v", tc, s, got, want)
		}
	}
}

// TestTraceparentRoundTripChildLinksToSender documents the receiver-side
// idiom: Child on the parsed context yields a span whose ParentSpanID is
// the sender's SpanID under the sender's RunID.
func TestTraceparentRoundTripChildLinksToSender(t *testing.T) {
	rh := fixedRandHex()
	sender := ev.NewTrace(rh)
	s, ok := ev.FormatTraceparent(sender)
	if !ok {
		t.Fatal("FormatTraceparent not ok")
	}
	parsed, err := ev.ParseTraceparent(s)
	if err != nil {
		t.Fatal(err)
	}
	receiver := parsed.Child(rh)
	if receiver.RunID != sender.RunID {
		t.Errorf("receiver RunID = %q, want sender's %q", receiver.RunID, sender.RunID)
	}
	if receiver.ParentSpanID != sender.SpanID {
		t.Errorf("receiver ParentSpanID = %q, want sender SpanID %q", receiver.ParentSpanID, sender.SpanID)
	}
	if receiver.SpanID == sender.SpanID {
		t.Error("receiver must mint a fresh SpanID")
	}
}

// ---- AppendTraceparent ----------------------------------------------------------

func TestAppendTraceparent(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/tmp"}

	t.Run("valid context appends one entry", func(t *testing.T) {
		env := ev.AppendTraceparent(append([]string(nil), base...), validTC())
		if len(env) != len(base)+1 {
			t.Fatalf("len(env) = %d, want %d: %q", len(env), len(base)+1, env)
		}
		want := "TRACEPARENT=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		if env[len(env)-1] != want {
			t.Errorf("appended entry = %q, want %q", env[len(env)-1], want)
		}
	})

	t.Run("zero context returns env unchanged", func(t *testing.T) {
		in := append([]string(nil), base...)
		out := ev.AppendTraceparent(in, ev.TraceContext{})
		if len(out) != len(in) {
			t.Fatalf("env changed for zero context: %q", out)
		}
		for i := range in {
			if out[i] != in[i] {
				t.Errorf("env[%d] = %q, want %q", i, out[i], in[i])
			}
		}
	})

	t.Run("malformed context returns env unchanged", func(t *testing.T) {
		out := ev.AppendTraceparent(base, ev.TraceContext{RunID: "short", SpanID: "also-bad"})
		if len(out) != len(base) {
			t.Fatalf("env changed for malformed context: %q", out)
		}
	})

	t.Run("nil env with valid context yields single entry", func(t *testing.T) {
		out := ev.AppendTraceparent(nil, validTC())
		if len(out) != 1 || !strings.HasPrefix(out[0], "TRACEPARENT=") {
			t.Fatalf("env = %q, want single TRACEPARENT entry", out)
		}
	})
}

// ---- TraceFromEnviron -----------------------------------------------------------

func TestTraceFromEnviron(t *testing.T) {
	valid := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	other := "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab-bbbbbbbbbbbbbbbb-01"

	tests := []struct {
		name    string
		environ []string
		want    ev.TraceContext
		wantOK  bool
	}{
		{
			name:    "present",
			environ: []string{"PATH=/usr/bin", "TRACEPARENT=" + valid},
			want:    validTC(),
			wantOK:  true,
		},
		{name: "absent", environ: []string{"PATH=/usr/bin"}, wantOK: false},
		{name: "nil environ", environ: nil, wantOK: false},
		{
			name:    "last wins on duplicates",
			environ: []string{"TRACEPARENT=" + other, "TRACEPARENT=" + valid},
			want:    validTC(),
			wantOK:  true,
		},
		{
			name:    "later malformed entry fails closed despite earlier valid one",
			environ: []string{"TRACEPARENT=" + valid, "TRACEPARENT=garbage"},
			wantOK:  false,
		},
		{
			name:    "malformed value",
			environ: []string{"TRACEPARENT=not-a-traceparent"},
			wantOK:  false,
		},
		{
			name:    "empty value",
			environ: []string{"TRACEPARENT="},
			wantOK:  false,
		},
		{
			name:    "similarly named variable does not match",
			environ: []string{"TRACEPARENT_BACKUP=" + valid, "XTRACEPARENT=" + valid},
			wantOK:  false,
		},
		{
			name:    "lowercase name does not match (case-sensitive)",
			environ: []string{"traceparent=" + valid},
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ev.TraceFromEnviron(tt.environ)
			if ok != tt.wantOK {
				t.Fatalf("TraceFromEnviron(%q) ok = %v, want %v (got %+v)", tt.environ, ok, tt.wantOK, got)
			}
			if !tt.wantOK {
				if !got.IsZero() {
					t.Errorf("fail-closed violated: not-ok returned non-zero %+v", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("TraceFromEnviron = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---- env carrier composes with the header carrier -------------------------------

func TestAppendTraceparentThenTraceFromEnvironRoundTrip(t *testing.T) {
	sender := validTC()
	env := ev.AppendTraceparent([]string{"AK_BASE=1"}, sender)
	got, ok := ev.TraceFromEnviron(env)
	if !ok {
		t.Fatal("TraceFromEnviron must recover the appended entry")
	}
	want := ev.TraceContext{RunID: sender.RunID, SpanID: sender.SpanID}
	if got != want {
		t.Errorf("env round trip = %+v, want %+v", got, want)
	}
}
