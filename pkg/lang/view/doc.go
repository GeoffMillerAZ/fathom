// Package view answers span questions over a lang.Result: which symbol
// encloses a line or a line range, which symbols a line range touches, and what
// bytes a symbol's span holds in the caller's own source. It also nests a Result's
// symbols into an outline of the file (Outline).
//
// Provisional: fields and functions may change in a minor release until the
// LP-8 readings pass and one release has shipped (CH-D5). The package imports
// pkg/lang and the standard library only. Its closure is pkg/lang's plus this
// package, so a consumer of view links one package more than a consumer of
// pkg/lang alone.
//
// A symbol has a known span when EndLine >= Line > 0. A symbol without one is
// never guessed: a lookup the Result cannot decide answers StatusUnknown, and
// a Touched set says it is incomplete. Nothing here reads a file or adds text a
// Result does not already carry, except Slice, which returns the caller's own
// bytes to the caller.
package view
