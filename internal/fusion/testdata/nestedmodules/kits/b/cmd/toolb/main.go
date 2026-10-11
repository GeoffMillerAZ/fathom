// Command toolb is kits/b's binary.
package main

import (
	"os"

	"example.com/nestedroot/kits/b/internal/cmd"
)

func main() {
	os.Exit(cmd.NewRoot())
}
