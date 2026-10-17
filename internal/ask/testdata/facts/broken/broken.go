// Package broken fails to type-check: every span in it is excluded, never guessed.
package broken

import "errors"

// Fine would compute cleanly, but its package does not load.
func Fine() error {
	return errors.New("x")
}

// Bad returns a string where an int is declared: a type error.
func Bad() int {
	return "not an int"
}
