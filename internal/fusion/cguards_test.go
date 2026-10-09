package fusion

import (
	"reflect"
	"sort"
	"testing"
)

// TestEvalCConditionThreeValued (O468): an #if is known true, known false or
// unknown; only what the build is known to define settles it.
func TestEvalCConditionThreeValued(t *testing.T) {
	env := cHostEnv("darwin")
	for m, v := range defineFlags([]string{"cc", "-DHAVE_CONFIG_H", "-DLEVEL=3", "-D", "EMPTY=", "-UNDEBUG"}) {
		env[m] = v
	}
	cases := []struct {
		expr string
		want tri
	}{
		{"defined(_WIN32)", triFalse},
		{"defined _WIN32 || defined(__VMS)", triFalse},
		{"defined(__APPLE__)", triTrue},
		{"defined(HAVE_CONFIG_H)", triTrue},
		{"defined(NDEBUG)", triFalse},
		{"defined(HAVE_POLL_H)", triUnknown},                  // a configure result: unknown
		{"defined(_WIN32) && defined(HAVE_POLL_H)", triFalse}, // a known false side settles &&
		{"defined(__APPLE__) || defined(HAVE_POLL_H)", triTrue},
		{"defined(__APPLE__) && defined(HAVE_POLL_H)", triUnknown},
		{"LEVEL >= 2", triTrue},
		{"LEVEL > 3", triFalse},
		{"_MSC_VER >= 1900", triFalse}, // an undefined macro reads 0
		{"__GNUC__ > 3 && !defined(_WIN32)", triTrue},
		{"0", triFalse},
		{"1", triTrue},
		{"0x10 == 16", triTrue},
		{"__has_include(<poll.h>)", triUnknown},
		{"EMPTY", triUnknown},
		{"defined(__APPLE__) ? 1 : 0", triTrue},
		{"(", triUnknown},
	}
	for _, c := range cases {
		if got := evalCCondition(c.expr, env); got != c.want {
			t.Errorf("evalCCondition(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

// TestCExcludedLinesKeepsUnknown: a branch known not taken is excluded, with
// its #elif and #else chain; an unknown condition keeps every branch, and a
// #define on a kept line becomes known.
func TestCExcludedLinesKeepsUnknown(t *testing.T) {
	src := `#ifndef GUARD_H
#define GUARD_H
#ifdef HAVE_CONFIG_H
#include "config.h"
#else
#ifdef _WIN32
#include "config-win.h"
#endif
#endif
#if defined(_WIN32)
#include "win.h"
#elif defined(__APPLE__)
#include "mac.h"
#else
#include "other.h"
#endif
#ifdef HAVE_POLL_H
#include "poll.h"
#endif
#define USE_LOCAL 1
#if USE_LOCAL
#include "local.h"
#endif
/* #ifdef _WIN32 in a comment
#include "nothing.h" */
#endif
`
	env := cHostEnv("darwin")
	env["HAVE_CONFIG_H"] = cMacro{defined: true, value: "1"}
	got := cExcludedLines([]byte(src), env)
	var lines []int
	for l := range got {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	// 6-8: inside the #else of a known-defined HAVE_CONFIG_H; 11: _WIN32's
	// branch; 15: the #else after a taken #elif (a group's own #else and
	// #endif lines belong to the level around it). GUARD_H and HAVE_POLL_H
	// (unknown) keep theirs, and USE_LOCAL, #defined under GUARD_H's unknown
	// branch, stays unknown and keeps its include too.
	want := []int{6, 7, 8, 11, 15}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("excluded lines %v, want %v", lines, want)
	}
	if cExcludedLines([]byte(src), nil) != nil {
		t.Error("no compile database (nil env): nothing may be excluded")
	}
}

// TestCTargetOf: the target from a compile command's flags.
func TestCTargetOf(t *testing.T) {
	for args, want := range map[string]string{
		"cc -arch arm64 -c a.c":                     "darwin",
		"cc -mmacosx-version-min=14.0 -c a.c":       "darwin",
		"clang --target=x86_64-unknown-linux-gnu":   "linux",
		"clang -target aarch64-linux-android21 a.c": "linux",
		"clang --target=x86_64-pc-windows-msvc":     "windows",
		"clang --target=riscv64-unknown-elf":        "other",
		"gcc -O2 -c a.c":                            "",
	} {
		if got := cTargetOf(splitCommand(args)); got != want {
			t.Errorf("cTargetOf(%q) = %q, want %q", args, got, want)
		}
	}
	if cHostEnv("other") != nil {
		t.Error("an unprofiled target knows no platform macro")
	}
}
