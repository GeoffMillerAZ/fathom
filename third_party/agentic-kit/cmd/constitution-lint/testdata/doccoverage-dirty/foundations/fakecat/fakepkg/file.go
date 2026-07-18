// Package fakepkg is a fixture package for the doc-coverage test.
package fakepkg

func NoDoc() int {
	return 1
}

// This comment does not start with the identifier's own name.
func BadDoc() int {
	return 2
}
