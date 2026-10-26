// Command scanner-canary emits deterministic fictitious scanner test material.
// It creates no credential and calls no service. Independent runs share no state.
// Keep generated material in a private qualification workspace, not source control.
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
)

func main() {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewPCG(1701, 831))
	value := make([]byte, 36)
	for i := range value {
		value[i] = alphabet[rng.IntN(len(alphabet))]
	}
	// Split the marker from its generated body so no credential-shaped value
	// is stored in this source file. This is a detector canary, not a token.
	out := fmt.Sprintf("token = %q\n", "ghp_"+string(value))
	if n, err := fmt.Fprint(os.Stdout, out); err != nil || n != len(out) {
		os.Exit(2)
	}
}
