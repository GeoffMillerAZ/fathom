// Package facts is the hand-labelled fixture for internal/ask's spanFacts.
// Every function carrying a "want:" line is one span; the line names the
// expected value of each of the five facts: yes (the fact applies and
// holds), no (it applies and does not hold), - (it does not apply).
package facts

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type myErr struct{}

func (*myErr) Error() string { return "my" }

func mayFail() error { return errors.New("x") }

func pair() (int, error) { return 0, nil }

func lookup(k string) (string, bool) { return k, k != "" }

func concrete() *myErr { return nil }

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// Every error is checked; a deferred Close is a defer statement, not an
// expression statement.
func checksAll(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := mayFail(); err != nil {
		return err
	}
	n, err := pair()
	_ = n
	return err
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
func dropsBare() {
	mayFail()
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
func dropsBlankTuple(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: the blanks fall on an int and on a bool, never on an
// error result.
func blankOnNonError(s string) error {
	_, err := pair()
	v, _ := lookup(s)
	_ = v
	return err
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
// fmt.Println returns (int, error): it counts like any other call.
func printsOnly() {
	fmt.Println("hi")
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
func noErrorCalls(s string) string {
	return strings.ToUpper(s)
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
// A blank assignment inside a function literal counts.
func dropsInClosure() func() {
	return func() { _ = mayFail() }
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: a go statement is not an expression statement.
func goStatement() {
	go mayFail()
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
func blankVar() {
	var _ = mayFail()
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: *myErr implements error but is not the error type.
func dropsConcrete() {
	concrete()
}

// want: unchecked=yes wrapped=- executes_only=- locked=- threaded=-
func removesBare(p string) {
	os.Remove(p)
}
