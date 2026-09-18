package main

// flagreorder.go — J6/O8 (app-kit dogfood): `fathom layers install <dir>
// --root DIR` documented flags AFTER the positional pack directory, but
// Go's flag.FlagSet.Parse stops scanning for flags at the first non-flag
// token — so the documented order (`layers install <dir> --root .`) was
// silently mis-parsed as THREE positional arguments ("<dir>", "--root",
// ".") and refused with "requires exactly one pack directory", while the
// undocumented `layers install --root . <dir>` order worked. The local
// workaround (O8's own recorded fix) was "always put flags first"; this
// closes the actual parser gap instead.
import (
	"flag"
	"strings"
)

// reorderFlagsFirst rearranges args so every token fs recognizes as a flag
// (and, for a flag taking a value as a SEPARATE token, that value) precedes
// every positional argument, preserving each group's own relative order —
// the standard trick that lets flag.FlagSet, which only ever scans a
// LEADING run of flags, accept them in any position. A bool flag's value
// token is consumed only in the "--name=value" form (exactly how
// flag.Parse itself treats a bool flag — a following bare token is never
// swallowed as its value). "--" stops all reordering: it and everything
// after it are left as positionals, unchanged, matching flag.Parse's own
// "-- ends flag processing" contract. A token that names no flag fs
// defines is left as a positional — fs.Parse itself still reports the
// real "flag provided but not defined" error once reordering is done, at
// whatever position that leaves it.
func reorderFlagsFirst(fs *flag.FlagSet, args []string) []string {
	var flagsOut, posOut []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			posOut = append(posOut, args[i:]...)
			break
		}
		name, isFlagToken := flagName(a)
		if !isFlagToken {
			posOut = append(posOut, a)
			i++
			continue
		}
		hasEq := strings.IndexByte(name, '=') >= 0
		if hasEq {
			name = name[:strings.IndexByte(name, '=')]
		}
		fl := fs.Lookup(name)
		if fl == nil {
			posOut = append(posOut, a)
			i++
			continue
		}
		flagsOut = append(flagsOut, a)
		i++
		if hasEq || isBoolFlag(fl) {
			continue
		}
		if i < len(args) {
			flagsOut = append(flagsOut, args[i])
			i++
		}
	}
	return append(flagsOut, posOut...)
}

// flagName strips a's leading "-"/"--" and reports whether a is shaped like
// a flag token at all (starts with "-", is not bare "-", and is not a
// negative-number-shaped token like "-1").
func flagName(a string) (name string, ok bool) {
	if len(a) < 2 || a[0] != '-' {
		return "", false
	}
	if strings.HasPrefix(a, "--") {
		return a[2:], len(a) > 2
	}
	if a[1] >= '0' && a[1] <= '9' {
		return "", false // "-1", not a flag
	}
	return a[1:], true
}

// isBoolFlag reports whether f's Value implements the unexported
// flag.boolFlag interface (IsBoolFlag() bool) — the same test flag.Parse
// itself uses to decide a bare following token is never that flag's value.
func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}
