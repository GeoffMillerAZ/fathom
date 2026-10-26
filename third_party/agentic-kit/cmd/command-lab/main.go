package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func run(args []string, w io.Writer) int {
	if len(args) < 1 {
		return emit(w, "text", map[string]any{"status": "incomplete", "code": "invalid_subcommand", "noActionAuthority": true}, 2)
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	format := fs.String("format", "text", "")
	manifest := fs.String("manifest", "", "")
	scopePath := fs.String("scope", "", "")
	policyPath := fs.String("policy", "", "")
	requestPath := fs.String("request", "", "")
	capturePath := fs.String("capture", "", "")
	if e := fs.Parse(args[1:]); e != nil || fs.NArg() != 0 || (*format != "text" && *format != "json") {
		return emit(w, *format, map[string]any{"status": "incomplete", "code": "invalid_flags", "noActionAuthority": true}, 2)
	}
	switch args[0] {
	case "audit":
		if *manifest == "" || *scopePath != "" || *policyPath != "" || *requestPath != "" || *capturePath != "" {
			return emit(w, *format, map[string]any{"status": "incomplete", "code": "invalid_flags", "noActionAuthority": true}, 2)
		}
		var m SourceManifest
		if e := readStrict(*manifest, &m); e != nil {
			return emit(w, *format, map[string]any{"status": "incomplete", "code": e.Error(), "noActionAuthority": true}, 2)
		}
		r := audit(m)
		code := 0
		if r.Status != "complete" {
			code = 2
		}
		return emit(w, *format, r, code)
	case "assess":
		if *manifest != "" || *scopePath == "" || *policyPath == "" || *requestPath == "" {
			return emit(w, *format, map[string]any{"status": "incomplete", "code": "invalid_flags", "noActionAuthority": true}, 2)
		}
		var s Scope
		var p Policy
		var r Operation
		for _, v := range []struct {
			path   string
			target any
		}{{*scopePath, &s}, {*policyPath, &p}, {*requestPath, &r}} {
			if e := readStrict(v.path, v.target); e != nil {
				return emit(w, *format, map[string]any{"status": "incomplete", "code": e.Error(), "noActionAuthority": true}, 2)
			}
		}
		var cap *Capture
		if *capturePath != "" {
			cap = &Capture{}
			if e := readStrict(*capturePath, cap); e != nil {
				return emit(w, *format, map[string]any{"status": "incomplete", "code": e.Error(), "noActionAuthority": true}, 2)
			}
		}
		out, e := assess(s, p, r, cap, decision.None{})
		if e != nil {
			return emit(w, *format, map[string]any{"status": "incomplete", "code": e.Error(), "noActionAuthority": true}, 2)
		}
		code := 0
		if out.Status == "blocked" {
			code = 1
		}
		return emit(w, *format, out, code)
	default:
		return emit(w, *format, map[string]any{"status": "incomplete", "code": "invalid_subcommand", "noActionAuthority": true}, 2)
	}
}
func emit(w io.Writer, format string, v any, code int) int {
	var b []byte
	var e error
	if format == "json" {
		b, e = json.Marshal(v)
	} else {
		b, e = json.MarshalIndent(v, "", "  ")
	}
	if e != nil {
		return 2
	}
	b = append(b, '\n')
	if _, e = fmt.Fprint(w, string(b)); e != nil {
		return 2
	}
	return code
}
func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
