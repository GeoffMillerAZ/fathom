package main

// observe_verb.go registers the `observe` verb (EW-45) in a file of its own, so
// the registration never edits the file that defines its handler,
// cmdObserve (observe.go).

import "io"

// observeRunner runs `fathom observe` (registered below; EW-45).
type observeRunner struct{}

func (observeRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdObserve(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "observe", Group: "operate", Order: 80, Run: observeRunner{},
		Summary: "File a structured friction observation as its own file, .fathom/observations/O<n>.md"})
}
