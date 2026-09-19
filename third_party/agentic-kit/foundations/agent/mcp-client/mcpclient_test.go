package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// ─── scripted fake server ────────────────────────────────────────────────────

// pipeServer drives a scripted MCP server over an in-process io.Pipe.
// Each round-trip: read one line from the client, write one pre-loaded response.
// Notifications (no response expected) are handled by skipOnce.
type pipeServer struct {
	cPR *io.PipeReader // client → server (server reads from here)
	sPW *io.PipeWriter // server → client (server writes here)
}

// newPipeServer returns (server, clientReadFrom, clientWriteTo) so the test
// can construct an IOTransport over the client side.
func newPipeServer() (*pipeServer, io.Reader, io.Writer) {
	sPR, sPW := io.Pipe() // server writes → client reads
	cPR, cPW := io.Pipe() // client writes → server reads
	return &pipeServer{cPR: cPR, sPW: sPW}, sPR, cPW
}

// readLine reads one '\n'-terminated line from the client (discards content).
func (ps *pipeServer) readLine() error {
	buf := make([]byte, 1)
	for {
		_, err := ps.cPR.Read(buf)
		if err != nil {
			return err
		}
		if buf[0] == '\n' {
			return nil
		}
	}
}

// respond reads one client line then sends resp+"\n".
func (ps *pipeServer) respond(resp string) {
	if err := ps.readLine(); err != nil {
		return
	}
	ps.sPW.Write([]byte(resp + "\n"))
}

// skip reads one client line and sends nothing (for notifications).
func (ps *pipeServer) skip() {
	ps.readLine() //nolint:errcheck
}

// close closes server-side pipes; client reads will get EOF.
func (ps *pipeServer) close() {
	ps.sPW.Close()
	ps.cPR.Close()
}

// ─── response builders ───────────────────────────────────────────────────────

func initResp(id int, version string) string {
	return fmt.Sprintf(
		`{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":%q,"capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"1"}}}`,
		id, version,
	)
}

type toolInfoJSON struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
}

func toolsListResp(id int, tools []toolInfoJSON) string {
	b, _ := json.Marshal(struct {
		Tools []toolInfoJSON `json:"tools"`
	}{Tools: tools})
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, id, b)
}

func toolsCallResp(id int, text string, isError bool) string {
	type item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	b, _ := json.Marshal(struct {
		Content []item `json:"content"`
		IsError bool   `json:"isError"`
	}{
		Content: []item{{Type: "text", Text: text}},
		IsError: isError,
	})
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, id, b)
}

func rpcErrResp(id, code int, msg string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"error":{"code":%d,"message":%q}}`, id, code, msg)
}

// recordingTransport is a deterministic fake Transport that records JSON-RPC
// methods, allowing tests to prove a rejected verification never dispatches
// tools/call.
type recordingTransport struct {
	mu        sync.Mutex
	responses []string
	methods   []string
}

func (t *recordingTransport) Send(_ context.Context, msg []byte) error {
	var req struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(msg, &req); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.methods = append(t.methods, req.Method)
	return nil
}

func (t *recordingTransport) Receive(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.responses) == 0 {
		return nil, errors.New("unexpected receive")
	}
	response := t.responses[0]
	t.responses = t.responses[1:]
	return []byte(response), nil
}

func (t *recordingTransport) Close() error { return nil }

func (t *recordingTransport) methodCount(method string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, got := range t.methods {
		if got == method {
			count++
		}
	}
	return count
}

func newRecordingClient(t *testing.T, responses ...string) (*Client, *recordingTransport) {
	t.Helper()
	tr := &recordingTransport{responses: responses}
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, tr
}

// ─── test client constructor ─────────────────────────────────────────────────

// defaultTools is the base tool set used by most tests.
var defaultTools = []toolInfoJSON{
	{
		Name:        "echo",
		Description: "echo text",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
	},
	{
		Name:        "add",
		Description: "add integers",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"]}`),
	},
}

// newTestClient creates a Client over a scripted pipeServer loaded with:
//   - initResp for initialize
//   - skip for notifications/initialized
//   - toolsListResp for the initial tools/list (called by New)
//
// It returns the client and the server (for subsequent interactions).
func newTestClient(t *testing.T, version string, tools []toolInfoJSON) (*Client, *pipeServer) {
	t.Helper()
	ps, r, w := newPipeServer()
	// id sequence: 1=initialize, 2=tools/list (notifications/initialized has no id)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ps.respond(initResp(1, version))
		ps.skip() // notifications/initialized
		ps.respond(toolsListResp(2, tools))
	}()

	tr := NewIOTransport(r, w, 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	<-done
	return c, ps
}

// ─── handshake / version negotiation ─────────────────────────────────────────

func TestHandshakeVersionNegotiation(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"2025-11-25", "2025-11-25"},
		{"2025-06-18", "2025-06-18"},
		{"older server", "2024-11-05"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestClient(t, tt.version, defaultTools)
			if c.ProtocolVersion != tt.version {
				t.Errorf("ProtocolVersion = %q, want %q", c.ProtocolVersion, tt.version)
			}
		})
	}
}

func TestHandshakeOffersLatestVersion(t *testing.T) {
	// Verify the client sends 2025-11-25 as its first offered version by
	// observing what lands in the transport (the fake server accepts anything).
	// We do this by inspecting offeredVersions directly — the protocol spec
	// says the client sends its preferred version in the initialize request.
	if len(offeredVersions) == 0 {
		t.Fatal("offeredVersions is empty")
	}
	if offeredVersions[0] != "2025-11-25" {
		t.Errorf("highest offered version = %q, want 2025-11-25", offeredVersions[0])
	}
}

func TestHandshakeRPCError(t *testing.T) {
	ps, r, w := newPipeServer()
	go ps.respond(rpcErrResp(1, -32600, "bad request"))

	tr := NewIOTransport(r, w, 0)
	_, err := New(context.Background(), tr, Options{})
	if err == nil {
		t.Fatal("expected error from RPC error response, got nil")
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Errorf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32600 {
		t.Errorf("RPCError.Code = %d, want -32600", rpcErr.Code)
	}
}

// ─── pin / drift / repin ─────────────────────────────────────────────────────

func TestPinRecordedAtFirstList(t *testing.T) {
	c, _ := newTestClient(t, "2025-11-25", defaultTools)
	for _, tool := range defaultTools {
		h, pinned := c.VerifyPin(tool.Name)
		if !pinned {
			t.Errorf("tool %q not pinned after New", tool.Name)
		}
		want := toolHash(tool.Name, tool.Description, tool.InputSchema, nil)
		if h != want {
			t.Errorf("tool %q: pin = %s, want %s", tool.Name, h, want)
		}
	}
}

func TestPinDriftDetected(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	changedTools := []toolInfoJSON{
		{Name: "echo", Description: "MODIFIED DESCRIPTION", InputSchema: defaultTools[0].InputSchema},
		defaultTools[1],
	}
	// id=3: the next call after handshake (1) and initial list (2)
	go ps.respond(toolsListResp(3, changedTools))

	_, err := c.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected ToolChangedError, got nil")
	}
	var changed *ToolChangedError
	if !errors.As(err, &changed) {
		t.Errorf("expected *ToolChangedError, got %T: %v", err, err)
	}
	if changed.Name != "echo" {
		t.Errorf("ToolChangedError.Name = %q, want \"echo\"", changed.Name)
	}
}

func TestRepinSilencesDrift(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	changedTools := []toolInfoJSON{
		{Name: "echo", Description: "MODIFIED", InputSchema: defaultTools[0].InputSchema},
		defaultTools[1],
	}
	newHash := toolHash("echo", "MODIFIED", defaultTools[0].InputSchema, nil)

	// First list: drift detected.
	go ps.respond(toolsListResp(3, changedTools))
	if _, err := c.ListTools(context.Background()); err == nil {
		t.Fatal("expected ToolChangedError on first drift")
	}

	// Repin to new hash.
	c.Repin("echo", newHash)

	// Second list: same modified tool — no error now.
	go ps.respond(toolsListResp(4, changedTools))
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Errorf("expected no error after Repin, got: %v", err)
	}
}

func TestUnchangedToolsNoError(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	go ps.respond(toolsListResp(3, defaultTools))
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Errorf("expected no error for unchanged tools, got: %v", err)
	}
}

// ─── tools/call ──────────────────────────────────────────────────────────────

func TestCallToolSuccess(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	go ps.respond(toolsCallResp(3, "hello", false))
	result, err := c.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestCallToolInBandError(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	go ps.respond(toolsCallResp(3, "boom", true))
	_, err := c.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"x"}`))
	if err == nil {
		t.Fatal("expected ToolCallError, got nil")
	}
	var tce *ToolCallError
	if !errors.As(err, &tce) {
		t.Errorf("expected *ToolCallError, got %T: %v", err, err)
	}
	if tce.Content != "boom" {
		t.Errorf("ToolCallError.Content = %q, want %q", tce.Content, "boom")
	}
}

func TestCallToolRPCError(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)

	go ps.respond(rpcErrResp(3, -32602, "invalid params"))
	_, err := c.CallTool(context.Background(), "echo", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsRPCError(err, -32602) {
		t.Errorf("expected RPCError -32602, got %v", err)
	}
}

func TestCallToolVerified(t *testing.T) {
	changedTools := []toolInfoJSON{
		{Name: "echo", Description: "changed", InputSchema: defaultTools[0].InputSchema},
		defaultTools[1],
	}
	tests := []struct {
		name      string
		tool      string
		responses []string
		cancel    bool
		want      string
		wantErr   any
		wantCalls int
	}{
		{
			name:      "valid metadata dispatches",
			tool:      "echo",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools), toolsListResp(3, defaultTools), toolsCallResp(4, "verified", false)},
			want:      "verified",
			wantCalls: 1,
		},
		{
			name:      "drift refuses before dispatch",
			tool:      "echo",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools), toolsListResp(3, changedTools)},
			wantErr:   &ToolChangedError{},
		},
		{
			name:      "missing refuses before dispatch",
			tool:      "echo",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools), toolsListResp(3, []toolInfoJSON{defaultTools[1]})},
			wantErr:   &ToolNotFoundError{},
		},
		{
			name:      "unpinned refuses without self-authorizing",
			tool:      "new-tool",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools)},
			wantErr:   &ToolNotPinnedError{},
		},
		{
			name:      "malformed list refuses before dispatch",
			tool:      "echo",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools), `{"jsonrpc":"2.0","id":3,"result":{"tools":`},
			wantErr:   &json.SyntaxError{},
		},
		{
			name:      "canceled list refuses before dispatch",
			tool:      "echo",
			responses: []string{initResp(1, "2025-11-25"), toolsListResp(2, defaultTools)},
			cancel:    true,
			wantErr:   context.Canceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, tr := newRecordingClient(t, tt.responses...)
			ctx := context.Background()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := c.CallToolVerified(ctx, tt.tool, json.RawMessage(`{}`))
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("CallToolVerified error = nil")
				}
				switch want := tt.wantErr.(type) {
				case error:
					switch tt.wantErr.(type) {
					case *ToolChangedError:
						var target *ToolChangedError
						if !errors.As(err, &target) {
							t.Errorf("error = %T (%v), want *ToolChangedError", err, err)
						}
					case *ToolNotFoundError:
						var target *ToolNotFoundError
						if !errors.As(err, &target) {
							t.Errorf("error = %T (%v), want *ToolNotFoundError", err, err)
						}
					case *ToolNotPinnedError:
						var target *ToolNotPinnedError
						if !errors.As(err, &target) {
							t.Errorf("error = %T (%v), want *ToolNotPinnedError", err, err)
						}
					case *json.SyntaxError:
						var target *json.SyntaxError
						if !errors.As(err, &target) {
							t.Errorf("error = %T (%v), want *json.SyntaxError", err, err)
						}
					default:
						if !errors.Is(err, want) {
							t.Errorf("error = %v, want %v", err, want)
						}
					}
				}
			} else if err != nil {
				t.Fatalf("CallToolVerified: %v", err)
			}
			if got != tt.want {
				t.Errorf("result = %q, want %q", got, tt.want)
			}
			if gotCalls := tr.methodCount("tools/call"); gotCalls != tt.wantCalls {
				t.Errorf("tools/call sends = %d, want %d", gotCalls, tt.wantCalls)
			}
		})
	}
}

func TestCallToolVerifiedHonorsConcurrentRepin(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", defaultTools)
	changedTools := []toolInfoJSON{
		{Name: "echo", Description: "changed", InputSchema: defaultTools[0].InputSchema},
		defaultTools[1],
	}
	newHash := toolHash("echo", "changed", defaultTools[0].InputSchema, nil)
	listed := make(chan struct{})
	continueList := make(chan struct{})
	go func() {
		if err := ps.readLine(); err != nil {
			return
		}
		close(listed)
		<-continueList
		ps.sPW.Write([]byte(toolsListResp(3, changedTools) + "\n"))
		ps.respond(toolsCallResp(4, "repinned", false))
	}()

	result := make(chan struct {
		text string
		err  error
	}, 1)
	go func() {
		text, err := c.CallToolVerified(context.Background(), "echo", json.RawMessage(`{}`))
		result <- struct {
			text string
			err  error
		}{text, err}
	}()
	<-listed
	c.Repin("echo", newHash)
	close(continueList)
	got := <-result
	if got.err != nil {
		t.Fatalf("CallToolVerified: %v", got.err)
	}
	if got.text != "repinned" {
		t.Errorf("result = %q, want repinned", got.text)
	}
}

func TestToolChangedErrorShortHashesAreSafe(t *testing.T) {
	err := (&ToolChangedError{Name: "echo", OldHash: "x", NewHash: ""}).Error()
	if !strings.Contains(err, "pin x → ") {
		t.Errorf("Error() = %q, want safe short-hash rendering", err)
	}
}

func TestToolHashCanonicalizesJSONMetadata(t *testing.T) {
	if got, want := toolHash("echo", "description", json.RawMessage(`{"b":2,"a":1}`), json.RawMessage(`{"z":0,"a":1}`)), toolHash("echo", "description", json.RawMessage(` { "a" : 1, "b" : 2 } `), json.RawMessage(` { "a": 1, "z": 0 } `)); got != want {
		t.Errorf("canonical metadata hashes differ: %s != %s", got, want)
	}
}

// ─── context cancellation ────────────────────────────────────────────────────

func TestContextCancellation(t *testing.T) {
	// Build a server that reads the client's request but never responds, so
	// Receive blocks. The client's context times out and the call returns.
	ps, r, w := newPipeServer()
	go func() {
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()                                  // notifications/initialized
		ps.respond(toolsListResp(2, defaultTools)) // initial list in New
		// Read the tools/call request but send no response.
		ps.readLine() //nolint:errcheck
		// Drain the pipe so the Send in call() completes, then block on Receive.
	}()

	tr := NewIOTransport(r, w, 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, callErr := c.CallTool(ctx, "echo", json.RawMessage(`{"text":"x"}`))
	if callErr == nil {
		t.Fatal("expected context error, got nil")
	}
	// Receive checks ctx before blocking, so we get DeadlineExceeded or Canceled.
	if !errors.Is(callErr, context.DeadlineExceeded) && !errors.Is(callErr, context.Canceled) {
		t.Errorf("expected context error, got: %v", callErr)
	}
}

func TestBridgeEndToEnd(t *testing.T) {
	ps, r, w := newPipeServer()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()                                  // notifications/initialized
		ps.respond(toolsListResp(2, defaultTools)) // initial list in New
		ps.respond(toolsListResp(3, defaultTools)) // list in RegisterAll
		ps.respond(toolsCallResp(4, "7", false))   // dispatch "add"
	}()

	tr := NewIOTransport(r, w, 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	reg := toolregistry.New(toolregistry.Options{})
	if err := RegisterAll(reg, c, BridgeOptions{}); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	// Dispatch "add" through the registry.
	block := llmclient.ToolUseBlock("call-1", "add", json.RawMessage(`{"a":3,"b":4}`))
	result := reg.Dispatch(context.Background(), block)
	if result.IsError {
		t.Fatalf("Dispatch returned is_error: %s", result.Content)
	}
	if result.Content != "7" {
		t.Errorf("result = %q, want \"7\"", result.Content)
	}
	<-done
}

func TestBridgeVerifiedPinDispatch(t *testing.T) {
	ps, r, w := newPipeServer()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()
		ps.respond(toolsListResp(2, defaultTools)) // initial list in New
		ps.respond(toolsListResp(3, defaultTools)) // list in RegisterAll
		ps.respond(toolsListResp(4, defaultTools)) // verified handler list
		ps.respond(toolsCallResp(5, "verified bridge", false))
	}()

	c, err := New(context.Background(), NewIOTransport(r, w, 0), Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	reg := toolregistry.New(toolregistry.Options{})
	if err := RegisterAll(reg, c, BridgeOptions{VerifyPinBeforeCall: true}); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	result := reg.Dispatch(context.Background(), llmclient.ToolUseBlock("call-verified", "echo", json.RawMessage(`{}`)))
	if result.IsError || result.Content != "verified bridge" {
		t.Errorf("Dispatch = %+v, want verified bridge success", result)
	}
	<-done
}

func TestBridgeHandlerInBandError(t *testing.T) {
	ps, r, w := newPipeServer()
	go func() {
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()
		ps.respond(toolsListResp(2, defaultTools))
		ps.respond(toolsListResp(3, defaultTools))
		ps.respond(toolsCallResp(4, "boom", true))
	}()

	tr := NewIOTransport(r, w, 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	reg := toolregistry.New(toolregistry.Options{})
	if err := RegisterAll(reg, c, BridgeOptions{}); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	block := llmclient.ToolUseBlock("call-2", "echo", json.RawMessage(`{"text":"x"}`))
	result := reg.Dispatch(context.Background(), block)
	if !result.IsError {
		t.Fatalf("expected is_error, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "boom") {
		t.Errorf("result %q does not contain 'boom'", result.Content)
	}
}

func TestBridgeOversizedResult(t *testing.T) {
	ps, r, w := newPipeServer()
	big := strings.Repeat("x", maxResultBytes+1)

	// We need a large IOTransport limit to receive the big response.
	// Build a custom transport with a generous max.
	go func() {
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()
		ps.respond(toolsListResp(2, defaultTools))
		ps.respond(toolsListResp(3, defaultTools))
		ps.respond(toolsCallResp(4, big, false))
	}()

	tr := NewIOTransport(r, w, maxResultBytes*4) // transport allows it through
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	reg := toolregistry.New(toolregistry.Options{})
	if err := RegisterAll(reg, c, BridgeOptions{}); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	block := llmclient.ToolUseBlock("call-3", "echo", json.RawMessage(`{"text":"x"}`))
	result := reg.Dispatch(context.Background(), block)
	if !result.IsError {
		t.Fatalf("expected is_error for oversized result, got success")
	}
	if !strings.Contains(result.Content, "exceeds") {
		t.Errorf("error %q does not mention 'exceeds'", result.Content)
	}
}

// ─── IOTransport unit tests ───────────────────────────────────────────────────

func TestIOTransportRoundtrip(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	// t1 sends on w2, receives on r1.
	// t2 sends on w1, receives on r2.
	t1 := NewIOTransport(r1, w2, 0)
	t2 := NewIOTransport(r2, w1, 0)

	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	recv := make(chan []byte, 1)
	go func() {
		b, err := t2.Receive(context.Background())
		if err != nil {
			recv <- nil
			return
		}
		recv <- b
	}()

	if err := t1.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := <-recv
	if string(got) != string(msg) {
		t.Errorf("got %q, want %q", got, msg)
	}
	t1.Close()
	t2.Close()
}

func TestIOTransportOversizedMessage(t *testing.T) {
	r, w := io.Pipe()
	tr := NewIOTransport(r, w, 16)

	go func() {
		w.Write([]byte(strings.Repeat("x", 20) + "\n"))
	}()

	_, err := tr.Receive(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q does not mention 'exceeds'", err)
	}
}

func TestIOTransportCRLF(t *testing.T) {
	r, w := io.Pipe()
	tr := NewIOTransport(r, w, 0)
	msg := []byte(`{"id":1}`)
	go func() { w.Write(append(msg, '\r', '\n')) }()

	got, err := tr.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if string(got) != string(msg) {
		t.Errorf("got %q, want %q", got, msg)
	}
}

func TestIOTransportClosedSend(t *testing.T) {
	r, w := io.Pipe()
	tr := NewIOTransport(r, w, 0)
	tr.Close()
	if err := tr.Send(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error on Send after Close")
	}
}

// ─── toolHash collision resistance ───────────────────────────────────────────

func TestToolHashStability(t *testing.T) {
	h1 := toolHash("echo", "echo text", json.RawMessage(`{"type":"object"}`), nil)
	h2 := toolHash("echo", "echo text", json.RawMessage(`{"type":"object"}`), nil)
	if h1 != h2 {
		t.Errorf("toolHash not stable: %s != %s", h1, h2)
	}
	// Different description → different hash.
	h3 := toolHash("echo", "MODIFIED", json.RawMessage(`{"type":"object"}`), nil)
	if h1 == h3 {
		t.Error("toolHash collision: description change not detected")
	}
	// Concatenation-collision resistance: "ab"/"c" vs "a"/"bc".
	h4 := toolHash("ab", "c", json.RawMessage(`{}`), nil)
	h5 := toolHash("a", "bc", json.RawMessage(`{}`), nil)
	if h4 == h5 {
		t.Error("toolHash: concatenation collision between name/description split")
	}
}
