// Spike: drive the appkit-mcp server over stdio with a real MCP client and
// exercise every tool. Args: <server-binary> <kit-root>. Throwaway.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: probe <server-binary> <kit-root>")
		os.Exit(2)
	}
	ctx := context.Background()
	cmd := exec.Command(os.Args[1])
	cmd.Dir = os.Args[2]
	cmd.Stderr = os.Stderr

	c := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "v0"}, nil)
	cs, err := c.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer cs.Close()

	if init := cs.InitializeResult(); init != nil {
		fmt.Printf("connected to %s %s\n", init.ServerInfo.Name, init.ServerInfo.Version)
	}

	lt, err := cs.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list tools:", err)
		os.Exit(1)
	}
	names := make([]string, 0, len(lt.Tools))
	for _, t := range lt.Tools {
		names = append(names, t.Name)
	}
	fmt.Println("tools:", strings.Join(names, ", "))

	call(ctx, cs, "search_components", map[string]any{"query": "pricing three tiers", "framework": "react", "tailwind_version": 4, "limit": 3})
	call(ctx, cs, "get_component", map[string]any{"full_name": "Marketing.Page Sections.Pricing Sections.Three tiers", "framework": "react", "tailwind_version": 4, "mode": "light"})
	call(ctx, cs, "list_sections", map[string]any{})
	call(ctx, cs, "list_components", map[string]any{"section": "Ecommerce", "framework": "vue", "tailwind_version": 4, "mode": "none", "limit": 4})
	call(ctx, cs, "get_component_preview", map[string]any{"full_name": "Marketing.Page Sections.Pricing Sections.Three tiers", "framework": "react", "tailwind_version": 4, "mode": "light"})
	call(ctx, cs, "get_kit_guidance", map[string]any{})
	call(ctx, cs, "list_kit_decisions", map[string]any{})
	call(ctx, cs, "get_kit_decision", map[string]any{"id": "loopback"})
	call(ctx, cs, "get_component", map[string]any{"full_name": "Does.Not.Exist", "framework": "react"}) // error path
}

func call(ctx context.Context, cs *mcp.ClientSession, name string, args map[string]any) {
	fmt.Printf("\n=== %s %v ===\n", name, args)
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		fmt.Println("  CallTool error:", err)
		return
	}
	if res.IsError {
		fmt.Print("  [isError] ")
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Println(indent(truncate(tc.Text, 360)))
		}
	}
}

func truncate(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}
