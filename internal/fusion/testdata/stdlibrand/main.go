package main

import "math/rand/v2"

func roll() int { return rand.IntN(6) }

func main() { _ = roll() }
