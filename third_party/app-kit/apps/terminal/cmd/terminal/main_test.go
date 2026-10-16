package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/terminal"
)

func TestJSONDoesNotEnterTerminalAndMatchesSharedCatalog(t *testing.T) {
	for _, args := range [][]string{{"--json"}, {"--json", "--query", "period filter", "--profile", "go-web"}, {"--json", "--query", "no-match-xyz"}} {
		var out, stderr bytes.Buffer
		root := newCommand(func() bool { t.Fatal("JSON checked TTY"); return false }, func(context.Context, *terminal.Model) error { t.Fatal("JSON entered TUI"); return nil })
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs(args)
		if err := root.ExecuteContext(context.Background()); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(&out)
		var got report
		if err := decoder.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			t.Fatalf("not one JSON report: %v", err)
		}
		if got.SchemaVersion != 1 || got.Outcome != "observed" || got.ExitCode != 0 || got.Page == nil || stderr.Len() != 0 {
			t.Fatalf("wrong machine report: %+v stderr=%q", got, stderr.String())
		}
		c, err := catalog.Default()
		if err != nil {
			t.Fatal(err)
		}
		query, _ := root.Flags().GetString("query")
		profile, _ := root.Flags().GetString("profile")
		want, err := c.Search(catalog.Query{Text: query, Profile: profile, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		a, _ := json.Marshal(got.Page)
		b, _ := json.Marshal(want)
		if !bytes.Equal(a, b) || bytes.Contains(a, []byte{0x1b}) {
			t.Fatal("terminal JSON changed shared metadata or emitted terminal escapes")
		}
	}
}

func TestTerminalRefusesPipesWithoutReadingOrRunning(t *testing.T) {
	root := newCommand(func() bool { return false }, func(context.Context, *terminal.Model) error { t.Fatal("entered TUI"); return nil })
	root.SetArgs(nil)
	err := root.ExecuteContext(context.Background())
	if command.Code(err) != 2 || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("pipe refusal = %v", err)
	}
}

func TestTerminalUsageQueryAndCancellationFailures(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"unexpected"}, {"--profile", "unknown"}, {"--query", "\x1b[2J"}} {
		root := newCommand(func() bool { return true }, func(context.Context, *terminal.Model) error { t.Fatal("invalid input entered TUI"); return nil })
		root.SetArgs(args)
		if code := command.Code(root.ExecuteContext(context.Background())); code != 2 {
			t.Fatalf("args=%q code=%d", args, code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := newCommand(func() bool { t.Fatal("cancelled startup checked TTY"); return true }, nil)
	root.SetArgs(nil)
	if err := root.ExecuteContext(ctx); command.Code(err) != 2 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled startup: %v", err)
	}
}

func TestJSONQueryFailureAndWriterFailure(t *testing.T) {
	var out bytes.Buffer
	root := newCommand(func() bool { t.Fatal("JSON checked TTY"); return false }, nil)
	root.SetOut(&out)
	root.SetArgs([]string{"--json", "--profile", "unknown"})
	if code := command.Code(root.ExecuteContext(context.Background())); code != 2 {
		t.Fatalf("query error code=%d", code)
	}
	var got report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.ExitCode != 2 || got.Outcome != "could-not-inspect" || got.Page != nil || got.Error == "" {
		t.Fatalf("wrong failure envelope: %+v %v", got, err)
	}
	root = newCommand(func() bool { return false }, nil)
	root.SetOut(failedWriter{})
	root.SetArgs([]string{"--json"})
	if code := command.Code(root.ExecuteContext(context.Background())); code != 2 {
		t.Fatalf("writer failure code=%d", code)
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
