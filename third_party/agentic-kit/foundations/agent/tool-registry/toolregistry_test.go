package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	toolexecutionpolicy "github.com/geoffmilleraz/agentic-kit/foundations/environment/tool-execution-policy"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// echoTool returns a Tool named name whose handler echoes its raw input
// prefixed with "echo:".
func echoTool(name string) Tool {
	return Tool{
		Def: llmclient.ToolDef{
			Name:        name,
			Description: "echoes its input",
			InputSchema: ObjectSchema(Prop{Name: "text", Type: "string", Required: true}),
		},
		Handler: func(_ context.Context, input json.RawMessage) (string, error) {
			return "echo:" + string(input), nil
		},
	}
}

// waitTool returns a Tool whose handler cooperatively blocks until its
// context is done, then returns ctx.Err() — the canonical well-behaved
// long-running handler.
func waitTool(name string, timeout time.Duration) Tool {
	return Tool{
		Def:     llmclient.ToolDef{Name: name, Description: "waits for ctx"},
		Timeout: timeout,
		Handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
}

func TestRegister(t *testing.T) {
	nopHandler := func(context.Context, json.RawMessage) (string, error) { return "", nil }

	tests := []struct {
		name    string
		tool    Tool
		wantErr string // substring; empty means success
	}{
		{
			name: "valid tool registers",
			tool: Tool{Def: llmclient.ToolDef{Name: "ok"}, Handler: nopHandler},
		},
		{
			name:    "empty name rejected",
			tool:    Tool{Def: llmclient.ToolDef{Name: ""}, Handler: nopHandler},
			wantErr: "name must be non-empty",
		},
		{
			name:    "whitespace name rejected",
			tool:    Tool{Def: llmclient.ToolDef{Name: "   "}, Handler: nopHandler},
			wantErr: "name must be non-empty",
		},
		{
			name:    "nil handler rejected",
			tool:    Tool{Def: llmclient.ToolDef{Name: "nilh"}},
			wantErr: "nil Handler",
		},
		{
			name:    "negative timeout rejected",
			tool:    Tool{Def: llmclient.ToolDef{Name: "neg"}, Handler: nopHandler, Timeout: -time.Second},
			wantErr: "negative Timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := New(Options{})
			err := reg.Register(tt.tool)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Register() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Register() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterDuplicate(t *testing.T) {
	reg := New(Options{})
	if err := reg.Register(echoTool("dup")); err != nil {
		t.Fatalf("first Register() = %v, want nil", err)
	}
	err := reg.Register(echoTool("dup"))
	if err == nil || !strings.Contains(err.Error(), `"dup" already registered`) {
		t.Fatalf("second Register() = %v, want duplicate error", err)
	}
}

func TestMustRegisterPanicsOnError(t *testing.T) {
	reg := New(Options{})
	defer func() {
		if recover() == nil {
			t.Fatal("MustRegister with empty name did not panic")
		}
	}()
	reg.MustRegister(Tool{Def: llmclient.ToolDef{Name: ""}})
}

func TestGet(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(echoTool("present"))

	got, ok := reg.Get("present")
	if !ok || got.Def.Name != "present" {
		t.Fatalf("Get(present) = (%q, %v), want (present, true)", got.Def.Name, ok)
	}
	if _, ok := reg.Get("absent"); ok {
		t.Fatal("Get(absent) = true, want false")
	}
}

func TestNamesAndDefsSorted(t *testing.T) {
	reg := New(Options{})
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		reg.MustRegister(echoTool(name))
	}

	wantNames := []string{"alpha", "bravo", "charlie"}
	gotNames := reg.Names()
	if len(gotNames) != len(wantNames) {
		t.Fatalf("Names() = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("Names() = %v, want %v", gotNames, wantNames)
		}
	}

	defs := reg.Defs()
	if len(defs) != len(wantNames) {
		t.Fatalf("Defs() returned %d defs, want %d", len(defs), len(wantNames))
	}
	for i, def := range defs {
		if def.Name != wantNames[i] {
			t.Fatalf("Defs()[%d].Name = %q, want %q", i, def.Name, wantNames[i])
		}
		if def.Description == "" || def.InputSchema == nil {
			t.Fatalf("Defs()[%d] lost Description or InputSchema: %+v", i, def)
		}
	}
}

func TestDispatchHappyPath(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(echoTool("echo"))

	block := llmclient.ToolUseBlock("tu_1", "echo", json.RawMessage(`{"text":"hi"}`))
	res := reg.Dispatch(context.Background(), block)

	if res.Type != llmclient.BlockToolResult {
		t.Fatalf("res.Type = %q, want %q", res.Type, llmclient.BlockToolResult)
	}
	if res.ToolUseID != "tu_1" {
		t.Fatalf("res.ToolUseID = %q, want tu_1", res.ToolUseID)
	}
	if res.IsError {
		t.Fatalf("res.IsError = true, content %q", res.Content)
	}
	if want := `echo:{"text":"hi"}`; res.Content != want {
		t.Fatalf("res.Content = %q, want %q", res.Content, want)
	}
}

func TestDispatchUnknownTool(t *testing.T) {
	reg := New(Options{})
	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_2", "nope", nil))

	if res.Type != llmclient.BlockToolResult || res.ToolUseID != "tu_2" {
		t.Fatalf("unexpected result block: %+v", res)
	}
	if !res.IsError {
		t.Fatal("unknown tool result not marked IsError")
	}
	if want := `unknown tool "nope"`; !strings.Contains(res.Content, want) {
		t.Fatalf("res.Content = %q, want substring %q", res.Content, want)
	}
}

func TestDispatchNonToolUseBlock(t *testing.T) {
	reg := New(Options{})
	res := reg.Dispatch(context.Background(), llmclient.TextBlock("not a call"))

	if res.Type != llmclient.BlockToolResult || !res.IsError {
		t.Fatalf("non-tool_use dispatch = %+v, want is_error tool_result", res)
	}
	if !strings.Contains(res.Content, "requires a tool_use block") {
		t.Fatalf("res.Content = %q, want misuse message", res.Content)
	}
}

func TestDispatchPolicyDeny(t *testing.T) {
	reg := New(Options{
		Policy: func(name string, _ json.RawMessage) error {
			if name == "forbidden" {
				return errors.New("not in this environment")
			}
			return nil
		},
	})
	reg.MustRegister(echoTool("forbidden"))
	reg.MustRegister(echoTool("allowed"))

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_3", "forbidden", nil))
	if !res.IsError {
		t.Fatal("policy-denied call not marked IsError")
	}
	if want := `tool "forbidden" denied by policy: not in this environment`; res.Content != want {
		t.Fatalf("res.Content = %q, want %q", res.Content, want)
	}

	res = reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_4", "allowed", json.RawMessage(`{}`)))
	if res.IsError {
		t.Fatalf("policy-allowed call marked IsError: %q", res.Content)
	}
}

func TestDispatchHandlerError(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(Tool{
		Def: llmclient.ToolDef{Name: "fails"},
		Handler: func(context.Context, json.RawMessage) (string, error) {
			return "", errors.New("disk on fire")
		},
	})

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_5", "fails", nil))
	if !res.IsError {
		t.Fatal("handler error not marked IsError")
	}
	if want := `tool "fails" failed: disk on fire`; res.Content != want {
		t.Fatalf("res.Content = %q, want %q", res.Content, want)
	}
}

func TestDispatchPanicRecovery(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(Tool{
		Def: llmclient.ToolDef{Name: "boom"},
		Handler: func(context.Context, json.RawMessage) (string, error) {
			panic("kaboom")
		},
	})
	reg.MustRegister(echoTool("echo"))

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_6", "boom", nil))
	if !res.IsError {
		t.Fatal("panicking handler not marked IsError")
	}
	if want := `tool "boom" panicked: kaboom`; res.Content != want {
		t.Fatalf("res.Content = %q, want %q", res.Content, want)
	}

	// The loop survives: a subsequent dispatch on the same registry works.
	res = reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_7", "echo", json.RawMessage(`{}`)))
	if res.IsError {
		t.Fatalf("dispatch after panic marked IsError: %q", res.Content)
	}
}

// TestDispatchTimeout uses a real (short) timeout: the handler cooperatively
// selects on ctx.Done, so the test's duration is bounded by the 20ms deadline
// and does not poll or sleep on its own.
func TestDispatchTimeout(t *testing.T) {
	const timeout = 20 * time.Millisecond
	reg := New(Options{})
	reg.MustRegister(waitTool("slow", timeout))

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_8", "slow", nil))
	if !res.IsError {
		t.Fatal("timed-out call not marked IsError")
	}
	if want := fmt.Sprintf(`tool "slow" timed out after %v`, timeout); res.Content != want {
		t.Fatalf("res.Content = %q, want %q", res.Content, want)
	}
}

// TestDispatchTimeoutPerToolOverridesDefault proves Tool.Timeout wins over
// Options.DefaultTimeout: the named duration in the error is the tool's own.
func TestDispatchTimeoutPerToolOverridesDefault(t *testing.T) {
	const toolTimeout = 10 * time.Millisecond
	reg := New(Options{DefaultTimeout: 10 * time.Second})
	reg.MustRegister(waitTool("slow", toolTimeout))

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_9", "slow", nil))
	if !res.IsError || !strings.Contains(res.Content, fmt.Sprintf("timed out after %v", toolTimeout)) {
		t.Fatalf("res = %+v, want timeout naming %v", res, toolTimeout)
	}
}

// TestDispatchTimeoutNonCooperativeHandler proves a handler that IGNORES its
// context cannot stall the loop: Dispatch returns the timeout result while
// the handler is still blocked. The handler is released via t.Cleanup so its
// goroutine exits before the test binary does.
func TestDispatchTimeoutNonCooperativeHandler(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	const timeout = 10 * time.Millisecond
	reg := New(Options{})
	reg.MustRegister(Tool{
		Def:     llmclient.ToolDef{Name: "stubborn"},
		Timeout: timeout,
		Handler: func(context.Context, json.RawMessage) (string, error) {
			<-release // ignores ctx entirely
			return "too late", nil
		},
	})

	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_10", "stubborn", nil))
	if !res.IsError || !strings.Contains(res.Content, "timed out after") {
		t.Fatalf("res = %+v, want timeout result despite non-cooperative handler", res)
	}
}

func TestDispatchParentCancellation(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(waitTool("waits", 0)) // no timeout: only the parent ctx applies

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before dispatch

	res := reg.Dispatch(ctx, llmclient.ToolUseBlock("tu_11", "waits", nil))
	if !res.IsError {
		t.Fatal("cancelled call not marked IsError")
	}
	if !strings.Contains(res.Content, "canceled") && !strings.Contains(res.Content, "context canceled") {
		t.Fatalf("res.Content = %q, want cancellation message", res.Content)
	}
}

func TestDispatchAllOrderingAndMixedBlocks(t *testing.T) {
	var order []string // safe: DispatchAll is sequential
	mkTool := func(name string) Tool {
		return Tool{
			Def: llmclient.ToolDef{Name: name},
			Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
				order = append(order, name)
				return "ran " + name, nil
			},
		}
	}

	reg := New(Options{})
	reg.MustRegister(mkTool("first"))
	reg.MustRegister(mkTool("second"))

	blocks := []llmclient.ContentBlock{
		llmclient.TextBlock("thinking out loud"),               // skipped
		llmclient.ToolUseBlock("tu_a", "first", nil),           // dispatched
		{Type: llmclient.BlockUnknown, Raw: []byte(`{"t":1}`)}, // skipped
		llmclient.ToolUseBlock("tu_b", "second", nil),          // dispatched
		llmclient.ToolResultBlock("tu_z", "old result", false), // skipped
		llmclient.ToolUseBlock("tu_c", "no_such_tool", nil),    // dispatched -> is_error
	}

	results := reg.DispatchAll(context.Background(), blocks)
	if len(results) != 3 {
		t.Fatalf("DispatchAll returned %d results, want 3: %+v", len(results), results)
	}

	wantIDs := []string{"tu_a", "tu_b", "tu_c"}
	for i, res := range results {
		if res.Type != llmclient.BlockToolResult {
			t.Fatalf("results[%d].Type = %q, want tool_result", i, res.Type)
		}
		if res.ToolUseID != wantIDs[i] {
			t.Fatalf("results[%d].ToolUseID = %q, want %q", i, res.ToolUseID, wantIDs[i])
		}
	}
	if results[0].IsError || results[1].IsError {
		t.Fatalf("known tools errored: %+v", results[:2])
	}
	if !results[2].IsError {
		t.Fatal("unknown tool result not marked IsError")
	}

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("handler execution order = %v, want [first second]", order)
	}
}

func TestDispatchAllNoToolUseBlocks(t *testing.T) {
	reg := New(Options{})
	results := reg.DispatchAll(context.Background(), []llmclient.ContentBlock{
		llmclient.TextBlock("just text"),
	})
	if results != nil {
		t.Fatalf("DispatchAll = %+v, want nil", results)
	}
}

func TestDispatchConcurrent(t *testing.T) {
	var calls int64
	var mu sync.Mutex

	reg := New(Options{})
	reg.MustRegister(Tool{
		Def: llmclient.ToolDef{Name: "count"},
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return "ok", nil
		},
	})

	const goroutines = 32
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("tu_%d", i)
			var res llmclient.ContentBlock
			if i%4 == 0 {
				res = reg.Dispatch(context.Background(), llmclient.ToolUseBlock(id, "missing", nil))
				if !res.IsError {
					t.Errorf("goroutine %d: unknown tool not IsError", i)
				}
			} else {
				res = reg.Dispatch(context.Background(), llmclient.ToolUseBlock(id, "count", nil))
				if res.IsError {
					t.Errorf("goroutine %d: dispatch errored: %q", i, res.Content)
				}
			}
			if res.ToolUseID != id {
				t.Errorf("goroutine %d: ToolUseID = %q, want %q", i, res.ToolUseID, id)
			}
			// Exercise the read paths concurrently too.
			reg.Names()
			reg.Defs()
			reg.Get("count")
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if want := int64(goroutines - goroutines/4); calls != want {
		t.Fatalf("handler ran %d times, want %d", calls, want)
	}
}

func TestObjectSchemaGolden(t *testing.T) {
	got := ObjectSchema(
		Prop{Name: "query", Type: "string", Description: "search terms", Required: true},
		Prop{Name: "limit", Type: "integer", Description: "max results"},
		Prop{Name: "strict", Type: "boolean"},
		Prop{Name: "scale", Type: "number", Required: true},
		Prop{Name: "mode", Type: "string", Enum: []string{"fast", "thorough"}},
		Prop{Name: "tags", Type: "array", Items: "string", Description: "filter tags"},
	)

	const golden = `{"type":"object","properties":{` +
		`"query":{"type":"string","description":"search terms"},` +
		`"limit":{"type":"integer","description":"max results"},` +
		`"strict":{"type":"boolean"},` +
		`"scale":{"type":"number"},` +
		`"mode":{"type":"string","enum":["fast","thorough"]},` +
		`"tags":{"type":"array","items":{"type":"string"},"description":"filter tags"}` +
		`},"required":["query","scale"]}`

	if string(got) != golden {
		t.Fatalf("ObjectSchema mismatch:\n got: %s\nwant: %s", got, golden)
	}
	if !json.Valid(got) {
		t.Fatalf("ObjectSchema emitted invalid JSON: %s", got)
	}
}

func TestObjectSchemaEmpty(t *testing.T) {
	got := ObjectSchema()
	const golden = `{"type":"object","properties":{}}`
	if string(got) != golden {
		t.Fatalf("ObjectSchema() = %s, want %s", got, golden)
	}
}

func TestObjectSchemaEscaping(t *testing.T) {
	got := ObjectSchema(Prop{Name: `na"me`, Type: "string", Description: "line1\nline2"})
	if !json.Valid(got) {
		t.Fatalf("ObjectSchema with quotes/newlines emitted invalid JSON: %s", got)
	}
	var parsed struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Properties[`na"me`].Description != "line1\nline2" {
		t.Fatalf("round-trip lost escaping: %s", got)
	}
}

func TestObjectSchemaPanics(t *testing.T) {
	tests := []struct {
		name  string
		props []Prop
		want  string // substring of the panic message
	}{
		{
			name:  "empty name",
			props: []Prop{{Type: "string"}},
			want:  "empty Name",
		},
		{
			name:  "duplicate name",
			props: []Prop{{Name: "x", Type: "string"}, {Name: "x", Type: "integer"}},
			want:  `duplicate prop "x"`,
		},
		{
			name:  "unsupported type",
			props: []Prop{{Name: "x", Type: "object"}},
			want:  `unsupported Type "object"`,
		},
		{
			name:  "array without items",
			props: []Prop{{Name: "x", Type: "array"}},
			want:  "needs a scalar Items type",
		},
		{
			name:  "array with non-scalar items",
			props: []Prop{{Name: "x", Type: "array", Items: "object"}},
			want:  "needs a scalar Items type",
		},
		{
			name:  "items on non-array",
			props: []Prop{{Name: "x", Type: "string", Items: "string"}},
			want:  "has Items but Type",
		},
		{
			name:  "enum on non-string",
			props: []Prop{{Name: "x", Type: "integer", Enum: []string{"1"}}},
			want:  "has Enum but Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				v := recover()
				if v == nil {
					t.Fatal("ObjectSchema did not panic")
				}
				if msg := fmt.Sprint(v); !strings.Contains(msg, tt.want) {
					t.Fatalf("panic = %q, want substring %q", msg, tt.want)
				}
			}()
			ObjectSchema(tt.props...)
		})
	}
}

func TestPolicyFromExecutionPolicy(t *testing.T) {
	ps, err := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "search", Allow: true, Environments: []string{"prod"}},
		{Tool: "delete", Allow: false, Environments: []string{"prod"}},
	})
	if err != nil {
		t.Fatalf("NewPolicySet: %v", err)
	}

	policy := PolicyFromExecutionPolicy(ps, "prod")

	if err := policy("search", nil); err != nil {
		t.Fatalf("allowed tool denied: %v", err)
	}
	if err := policy("delete", nil); err == nil || !strings.Contains(err.Error(), "explicit deny") {
		t.Fatalf("explicitly denied tool: err = %v, want explicit-deny reason", err)
	}
	if err := policy("unlisted", nil); err == nil || !strings.Contains(err.Error(), "deny-by-default") {
		t.Fatalf("unlisted tool: err = %v, want deny-by-default reason", err)
	}
	// Decide errors (empty tool name) also deny — safe by default.
	if err := policy("", nil); err == nil {
		t.Fatal("empty tool name: err = nil, want denial")
	}

	// End to end: the denial reason reaches the model as an is_error result.
	reg := New(Options{Policy: policy})
	reg.MustRegister(echoTool("delete"))
	res := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("tu_p", "delete", nil))
	if !res.IsError || !strings.Contains(res.Content, "denied by policy") {
		t.Fatalf("res = %+v, want policy denial surfaced to the model", res)
	}
}

// ── Capability taxonomy ───────────────────────────────────────────────────────

func TestCapabilitySetDeclaredVsUnknown(t *testing.T) {
	// nil = unknown, distinct from explicitly empty.
	var unknown CapabilitySet
	if unknown != nil {
		t.Fatal("zero-value CapabilitySet should be nil (unknown)")
	}

	empty := DeclareNone()
	if empty == nil {
		t.Fatal("DeclareNone() should return non-nil (declared-empty)")
	}
	if empty.Has(CapReadsUntrusted) {
		t.Fatal("declared-empty set should not have CapReadsUntrusted")
	}

	full := NewCapabilitySet(CapReadsUntrusted, CapReadsPrivate, CapWritesExternal)
	for _, c := range []Capability{CapReadsUntrusted, CapReadsPrivate, CapWritesExternal} {
		if !full.Has(c) {
			t.Errorf("full set missing %q", c)
		}
	}
}

func TestCapabilitySetUnion(t *testing.T) {
	a := NewCapabilitySet(CapReadsUntrusted)
	b := NewCapabilitySet(CapReadsPrivate, CapWritesExternal)
	u := a.union(b)
	for _, c := range []Capability{CapReadsUntrusted, CapReadsPrivate, CapWritesExternal} {
		if !u.Has(c) {
			t.Errorf("union missing %q", c)
		}
	}
}

func TestCapabilitySetCompletesTrifecta(t *testing.T) {
	cases := []struct {
		caps []Capability
		want bool
	}{
		{[]Capability{CapReadsUntrusted, CapReadsPrivate, CapWritesExternal}, true},
		{[]Capability{CapReadsUntrusted, CapReadsPrivate}, false},
		{[]Capability{CapReadsUntrusted, CapWritesExternal}, false},
		{[]Capability{CapReadsPrivate, CapWritesExternal}, false},
		{[]Capability{CapReadsUntrusted}, false},
		{[]Capability{}, false},
	}
	for _, tc := range cases {
		s := NewCapabilitySet(tc.caps...)
		if got := s.completesTrifecta(); got != tc.want {
			t.Errorf("caps=%v completesTrifecta()=%v, want %v", tc.caps, got, tc.want)
		}
	}
}

// ── Trifecta detection — permissive mode ─────────────────────────────────────

func TestTrifectaPermissive_AllowsCompletion(t *testing.T) {
	var completed string
	reg := New(Options{
		Trifecta: TrifectaOptions{
			Mode: TrifectaPermissive,
			OnComplete: func(name string) {
				completed = name
			},
		},
	})

	// Register three tools across three registration calls, each with one leg.
	if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted)); err != nil {
		t.Fatalf("Register reader: %v", err)
	}
	if err := reg.Register(toolWithCaps("vault", CapReadsPrivate)); err != nil {
		t.Fatalf("Register vault: %v", err)
	}
	// "sender" completes the trifecta — should succeed in permissive mode.
	if err := reg.Register(toolWithCaps("sender", CapWritesExternal)); err != nil {
		t.Fatalf("Register sender: %v (permissive should allow): %v", "sender", err)
	}

	if completed != "sender" {
		t.Errorf("OnComplete called with %q, want \"sender\"", completed)
	}
	if !reg.TrifectaComplete() {
		t.Error("TrifectaComplete() = false, want true after all three legs registered")
	}
}

// TestTrifectaStrict_BlocksCompletion verifies that TrifectaStrict refuses the
// tool that completes the trifecta, and that registration order doesn't matter.
func TestTrifectaStrict_BlocksCompletion(t *testing.T) {
	// Table: each permutation of which tool is registered last (completes).
	type leg struct {
		name string
		cap  Capability
	}
	legs := []leg{
		{"reader", CapReadsUntrusted},
		{"vault", CapReadsPrivate},
		{"sender", CapWritesExternal},
	}
	permutations := [][3]int{
		{0, 1, 2},
		{0, 2, 1},
		{1, 0, 2},
		{1, 2, 0},
		{2, 0, 1},
		{2, 1, 0},
	}

	for _, perm := range permutations {
		var completedName string
		reg := New(Options{
			Trifecta: TrifectaOptions{
				Mode: TrifectaStrict,
				OnComplete: func(name string) {
					completedName = name
				},
			},
		})

		// First two succeed.
		for _, idx := range perm[:2] {
			if err := reg.Register(toolWithCaps(legs[idx].name, legs[idx].cap)); err != nil {
				t.Fatalf("perm %v: Register[%d] %q: unexpected error: %v", perm, idx, legs[idx].name, err)
			}
		}

		// Third completes the trifecta and must be blocked.
		lastIdx := perm[2]
		err := reg.Register(toolWithCaps(legs[lastIdx].name, legs[lastIdx].cap))
		if err == nil {
			t.Errorf("perm %v: third Register should have been blocked, got nil", perm)
			continue
		}
		var blocked *ErrTrifectaBlocked
		if !errors.As(err, &blocked) {
			t.Errorf("perm %v: error type = %T, want *ErrTrifectaBlocked; err: %v", perm, err, err)
			continue
		}
		if blocked.ToolName != legs[lastIdx].name {
			t.Errorf("perm %v: ErrTrifectaBlocked.ToolName = %q, want %q", perm, blocked.ToolName, legs[lastIdx].name)
		}
		if completedName != legs[lastIdx].name {
			t.Errorf("perm %v: OnComplete called with %q, want %q", perm, completedName, legs[lastIdx].name)
		}
		// After a blocked registration, the trifecta must NOT be marked complete.
		if reg.TrifectaComplete() {
			t.Errorf("perm %v: TrifectaComplete() = true after blocked registration", perm)
		}
		// The two successfully registered tools should still be present.
		if len(reg.Names()) != 2 {
			t.Errorf("perm %v: Names() = %v, want 2 tools after blocked registration", perm, reg.Names())
		}
	}
}

// TestTrifectaStrict_OverrideAllows confirms that a valid TrifectaOverride
// permits completion under strict mode and that the override record is retrievable.
func TestTrifectaStrict_OverrideAllows(t *testing.T) {
	override := &TrifectaOverride{
		AuthorizedBy: "security-team@example.com",
		Reason:       "read-web tool is sandboxed; private data is pre-filtered before the agent sees it",
	}
	reg := New(Options{
		Trifecta: TrifectaOptions{
			Mode:     TrifectaStrict,
			Override: override,
		},
	})

	if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted)); err != nil {
		t.Fatalf("Register reader: %v", err)
	}
	if err := reg.Register(toolWithCaps("vault", CapReadsPrivate)); err != nil {
		t.Fatalf("Register vault: %v", err)
	}
	// With override, completing tool should be accepted.
	if err := reg.Register(toolWithCaps("sender", CapWritesExternal)); err != nil {
		t.Fatalf("Register sender with override: %v", err)
	}

	if !reg.TrifectaComplete() {
		t.Error("TrifectaComplete() = false, want true")
	}
	got := reg.TrifectaOverrideRecord()
	if got != override {
		t.Errorf("TrifectaOverrideRecord() = %v, want the same pointer", got)
	}
	if got.AuthorizedBy != override.AuthorizedBy || got.Reason != override.Reason {
		t.Errorf("override record mismatch: %+v", got)
	}
}

// TestTrifectaStrict_InvalidOverrideBlocks confirms that an override with
// missing fields is treated as absent (still blocks).
func TestTrifectaStrict_InvalidOverrideBlocks(t *testing.T) {
	cases := []struct {
		name     string
		override *TrifectaOverride
	}{
		{"nil override", nil},
		{"empty AuthorizedBy", &TrifectaOverride{AuthorizedBy: "", Reason: "valid reason"}},
		{"empty Reason", &TrifectaOverride{AuthorizedBy: "alice", Reason: ""}},
		{"whitespace AuthorizedBy", &TrifectaOverride{AuthorizedBy: "   ", Reason: "valid reason"}},
		{"whitespace Reason", &TrifectaOverride{AuthorizedBy: "alice", Reason: "  "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := New(Options{
				Trifecta: TrifectaOptions{
					Mode:     TrifectaStrict,
					Override: tc.override,
				},
			})
			if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted)); err != nil {
				t.Fatalf("Register reader: %v", err)
			}
			if err := reg.Register(toolWithCaps("vault", CapReadsPrivate)); err != nil {
				t.Fatalf("Register vault: %v", err)
			}
			err := reg.Register(toolWithCaps("sender", CapWritesExternal))
			if err == nil {
				t.Fatal("expected ErrTrifectaBlocked, got nil")
			}
			var blocked *ErrTrifectaBlocked
			if !errors.As(err, &blocked) {
				t.Errorf("error type = %T, want *ErrTrifectaBlocked", err)
			}
		})
	}
}

// TestTrifectaStrict_UnknownCapsAsWorstCase confirms that a tool with nil
// Capabilities is treated as all-three under strict mode.
func TestTrifectaStrict_UnknownCapsAsWorstCase(t *testing.T) {
	reg := New(Options{
		Trifecta: TrifectaOptions{Mode: TrifectaStrict},
	})

	// Register one tool that covers two legs, then an unknown-caps tool should
	// complete the trifecta (unknown = all three = the remaining leg).
	if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted, CapReadsPrivate)); err != nil {
		t.Fatalf("Register reader: %v", err)
	}

	// Unknown tool: nil Capabilities.
	unknown := echoTool("legacy") // echoTool has nil Capabilities
	err := reg.Register(unknown)
	if err == nil {
		t.Fatal("unknown-caps tool should have been blocked in strict mode (treated as all-three)")
	}
	var blocked *ErrTrifectaBlocked
	if !errors.As(err, &blocked) {
		t.Errorf("error type = %T, want *ErrTrifectaBlocked", err)
	}
}

// TestTrifectaPermissive_UnknownCapsNotBlocked confirms that unknown-caps tools
// are NOT blocked under permissive mode even if they could complete the trifecta.
func TestTrifectaPermissive_UnknownCapsNotBlocked(t *testing.T) {
	reg := New(Options{
		Trifecta: TrifectaOptions{Mode: TrifectaPermissive},
	})
	if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted, CapReadsPrivate)); err != nil {
		t.Fatalf("Register reader: %v", err)
	}
	// Unknown tool should be accepted without error in permissive mode.
	if err := reg.Register(echoTool("legacy")); err != nil {
		t.Fatalf("Register legacy (permissive): %v", err)
	}
}

// TestTrifectaStrict_DeclaredNoneNotBlocked confirms that a tool explicitly
// declared with DeclareNone() does not contribute to trifecta completion.
func TestTrifectaStrict_DeclaredNoneNotBlocked(t *testing.T) {
	reg := New(Options{
		Trifecta: TrifectaOptions{Mode: TrifectaStrict},
	})
	if err := reg.Register(toolWithCaps("reader", CapReadsUntrusted, CapReadsPrivate)); err != nil {
		t.Fatalf("Register reader: %v", err)
	}
	// Explicitly declared as having no relevant capabilities.
	clean := echoTool("clean-legacy")
	clean.Capabilities = DeclareNone()
	if err := reg.Register(clean); err != nil {
		t.Fatalf("Register clean-legacy (declared none) should not be blocked: %v", err)
	}
}

// TestTrifectaStrict_OverrideProvenanceRecorded verifies that the override
// fields are preserved verbatim for audit retrieval.
func TestTrifectaStrict_OverrideProvenanceRecorded(t *testing.T) {
	const who = "ciso@example.com"
	const why = "isolated read environment; external writes are append-only audit log"
	reg := New(Options{
		Trifecta: TrifectaOptions{
			Mode:     TrifectaStrict,
			Override: &TrifectaOverride{AuthorizedBy: who, Reason: why},
		},
	})
	for _, tc := range []struct {
		name string
		cap  Capability
	}{
		{"reader", CapReadsUntrusted},
		{"vault", CapReadsPrivate},
		{"sender", CapWritesExternal},
	} {
		if err := reg.Register(toolWithCaps(tc.name, tc.cap)); err != nil {
			t.Fatalf("Register %q: %v", tc.name, err)
		}
	}

	rec := reg.TrifectaOverrideRecord()
	if rec == nil {
		t.Fatal("TrifectaOverrideRecord() = nil, want override")
	}
	if rec.AuthorizedBy != who {
		t.Errorf("AuthorizedBy = %q, want %q", rec.AuthorizedBy, who)
	}
	if rec.Reason != why {
		t.Errorf("Reason = %q, want %q", rec.Reason, why)
	}
}

// TestTrifectaNoOverrideRecord_NilWhenNotSet confirms nil is returned when no
// override was supplied.
func TestTrifectaNoOverrideRecord_NilWhenNotSet(t *testing.T) {
	reg := New(Options{})
	if got := reg.TrifectaOverrideRecord(); got != nil {
		t.Errorf("TrifectaOverrideRecord() = %+v, want nil", got)
	}
}

// TestErrTrifectaBlocked_ErrorMessage confirms the error message is
// human-readable and contains key fields.
func TestErrTrifectaBlocked_ErrorMessage(t *testing.T) {
	err := &ErrTrifectaBlocked{
		ToolName:          "exfil-tool",
		CapabilitiesUnion: "reads_untrusted + reads_private + writes_external",
	}
	msg := err.Error()
	if !strings.Contains(msg, "exfil-tool") {
		t.Errorf("error message missing tool name: %q", msg)
	}
	if !strings.Contains(msg, "lethal trifecta") {
		t.Errorf("error message missing 'lethal trifecta': %q", msg)
	}
	if !strings.Contains(msg, "TrifectaOverride") {
		t.Errorf("error message missing override hint: %q", msg)
	}
}

// TestTrifectaComplete_FalseBeforeThirdLeg confirms TrifectaComplete is false
// until the third leg is added.
func TestTrifectaComplete_FalseBeforeThirdLeg(t *testing.T) {
	reg := New(Options{})
	if reg.TrifectaComplete() {
		t.Fatal("TrifectaComplete() = true on empty registry")
	}
	reg.MustRegister(toolWithCaps("a", CapReadsUntrusted))
	if reg.TrifectaComplete() {
		t.Fatal("TrifectaComplete() = true after one leg")
	}
	reg.MustRegister(toolWithCaps("b", CapReadsPrivate))
	if reg.TrifectaComplete() {
		t.Fatal("TrifectaComplete() = true after two legs")
	}
	reg.MustRegister(toolWithCaps("c", CapWritesExternal))
	if !reg.TrifectaComplete() {
		t.Fatal("TrifectaComplete() = false after all three legs")
	}
}

// toolWithCaps returns a minimal echo Tool with the given capabilities.
func toolWithCaps(name string, caps ...Capability) Tool {
	t := echoTool(name)
	t.Capabilities = NewCapabilitySet(caps...)
	return t
}
