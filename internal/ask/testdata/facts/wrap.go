package facts

import "fmt"

type logger struct{}

func (logger) Errorf(format string, args ...any) error { return nil }

// want: unchecked=no wrapped=yes executes_only=- locked=- threaded=-
func wrapsW(err error) error {
	return fmt.Errorf("open: %w", err)
}

// want: unchecked=no wrapped=no executes_only=- locked=- threaded=-
func wrapsV(err error) error {
	return fmt.Errorf("open: %v", err)
}

// want: unchecked=no wrapped=no executes_only=- locked=- threaded=-
// One of two error arguments is formatted with %s.
func mixed(a, b error) error {
	return fmt.Errorf("%w and %s", a, b)
}

// want: unchecked=no wrapped=yes executes_only=- locked=- threaded=-
// %% consumes no argument; %d and %q fall on an int and a string.
func wrapsAfterOthers(n int, s string, err error) error {
	return fmt.Errorf("100%% %d %q: %w", n, s, err)
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
func noErrArg(s string) error {
	return fmt.Errorf("bad %s", s)
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: an Errorf method that is not fmt.Errorf.
func otherErrorf(l logger, err error) error {
	return l.Errorf("x: %v", err)
}

// want: unchecked=no wrapped=no executes_only=- locked=- threaded=-
// A concrete error type implements error: formatted with %v, not wrapped.
func concreteV(e *myErr) error {
	return fmt.Errorf("x: %v", e)
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: err.Error() is a string, not an error value.
func stringified(err error) error {
	return fmt.Errorf("x: %s", err.Error())
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// A non-constant format is not examined.
func dynamicFormat(f string, err error) error {
	return fmt.Errorf(f, err)
}
