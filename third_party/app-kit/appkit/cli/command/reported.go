package command

import (
	"io"

	"charm.land/fang/v2"
)

// Reported marks err as already rendered by the caller. The returned error
// keeps err's message, unwrap chain, and any exit code carried by Exit.
//
// Reporting is deliberately explicit: an error's text, type, or exit code
// never implies that it has already been shown to the user.
func Reported(err error) error {
	if err == nil {
		return nil
	}
	return &reportedError{err: err}
}

type reportedError struct {
	err error
}

func (e *reportedError) Error() string { return e.err.Error() }
func (e *reportedError) Unwrap() error { return e.err }
func (e *reportedError) reported()     {}

// UnreportedErrorHandler returns a Fang error handler that renders only errors
// not already acknowledged with Reported. A nil next handler uses Fang's
// default renderer. A nil error is ignored.
func UnreportedErrorHandler(next fang.ErrorHandler) fang.ErrorHandler {
	if next == nil {
		next = fang.DefaultErrorHandler
	}

	return func(w io.Writer, styles fang.Styles, err error) {
		if err == nil || fullyReported(err) {
			return
		}
		next(w, styles, err)
	}
}

type reportingMarker interface {
	reported()
}

type singleUnwrapper interface {
	Unwrap() error
}

type multiUnwrapper interface {
	Unwrap() []error
}

// fullyReported follows ordinary wrapping but treats a multi-error as reported
// only when every non-nil branch is reported. A marker around the multi-error
// acknowledges the whole value; a marker on only one joined branch does not.
func fullyReported(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(reportingMarker); ok {
		return true
	}
	if joined, ok := err.(multiUnwrapper); ok {
		hasError := false
		for _, child := range joined.Unwrap() {
			if child == nil {
				continue
			}
			hasError = true
			if !fullyReported(child) {
				return false
			}
		}
		return hasError
	}
	if wrapped, ok := err.(singleUnwrapper); ok {
		return fullyReported(wrapped.Unwrap())
	}
	return false
}
