package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// Golden inputSchema JSON, exactly as toolregistry.ObjectSchema emits it.
const (
	addSchemaJSON  = `{"type":"object","properties":{"a":{"type":"integer","description":"first addend"},"b":{"type":"integer","description":"second addend"}},"required":["a","b"]}`
	echoSchemaJSON = `{"type":"object","properties":{"text":{"type":"string","description":"text to echo"}},"required":["text"]}`
	failSchemaJSON = `{"type":"object","properties":{}}`
)

// newTestRegistry builds a registry with two real working tools (add, echo)
// plus one that always errors (fail), so the whole flow — list, call,
// in-band error — runs against genuine registrations.
func newTestRegistry(t *testing.T) *toolregistry.Registry {
	t.Helper()
	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "add",
			Description: "add two integers",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "a", Type: "integer", Description: "first addend", Required: true},
				toolregistry.Prop{Name: "b", Type: "integer", Description: "second addend", Required: true},
			),
		},
		Handler: func(_ context.Context, input json.RawMessage) (string, error) {
			var in struct {
				A int `json:"a"`
				B int `json:"b"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", err
			}
			return strconv.Itoa(in.A + in.B), nil
		},
	})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "echo",
			Description: "echo text back",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "text", Type: "string", Description: "text to echo", Required: true},
			),
		},
		Handler: func(_ context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", err
			}
			return in.Text, nil
		},
	})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "fail",
			Description: "always fails",
			InputSchema: toolregistry.ObjectSchema(),
		},
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", errors.New("boom")
		},
	})
	return reg
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// advancingClock returns a clock that yields start, start+step, start+2*step,
// ... on successive calls. Serve is single-goroutine, so no locking needed.
func advancingClock(start time.Time, step time.Duration) func() time.Time {
	cur := start
	return func() time.Time {
		t := cur
		cur = cur.Add(step)
		return t
	}
}

var testEpoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func newTestServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	if cfg.Name == "" {
		cfg.Name = "demo-tools"
	}
	if cfg.Version == "" {
		cfg.Version = "1.2.3"
	}
	return New(cfg, newTestRegistry(t), fixedClock(testEpoch))
}

// runServe feeds the full input, runs Serve synchronously to EOF, then
// returns the output lines — input and output are never touched concurrently.
func runServe(t *testing.T, s *Server, input string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	raw := out.String()
	if raw == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d response lines, want %d\ngot:  %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

// ─── protocol flow ───────────────────────────────────────────────────────────

func TestFullHandshakeGolden(t *testing.T) {
	s := newTestServer(t, Config{})
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"0.0.1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"add","arguments":{"a":2,"b":3}}}`,
	}, "\n") + "\n"

	want := []string{
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"demo-tools","version":"1.2.3"}}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"tools":[` +
			`{"name":"add","description":"add two integers","inputSchema":` + addSchemaJSON + `},` +
			`{"name":"echo","description":"echo text back","inputSchema":` + echoSchemaJSON + `},` +
			`{"name":"fail","description":"always fails","inputSchema":` + failSchemaJSON + `}]}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"hello"}],"isError":false}}`,
		`{"jsonrpc":"2.0","id":4,"result":{"content":[{"type":"text","text":"5"}],"isError":false}}`,
	}
	assertLines(t, runServe(t, s, input), want)
}

func TestToolsCallInBandErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "handler error becomes isError result",
			input: `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"fail","arguments":{}}}`,
			want:  `{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"tool \"fail\" failed: boom"}],"isError":true}}`,
		},
		{
			name:  "unknown tool becomes isError result, not a protocol error",
			input: `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
			want:  `{"jsonrpc":"2.0","id":12,"result":{"content":[{"type":"text","text":"unknown tool \"nope\""}],"isError":true}}`,
		},
		{
			name:  "absent arguments default to empty object",
			input: `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"echo"}}`,
			want:  `{"jsonrpc":"2.0","id":13,"result":{"content":[{"type":"text","text":""}],"isError":false}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, Config{})
			assertLines(t, runServe(t, s, tt.input+"\n"), []string{tt.want})
		})
	}
}

func TestProtocolErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
		wantID   string // raw JSON of the id in the response
	}{
		{"unknown method", `{"jsonrpc":"2.0","id":5,"method":"frobnicate"}`, ErrCodeNotFound, "5"},
		{"prompts not advertised", `{"jsonrpc":"2.0","id":6,"method":"prompts/list"}`, ErrCodeNotFound, "6"},
		{"resources not advertised", `{"jsonrpc":"2.0","id":7,"method":"resources/list"}`, ErrCodeNotFound, "7"},
		{"malformed json", `{not json`, ErrCodeParse, "null"},
		{"bare scalar is invalid request", `42`, ErrCodeInvalid, "null"},
		{"batch rejected", `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, ErrCodeInvalid, "null"},
		{"initialize bad params", `{"jsonrpc":"2.0","id":8,"method":"initialize","params":42}`, ErrCodeParams, "8"},
		{"tools/call missing params", `{"jsonrpc":"2.0","id":9,"method":"tools/call"}`, ErrCodeParams, "9"},
		{"tools/call params wrong type", `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":"nope"}`, ErrCodeParams, "10"},
		{"tools/call missing name", `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"arguments":{}}}`, ErrCodeParams, "11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, Config{})
			lines := runServe(t, s, tt.input+"\n")
			if len(lines) != 1 {
				t.Fatalf("got %d response lines, want 1: %q", len(lines), lines)
			}
			var resp struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Error   *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
				t.Fatalf("unmarshal response: %v\nline: %s", err, lines[0])
			}
			if resp.Error == nil {
				t.Fatalf("response has no error object: %s", lines[0])
			}
			if resp.Error.Code != tt.wantCode {
				t.Errorf("error code = %d, want %d (message %q)", resp.Error.Code, tt.wantCode, resp.Error.Message)
			}
			if string(resp.ID) != tt.wantID {
				t.Errorf("response id = %s, want %s", resp.ID, tt.wantID)
			}
		})
	}
}

func TestNotificationsGetNoResponse(t *testing.T) {
	s := newTestServer(t, Config{})
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // no id
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`,            // literal null id
		`{"jsonrpc":"2.0","method":"tools/list"}`,                // request-shaped, still no id
		`{"jsonrpc":"2.0","id":11,"method":"ping"}`,              // the only real request
	}, "\n") + "\n"
	assertLines(t, runServe(t, s, input), []string{
		`{"jsonrpc":"2.0","id":11,"result":{}}`,
	})
}

func TestInitializedWithIDGetsBenignResult(t *testing.T) {
	// Defensive path: a client that sends notifications/initialized WITH an
	// id gets an empty result rather than -32601.
	s := newTestServer(t, Config{})
	assertLines(
		t,
		runServe(t, s, `{"jsonrpc":"2.0","id":9,"method":"notifications/initialized"}`+"\n"),
		[]string{`{"jsonrpc":"2.0","id":9,"result":{}}`},
	)
}

func TestIDPassthrough(t *testing.T) {
	s := newTestServer(t, Config{})
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"abc","method":"ping"}`,
		`{"jsonrpc":"2.0","id":42,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":0,"method":"ping"}`,
	}, "\n") + "\n"
	assertLines(t, runServe(t, s, input), []string{
		`{"jsonrpc":"2.0","id":"abc","result":{}}`,
		`{"jsonrpc":"2.0","id":42,"result":{}}`,
		`{"jsonrpc":"2.0","id":0,"result":{}}`,
	})
}

func TestInitializeVersionNegotiation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "2024-11-05 echoed",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			want:  "2024-11-05",
		},
		{
			name:  "2025-03-26 echoed",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
			want:  "2025-03-26",
		},
		{
			name:  "2025-06-18 echoed",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
			want:  "2025-06-18",
		},
		{
			name:  "2025-11-25 echoed (newly supported revision)",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
			want:  "2025-11-25",
		},
		{
			name:  "unknown requested version falls back to latest",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
			want:  latestProtocolVersion,
		},
		{
			name:  "absent params falls back to latest",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
			want:  latestProtocolVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, Config{})
			lines := runServe(t, s, tt.input+"\n")
			if len(lines) != 1 {
				t.Fatalf("got %d response lines, want 1: %q", len(lines), lines)
			}
			var resp struct {
				Result struct {
					ProtocolVersion string `json:"protocolVersion"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if resp.Result.ProtocolVersion != tt.want {
				t.Errorf("protocolVersion = %q, want %q", resp.Result.ProtocolVersion, tt.want)
			}
		})
	}
}

// ─── input bounds ────────────────────────────────────────────────────────────

func TestOversizedMessages(t *testing.T) {
	t.Run("id recovered from prefix, then resync", func(t *testing.T) {
		s := newTestServer(t, Config{MaxMessageBytes: 96})
		big := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"echo","arguments":{"text":"` +
			strings.Repeat("x", 300) + `"}}}`
		input := big + "\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
		assertLines(t, runServe(t, s, input), []string{
			`{"jsonrpc":"2.0","id":42,"error":{"code":-32700,"message":"message exceeds maximum size of 96 bytes"}}`,
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
		})
	})

	t.Run("id past truncation point falls back to null", func(t *testing.T) {
		s := newTestServer(t, Config{MaxMessageBytes: 96})
		big := `{"jsonrpc":"2.0","blob":"` + strings.Repeat("x", 300) + `","id":7,"method":"ping"}`
		input := big + "\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
		assertLines(t, runServe(t, s, input), []string{
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"message exceeds maximum size of 96 bytes"}}`,
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
		})
	})

	t.Run("resync across a line larger than the read buffer", func(t *testing.T) {
		// 40000 bytes > readBufSize (32 KiB) forces the explicit
		// discard-to-newline path rather than the single-fragment path.
		s := newTestServer(t, Config{MaxMessageBytes: 1024})
		big := `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"echo","arguments":{"text":"` +
			strings.Repeat("x", 40000) + `"}}}`
		input := big + "\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
		assertLines(t, runServe(t, s, input), []string{
			`{"jsonrpc":"2.0","id":13,"error":{"code":-32700,"message":"message exceeds maximum size of 1024 bytes"}}`,
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
		})
	})

	t.Run("oversized final line at EOF still gets a response", func(t *testing.T) {
		s := newTestServer(t, Config{MaxMessageBytes: 96})
		big := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"` +
			strings.Repeat("x", 300) + `"}}}` // no trailing newline
		assertLines(t, runServe(t, s, big), []string{
			`{"jsonrpc":"2.0","id":3,"error":{"code":-32700,"message":"message exceeds maximum size of 96 bytes"}}`,
		})
	})

	t.Run("message of exactly the maximum size is not rejected", func(t *testing.T) {
		s := newTestServer(t, Config{MaxMessageBytes: 40})
		line := `{"jsonrpc":"2.0","id":1,"method":"ping"}` // exactly 40 bytes
		if len(line) != 40 {
			t.Fatalf("test fixture is %d bytes, want 40", len(line))
		}
		assertLines(t, runServe(t, s, line+"\n"), []string{
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
		})
	})
}

func TestBestEffortID(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"number id before truncation", `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"e`, "42"},
		{"string id before truncation", `{"jsonrpc":"2.0","id":"abc","params":{"x":"trunc`, `"abc"`},
		{"negative number id", `{"jsonrpc":"2.0","id":-7,"method":"x"}`, "-7"},
		{"id past truncation", `{"jsonrpc":"2.0","params":{"blob":"xxxxxxxxxxxxxxxxxxxx`, "null"},
		{"null id", `{"id":null,"method":"x"}`, "null"},
		{"object id not echoable", `{"id":{"a":1},"method":"x"}`, "null"},
		{"no id key", `{"jsonrpc":"2.0","method":"ping"}`, "null"},
		{"not an object", `[1,2,3`, "null"},
		{"garbage", `garbage`, "null"},
		{"empty", ``, "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(bestEffortID([]byte(tt.prefix))); got != tt.want {
				t.Errorf("bestEffortID(%q) = %s, want %s", tt.prefix, got, tt.want)
			}
		})
	}
}

// ─── framing tolerance ───────────────────────────────────────────────────────

func TestCRLFAndBlankLinesTolerated(t *testing.T) {
	s := newTestServer(t, Config{})
	input := "\r\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\r\n" +
		"\n" +
		"   \n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\"}\n"
	assertLines(t, runServe(t, s, input), []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{}}`,
	})
}

func TestUnterminatedFinalLineIsProcessed(t *testing.T) {
	s := newTestServer(t, Config{})
	assertLines(
		t,
		runServe(t, s, `{"jsonrpc":"2.0","id":1,"method":"ping"}`), // no trailing \n
		[]string{`{"jsonrpc":"2.0","id":1,"result":{}}`},
	)
}

// ─── lifecycle ───────────────────────────────────────────────────────────────

func TestServeContextCancel(t *testing.T) {
	s := newTestServer(t, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	err := s.Serve(ctx, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve returned %v, want context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Errorf("cancelled Serve wrote output: %q", out.String())
	}
}

func TestNewPanicsOnNilRegistry(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with nil registry did not panic")
		}
	}()
	New(Config{Name: "x", Version: "y"}, nil, nil)
}

// ─── call log ────────────────────────────────────────────────────────────────

func TestCallLogGolden(t *testing.T) {
	var log bytes.Buffer
	cfg := Config{Name: "demo-tools", Version: "1.2.3", CallLog: &log}
	s := New(cfg, newTestRegistry(t), advancingClock(testEpoch, 5*time.Millisecond))

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // notification: not logged
		`{not json`, // pre-dispatch parse error: not logged
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"secret-user-data"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"fail","arguments":{}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	want := []string{
		`{"ts":"2026-01-02T03:04:05Z","method":"initialize","duration_ms":5,"is_error":false}`,
		`{"ts":"2026-01-02T03:04:05.01Z","method":"tools/list","duration_ms":5,"is_error":false}`,
		`{"ts":"2026-01-02T03:04:05.02Z","method":"tools/call","tool":"echo","duration_ms":5,"is_error":false}`,
		`{"ts":"2026-01-02T03:04:05.03Z","method":"tools/call","tool":"fail","duration_ms":5,"is_error":true}`,
	}
	got := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
	assertLines(t, got, want)

	// The privacy contract: payloads never reach the log by default.
	if strings.Contains(log.String(), "secret-user-data") {
		t.Error("call log contains tool arguments without LogPayloads opt-in")
	}
}

func TestCallLogWithPayloads(t *testing.T) {
	var log bytes.Buffer
	cfg := Config{Name: "demo-tools", Version: "1.2.3", CallLog: &log, LogPayloads: true}
	s := New(cfg, newTestRegistry(t), advancingClock(testEpoch, 5*time.Millisecond))

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fail","arguments":{}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	want := []string{
		`{"ts":"2026-01-02T03:04:05Z","method":"tools/call","tool":"echo","duration_ms":5,"is_error":false,"arguments":{"text":"hi"},"result":"hi"}`,
		`{"ts":"2026-01-02T03:04:05.01Z","method":"tools/call","tool":"fail","duration_ms":5,"is_error":true,"arguments":{},"result":"tool \"fail\" failed: boom"}`,
	}
	got := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
	assertLines(t, got, want)
}
