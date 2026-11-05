// Package lang reads source files and configuration into facts: symbols,
// references, refused references and the facts a dialect declares, each with
// the level that readings measured for the dialect that read it.
//
// Bytes and a path go in. Nothing else is needed: no store, no daemon, no
// index, no git and no network. A Reader is safe for concurrent use and
// deterministic: equal inputs give byte-equal JSON.
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
// Refused files. A parser that recurses, or whose cost grows with the square of
// its input, cannot be interrupted, so a file is refused before it reaches one:
// it reads Incomplete with the reason in Notes. The guards count what each parser
// is known to recurse on or spend its time on (nesting past 10,000 levels, a run
// of more than 10,000 tokens no statement ends, more than 10,000 lexical errors,
// a TOML document of more than 40,000 keys). A shape no guard counts is not
// promised refused.
//
// The wazero runtime that runs the grammars is process-wide, so its on-disk
// compilation cache is a package function, EnableCompilationCache, and is
// off by default. Without it the package writes no file.
package lang

//go:generate go run levels_gen.go
