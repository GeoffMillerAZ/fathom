package main

// servehttp_verb.go registers the `serve-http` verb (EW-45) in a file of its own, so
// the registration never edits the file that defines its handler,
// runServeHTTP (servehttp.go).

import "io"

// serveHTTPRunner runs `fathom serve-http` (registered below; EW-45).
type serveHTTPRunner struct{}

func (serveHTTPRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runServeHTTP(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "serve-http", Group: "serve", Order: 20, Run: serveHTTPRunner{},
		Summary: "Serve the MCP JSON-RPC protocol over HTTP (POST /rpc)"})
}
