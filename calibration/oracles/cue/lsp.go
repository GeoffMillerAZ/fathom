package main

// lsp.go — a minimal Language Server Protocol client for `cue lsp serve`, the
// cue binary's own language server (cuelang.org/go/cmd/cue v0.16.1 ships it as
// the `lsp` subcommand). JSON-RPC 2.0 over stdio with Content-Length framing;
// requests are answered in order, server-to-client requests (configuration,
// capability registration, progress) are acknowledged with an empty result.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type lspPos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPos `json:"start"`
	End   lspPos `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type docSymbol struct {
	Name           string      `json:"name"`
	Detail         string      `json:"detail"`
	Kind           int         `json:"kind"`
	Range          lspRange    `json:"range"`
	SelectionRange lspRange    `json:"selectionRange"`
	Children       []docSymbol `json:"children"`
}

type rpcMsg struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type lspClient struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	out      *bufio.Reader
	mu       sync.Mutex
	nextID   int
	encoding string // "utf-8" or "utf-16", what the server chose
	timeout  time.Duration
}

// startLSP starts `<cue> lsp serve` in root (a module root) and initializes
// it with root as its one workspace folder.
func startLSP(ctx context.Context, cueBin, root string, env []string, timeout time.Duration) (*lspClient, error) {
	cmd := exec.CommandContext(ctx, cueBin, "lsp", "serve")
	cmd.Dir = root
	cmd.Env = env
	cmd.Stderr = io.Discard
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &lspClient{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20), timeout: timeout, encoding: "utf-16"}
	uri := fileURI(root)
	var initRes struct {
		Capabilities struct {
			PositionEncoding string `json:"positionEncoding"`
		} `json:"capabilities"`
	}
	err = c.call("initialize", map[string]any{
		"processId":        os.Getpid(),
		"rootUri":          uri,
		"workspaceFolders": []map[string]string{{"uri": uri, "name": "subject"}},
		"capabilities": map[string]any{
			"general":      map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}},
			"textDocument": map[string]any{"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true}},
		},
	}, &initRes)
	if err != nil {
		c.close()
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if e := initRes.Capabilities.PositionEncoding; e != "" {
		c.encoding = e
	}
	if err := c.notify("initialized", map[string]any{}); err != nil {
		c.close()
		return nil, err
	}
	return c, nil
}

func fileURI(p string) string { return "file://" + p }

func (c *lspClient) write(m any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = c.in.Write(b)
	return err
}

func (c *lspClient) read() (rpcMsg, error) {
	n := -1
	for {
		line, err := c.out.ReadString('\n')
		if err != nil {
			return rpcMsg{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, _ := strings.Cut(line, ":")
		if strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			n, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if n < 0 {
		return rpcMsg{}, fmt.Errorf("lsp: frame without Content-Length")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(c.out, b); err != nil {
		return rpcMsg{}, err
	}
	var m rpcMsg
	err := json.Unmarshal(b, &m)
	return m, err
}

func (c *lspClient) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// call sends one request and reads until its response, acknowledging any
// server request in between. A response slower than the client's timeout
// fails the call (and the session: the reader is abandoned).
func (c *lspClient) call(method string, params, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return err
	}
	type got struct {
		m   rpcMsg
		err error
	}
	for {
		ch := make(chan got, 1)
		go func() {
			m, err := c.read()
			ch <- got{m, err}
		}()
		var g got
		select {
		case g = <-ch:
		case <-time.After(c.timeout):
			return fmt.Errorf("lsp %s: no answer in %s", method, c.timeout)
		}
		if g.err != nil {
			return g.err
		}
		m := g.m
		if m.Method != "" {
			if m.ID != nil { // a server request: acknowledge it
				var res any
				if m.Method == "workspace/configuration" {
					var p struct {
						Items []any `json:"items"`
					}
					_ = json.Unmarshal(m.Params, &p)
					arr := make([]map[string]any, len(p.Items))
					for i := range arr {
						arr[i] = map[string]any{}
					}
					res = arr
				}
				if err := c.write(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": res}); err != nil {
					return err
				}
			}
			continue
		}
		if m.ID == nil {
			continue
		}
		var gotID int
		if err := json.Unmarshal(*m.ID, &gotID); err != nil || gotID != id {
			continue
		}
		if m.Error != nil {
			return fmt.Errorf("lsp %s: %d %s", method, m.Error.Code, m.Error.Message)
		}
		if result != nil && len(m.Result) > 0 && string(m.Result) != "null" {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	}
}

func (c *lspClient) open(path, text string) error {
	return c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path), "languageId": "cue", "version": 1, "text": text},
	})
}

func (c *lspClient) symbols(path string) ([]docSymbol, error) {
	var out []docSymbol
	err := c.call("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": fileURI(path)}}, &out)
	return out, err
}

func (c *lspClient) definition(path string, line, char int) ([]lspLocation, error) {
	var raw json.RawMessage
	err := c.call("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path)},
		"position":     lspPos{Line: line, Character: char},
	}, &raw)
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	var locs []lspLocation
	if raw[0] == '[' {
		err = json.Unmarshal(raw, &locs)
	} else {
		var one lspLocation
		if err = json.Unmarshal(raw, &one); err == nil {
			locs = []lspLocation{one}
		}
	}
	return locs, err
}

func (c *lspClient) close() {
	if c == nil || c.cmd == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = c.call("shutdown", nil, nil)
		_ = c.notify("exit", nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	_ = c.in.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}
