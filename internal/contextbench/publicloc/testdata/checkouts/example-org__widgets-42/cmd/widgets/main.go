package main

import (
	"fmt"

	"example.com/widgets/pkg/store"
)

func main() {
	Run()
}

// Run wires the store.
func Run() {
	s := store.Open()
	v, _ := s.Get("k")
	fmt.Println(v)
}
