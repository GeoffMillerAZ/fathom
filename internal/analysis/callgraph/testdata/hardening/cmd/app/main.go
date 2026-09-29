package main

import (
	"strings"

	"fixmod/iface"
	"fixmod/lib"
	"fixmod/other"
	"fixmod/reg"
)

func main() {
	_ = lib.Wired()
	_ = other.Render()
	var d iface.Doer = iface.Real{}
	_ = d.Do()
	_ = unusedHelper
	r := strings.NewReader("x")
	_ = r.Len() // strings.Reader's Len, never lib.Stack's
	var t reg.T
	_ = t.Run() // reg is linked only through a method
}

func unusedHelper() int { return lib.DeadViaUncalledMainHelper() }
