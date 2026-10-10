// Command worker is svc/worker's binary.
package main

import (
	"os"

	"example.com/worker/internal/cmd"
)

func main() {
	os.Exit(cmd.NewRoot())
}
