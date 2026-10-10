// Command api is svc/api's binary.
package main

import (
	"os"

	"example.com/api/internal/cmd"
)

func main() {
	os.Exit(cmd.NewRoot())
}
