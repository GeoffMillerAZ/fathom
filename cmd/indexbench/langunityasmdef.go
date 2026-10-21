package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cmdLangUnityAsmdef calibrates declared assembly facts against an independently
// retained Unity Editor receipt. It opens no store, invokes no Editor, and calls
// no model or daemon. A partial/unjudged comparison cannot exit successfully.
func cmdLangUnityAsmdef(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lang-unity-asmdef", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("project", "", "Unity project root (required)")
	oracle := flags.String("oracle", "", "retained CompilationPipeline JSON (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *root == "" || *oracle == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-unity-asmdef -project PATH -oracle RECEIPT.json")
		return 2
	}
	raw, err := os.ReadFile(*oracle)
	if err != nil {
		fmt.Fprintln(stderr, "lang-unity-asmdef:", err)
		return 2
	}
	reading, err := langbench.CalibrateUnityAsmdefs(os.DirFS(*root), raw)
	if err != nil {
		fmt.Fprintln(stderr, "lang-unity-asmdef:", err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(reading); err != nil {
		fmt.Fprintln(stderr, "lang-unity-asmdef:", err)
		return 2
	}
	if !reading.Passed || !reading.Complete {
		return 1
	}
	return 0
}
