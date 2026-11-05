// Package lang reads source files and configuration into facts: symbols,
// references, refused references and the facts a dialect declares, each with
// the level that readings measured for the dialect that read it.
//
// Bytes and a path go in. Nothing else is needed: no store, no daemon, no
// index, no git and no network. A Reader is safe for concurrent use and
// deterministic: equal inputs give byte-equal JSON, except where a time limit
// decides (a read that overruns its deadline, a Reader that is busy).
//
// The package is a door over fathom's own extraction code (ADR-092). Public
// types are defined here, so a change inside fathom does not change this
// API. Symbol.Canonical is a closed vocabulary that only grows; Symbol.Kind
// is each dialect's own name and may change as a language improves.
//
// Withheld values. A Result is not promised to carry no credential, but it
// withholds the places one is found. The userinfo of a URL is dropped wherever a
// string from the file lands (a name, a key, a reference target or its raw
// spelling, an unresolved target, a fact), keeping an ssh account that has no
// password and the image of a docker or oci reference; a userinfo ends at the
// first '/', '?' or '#', so the host of a URL is never changed. A string literal
// a Signature binds to a name or a key is masked as *** where it is a parameter's
// default, a rule's value or a dictionary entry, and in every fact value. The text
// of a script (a package.json or composer.json script, any task) is not returned,
// only the program it runs, because a credential can sit in a script in more
// shapes than a pattern covers. A credential flag (--password, -u user:pass) and
// an Authorization header are masked in the rest of a Signature. A credential in
// any other place (a name, a label, a literal that is an argument, a block's own
// attributes) is returned as written.
//
// Reading hostile input. Two boundaries, and each promises only what it can
// keep. In process, Extract returns by a deadline (DefaultReadDeadline, lowered
// by WithLimits): a read that overruns it is Incomplete, with one note naming
// the deadline and none of its facts. Go cannot interrupt a goroutine, so the
// parser is abandoned and keeps its CPU, and what it allocates, until it
// returns on its own. Once GOMAXPROCS abandoned reads are running, Extract
// starts nothing more and says SkipBusy. A fatal stack overflow, a parser that
// never returns and a parser that allocates without bound end or starve the
// calling process whatever the deadline says. The process boundary keeps those:
// cmd/fathom-lang reads in a supervised child by default, kills it on an
// overrun, a death or a memory limit, and the file in hand reads Incomplete
// while the batch continues. This package starts no process, so a consumer
// that reads input it does not trust uses that boundary.
//
// Guards stay where nothing in process survives a failure: a parser that
// recurses on nesting is handed no file nested past 10,000 levels, because a
// stack overflow is fatal. A few parsers also refuse a shape that cost them
// hours (a run of tokens no statement ends, a flood of lexical errors, a TOML
// document of 40,000 keys). A refused file reads Incomplete with the reason in
// Notes. No other shape is promised refused or bounded in time or memory.
//
// The wazero runtime that runs the grammars is process-wide, so its on-disk
// compilation cache is a package function, EnableCompilationCache, and is
// off by default. Without it the package writes no file.
package lang

//go:generate go run levels_gen.go
