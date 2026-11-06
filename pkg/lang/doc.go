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
// Withheld values. This is not a secret scanner, and a Result is not promised
// to carry no credential. It follows one rule about which of the file's own
// text it returns, and the rule fails closed. A Signature from a code dialect (a
// DialectInfo of Kind "language") on a list this package holds and its tests
// check, each of whose string and char literal forms opens with a quote
// character, is kept up to its first quote character (', " or a backtick) and
// reads *** from there on (javascript, typescript and scala also from their
// first <, where JSX text and XML literals open). There is no pairing and no grammar: a quote that
// opens no literal (a comment's apostrophe, a digit separator, a quote in a
// regex) cuts too, and only a Rust lifetime or label is kept. Over-masking is
// accepted; a value written before the first quote stays, and so do the
// prefix characters of a literal (r, b, u8, R, @, $, #, f). A Signature from any
// other code dialect is empty. A Signature from a dialect that is no code is
// empty unless the symbol's Kind is one whose Signature is structure and holds
// no value (a keyword or a declared name, on a list this package holds and its
// tests check), and a Signature that list keeps is cut at its first quote the
// same way, so the text of a script is never
// returned and a Kind added later is withheld until it is listed. A Fact value
// under a key whose last segment looks like a credential (password, passwd,
// secret, token, apikey, credential, private_key, in any case or separator)
// reads ***, as does a literal that a fact value binds with =, := or :; other
// Fact values are returned as written. The userinfo of a URL, written with or
// without its scheme (user:password@host, //user:password@host), is dropped
// wherever a string from the file lands, keeping the ssh account git and the
// image of a docker or oci reference that a digest follows; a userinfo ends at
// the first '/', '?' or '#', so the host of a URL is never changed. Notes carry
// a reason and a line, never the file's text. Everything else (a symbol's name,
// a reference target and its raw spelling, a value the rule does not name) is
// returned as written, and an image reference is available in Reference.To.
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
// hours (a TOML document of 40,000 keys, a proto file with a bracket open at
// its end). A refused file reads Incomplete with the reason in
// Notes. No other shape is promised refused or bounded in time or memory.
//
// The wazero runtime that runs the grammars is process-wide, so its on-disk
// compilation cache is a package function, EnableCompilationCache, and is
// off by default. Without it the package writes no file.
package lang

//go:generate go run levels_gen.go
