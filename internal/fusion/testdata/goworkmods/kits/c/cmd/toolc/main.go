// Command toolc is kits/c's binary.
package main

import (
	"os"

	"example.com/workroot/kits/c/internal/cmd"
)

func main() {
	os.Exit(cmd.NewRoot())
}
