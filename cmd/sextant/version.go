package main

import (
	"fmt"
	"io"
)

// Version is the current sextant CLI version.
const Version = "sextant v0.1.0"

func cmdVersion(_ []string, stdout, _ io.Writer) int {
	fmt.Fprintln(stdout, Version)
	return 0
}

func cmdHelp(_ []string, stdout, _ io.Writer) int {
	printUsage(stdout)
	return 0
}
