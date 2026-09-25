package command

import "errors"

// Exit is the cli-kit's 0/1/2 process-exit convention, carried through as
// an ordinary error so a RunE (or any deeper call) can report a code
// without touching os.Exit directly. The convention itself is inherited
// from platform-engineering's doctor commands (kits/env-kit/internal/doctor,
// cli/cmd/ci_doctor.go): 0 = ok, 1 = checked and broken (the command ran
// to completion and found a real problem), 2 = could not check (usage
// error, missing dependency, or anything that stopped the check from
// running at all — never confused with "checked and it's bad").
//
// Exit(code, err) wraps err with the code; Code(err) recovers it. A plain,
// unwrapped error defaults to exit 1 (checked and broken) — 2 is never
// guessed, only explicit.
func Exit(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }

// ExplicitCode reports the first code carried by an Exit-wrapped error.
// The boolean distinguishes an explicit code (including zero or one) from
// Code's defaults for nil and otherwise unclassified errors.
func ExplicitCode(err error) (int, bool) {
	var ex *exitError
	if errors.As(err, &ex) {
		return ex.code, true
	}
	return 0, false
}

// Code reports the process exit code err carries: 0 for nil, the code an
// Exit-wrapped error names (found via errors.As, so a wrapped/joined error
// still resolves), and 1 for any other non-nil error.
func Code(err error) int {
	if code, ok := ExplicitCode(err); ok {
		return code
	}
	if err == nil {
		return 0
	}
	return 1
}
