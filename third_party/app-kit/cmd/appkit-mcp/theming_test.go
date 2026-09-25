package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRetargetUtility(t *testing.T) {
	cases := []struct {
		in     string
		review bool
	}{
		{"bg-indigo-600", true},
		{"hover:bg-indigo-700", true},
		{"focus:ring-indigo-500", true},
		{"text-white", true},
		{"text-gray-300", true},
		{"bg-pink-500", true},
		{"bg-blue-500", true},
		{"dark:bg-gray-800", true},
		{"dark:hover:text-white", true},
		{"bg-indigo-600/50", true},
		{"!text-white", true},
		{"hover:bg-blue-500!", true},
		{"[&:nth-child(2)]:bg-blue-500", true},
		{"dark:[&:is(button,a)]:hover:bg-indigo-700", true},
		{"bg-[color:var(--brand)]", true},
		{"[color:var(--brand)]", true},
		{"bg-[#ff0000]", true},
		{"dark:text-lg", false},
		{"dark:p-4", false},
		{"text-transparent", false},
		{"text-current", false},
		{"bg-accent", false},
		{"text-ink-inverse", false},
		{"hover:bg-surface-raised", false},
		{"dark:text-ink", false},
		{"ring-focus", false},
		{"flex", false},
		{"p-4", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := retargetUtility(c.in)
			if got.Original != c.in || got.Action != actionKeep || got.Result != "" {
				t.Fatalf("source class must be kept without a replacement: %+v", got)
			}
			if review := strings.HasPrefix(got.Note, "Review:"); review != c.review {
				t.Errorf("review = %v, want %v: %+v", review, c.review, got)
			}
		})
	}
}

func TestRetargetClassList(t *testing.T) {
	for _, in := range []string{
		"bg-indigo-600 text-white hover:bg-indigo-700 dark:bg-indigo-500 focus:ring-indigo-400",
		"dark:text-lg dark:p-4 dark:bg-gray-800 bg-white text-gray-900",
		"[&:nth-child(2)]:bg-blue-500 bg-[color:var(--brand)] [@supports(color:oklch(0_0_0))]:text-white",
		"  bg-accent\ttext-ink-inverse\nhover:bg-surface-raised  bg-accent ",
		"",
		" \n\t ",
	} {
		verdicts, rewritten := RetargetClassList(in)
		if rewritten != in {
			t.Errorf("original class string changed:\n got %q\nwant %q", rewritten, in)
		}
		tokens := strings.Fields(in)
		if len(verdicts) != len(tokens) {
			t.Fatalf("got %d verdicts for %d classes", len(verdicts), len(tokens))
		}
		for i, verdict := range verdicts {
			if verdict.Original != tokens[i] || verdict.Action != actionKeep || verdict.Result != "" {
				t.Errorf("class %d must be kept without an applied replacement: %+v", i, verdict)
			}
		}
	}
}

func TestThemingGuideMentionsKeyRules(t *testing.T) {
	g := themingGuide()
	for _, want := range []string{
		"Preserve the original design first",
		"candidate advice is never an automatic rewrite",
		"Pink does not establish danger", "blue does not establish accent",
		"Keep dark: classes", "colons inside brackets",
		"text-white is not evidence for faint text", "text-on-fill contrast",
		"Do not collapse their shades", "add consumer roles",
		"Source-reference selection", "Renderer selection",
		"Elements is an optional candidate", "not proof of app-kit compatibility",
		"ADR-0021", "selected layers/profile", "consumer's own build",
		"bg-accent", "ring-focus", "surface",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("theming guide missing %q", want)
		}
	}
	for _, retired := range []string{
		"collapse shades", "dark: color variants are redundant",
		"Color families → roles", "No Node —",
	} {
		if strings.Contains(g, retired) {
			t.Errorf("theming guide retains unsafe instruction %q", retired)
		}
	}
}

// Keep the existing plan-linked regression name: semantic variables in style
// and presentation attributes remain legitimate L3a token consumption.
func TestThemingGuideDoesNotBanInlineColor(t *testing.T) {
	g := themingGuide()
	for _, banned := range []string{"use utilities only", "inline color styles"} {
		if strings.Contains(strings.ToLower(g), banned) {
			t.Errorf("theming guide still bans L3a consumption with %q", banned)
		}
	}
	for _, want := range []string{"L3a", "L3b/L4", "var(--ak-color-", "literal", "not a universal", "ban on Node"} {
		if !strings.Contains(g, want) {
			t.Errorf("theming guide does not explain the scoped contract %q", want)
		}
	}
}

// appkit/ui/viz's TestVizEmitsNoClassAttribute verifies the renderer side.
func TestThemingGuideMatchesVizReality(t *testing.T) {
	if !strings.Contains(themingGuide(), "appkit/ui/viz emits no class attributes") {
		t.Error("theming guide must name the package whose L3a behavior it describes")
	}
}

func TestThemingMCPAdvisoryProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := mcp.NewServer(&mcp.Implementation{Name: "theming-test", Version: "test"}, nil)
	registerKitTools(server, testDocs())
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "theming-client-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range listed.Tools {
		if tool.Name != "get_kit_theming" {
			continue
		}
		found = true
		for _, want := range []string{"preserving the original design", "Review notes", "no automatic"} {
			if !strings.Contains(strings.ToLower(tool.Description), strings.ToLower(want)) {
				t.Errorf("advertised theming description missing %q: %s", want, tool.Description)
			}
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Error("typed theming schemas must remain available")
		}
	}
	if !found {
		t.Fatal("get_kit_theming tool disappeared")
	}

	const classes = "  bg-indigo-600 text-white\thover:bg-indigo-700 dark:text-lg\n[&:nth-child(2)]:bg-blue-500 bg-accent "
	for _, in := range []string{"", classes} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_kit_theming", Arguments: map[string]any{"classes": in},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError || len(result.Content) != 1 {
			t.Fatalf("theming call failed: %+v", result)
		}
		body, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatal("theming must retain text content")
		}
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var output themingOutput
		if err := json.Unmarshal(encoded, &output); err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if in == "" {
			if len(fields) != 1 || output.Guide != themingGuide() || body.Text != output.Guide {
				t.Fatalf("guide response shape or served identity changed: %s", encoded)
			}
			continue
		}
		if len(fields) != 2 || fields["rewritten"] == nil || fields["verdicts"] == nil {
			t.Fatalf("class response fields changed: %s", encoded)
		}
		if output.Rewritten != in || len(output.Verdicts) != 6 {
			t.Fatalf("protocol changed the original classes or lost verdicts: %+v", output)
		}
		for _, v := range output.Verdicts {
			if v.Action != actionKeep || v.Result != "" {
				t.Errorf("protocol suggests an applied rewrite without mapping context: %+v", v)
			}
		}
		if strings.Contains(string(encoded), "\"result\":") {
			t.Error("candidate advice must never appear in the applied result field")
		}
		if !strings.Contains(body.Text, in) || !strings.Contains(body.Text, "Review:") {
			t.Errorf("text-only clients must receive original classes and review advice: %s", body.Text)
		}
		if strings.Contains(body.Text, "✗ drop") {
			t.Error("text response still instructs the client to remove classes")
		}
	}
}
