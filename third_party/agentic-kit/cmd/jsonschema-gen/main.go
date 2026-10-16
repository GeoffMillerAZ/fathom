// Command jsonschema-gen derives a JSON Schema (draft 2020-12) document from
// a CUE package directory and writes it to an output file (or stdout).
//
// It is a generation-time tool: the output is meant to be committed alongside
// the originating schema.cue so that runtime code can use the JSON Schema
// without depending on the cue binary.
//
// Usage:
//
//	go run ./cmd/jsonschema-gen -dir <cue-pkg-dir> -out <output.json>
//	go run ./cmd/jsonschema-gen -dir <cue-pkg-dir>          # stdout
//
// The cue binary must be on PATH (or set via -cue flag).  agentic-kit's CI
// installs cue; `task doctor` checks it.
//
// Examples:
//
//	# Derive schema from the circuit-breaker package:
//	go run ./cmd/jsonschema-gen -dir foundations/governance/circuit-breaker \
//	    -out workspace/schemas/circuitbreaker.json
//
//	# Derive and print to stdout:
//	go run ./cmd/jsonschema-gen -dir toolchains/schemas
//
// Exit codes: 0 on success, 1 on error (error written to stderr).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/jsonschema"
)

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("jsonschema-gen", flag.ContinueOnError)
	dir := fs.String("dir", ".", "CUE package directory to derive schema from")
	out := fs.String("out", "", "output file path; omit or use '-' for stdout")
	cue := fs.String("cue", jsonschemagen.DefaultCUEBinary, "path to the cue binary")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}

	// Resolve -dir relative to the working directory.
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsonschema-gen: resolve dir %q: %v\n", *dir, err)
		return 1
	}

	opts := jsonschemagen.Options{CUEBinary: *cue}
	schema, err := jsonschemagen.DeriveFromDir(absDir, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsonschema-gen: %v\n", err)
		return 1
	}

	// Write to stdout or file.
	if *out == "" || *out == "-" {
		if _, err := os.Stdout.Write(schema); err != nil {
			fmt.Fprintf(os.Stderr, "jsonschema-gen: write stdout: %v\n", err)
			return 1
		}
		return 0
	}

	absOut, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsonschema-gen: resolve out %q: %v\n", *out, err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "jsonschema-gen: mkdir %s: %v\n", filepath.Dir(absOut), err)
		return 1
	}
	if err := os.WriteFile(absOut, schema, 0o640); err != nil {
		fmt.Fprintf(os.Stderr, "jsonschema-gen: write %s: %v\n", absOut, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "jsonschema-gen: wrote %s\n", absOut)
	return 0
}
