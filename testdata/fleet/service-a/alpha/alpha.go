// Package alpha is a tiny fixture service exposing an exported function.
package alpha

// Greet returns a greeting string for the given name.
func Greet(name string) string {
	return "hello, " + name
}

// Version is an exported constant consumed by cross-repo callers.
const Version = "1.0.0"
