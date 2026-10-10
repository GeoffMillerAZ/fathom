// Command toola is kits/a's binary: its main is a product entry point.
package main

import (
	"os"

	"example.com/nestedroot/kits/a/internal/cmd"
)

func main() {
	os.Exit(cmd.NewRoot().Execute())
}
