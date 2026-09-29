package extractor

import (
	"bufio"
	"bytes"
	"go/build/constraint"
	"regexp"
	"strings"
)

// buildconstraint.go — EW-15: which Go files no default build compiles.
//
// A file whose build constraint no default build satisfies — `//go:build
// ignore` (a `go run` generator), `//go:build tools`, a custom tag such as
// `//go:build integration` — is indexed like any other, but its symbols are
// marked core.ToolKindGoBuildExcluded so the product wiring check never
// counts its `func main` as an entry point. "Default" means no custom tags:
// the constraint is evaluated for every known GOOS/GOARCH pair, with and
// without cgo, under the gc toolchain and every go1.N release tag, and a
// file any one of those builds compiles is not excluded. The answer is a
// function of the file's bytes alone (never the host's GOOS), so it is
// deterministic across machines.

// knownOS, unixOS and knownArch mirror go/build's syslist.
var (
	knownOS = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux",
		"nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos"}
	unixOS = map[string]bool{"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
		"hurd": true, "illumos": true, "ios": true, "linux": true, "netbsd": true, "openbsd": true, "solaris": true}
	knownArch = []string{"386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "loong64", "mips", "mipsle",
		"mips64", "mips64le", "mips64p32", "mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390",
		"s390x", "sparc", "sparc64", "wasm"}
	releaseTag = regexp.MustCompile(`^go1\.[0-9]+$`)
)

// defaultBuildExcludes reports whether src's build constraint — its
// `//go:build` line, else its legacy `// +build` lines, in the header
// before the package clause — is satisfied by no default build. A file
// with no constraint, or one that does not parse, is not excluded.
func defaultBuildExcludes(src []byte) bool {
	expr, ok := headerConstraint(src)
	if !ok {
		return false
	}
	for _, goos := range knownOS {
		for _, goarch := range knownArch {
			for _, cgo := range []bool{false, true} {
				if expr.Eval(func(tag string) bool { return defaultTag(tag, goos, goarch, cgo) }) {
					return false
				}
			}
		}
	}
	return true
}

// defaultTag reports whether tag is set in the default build for goos,
// goarch and cgo — the tags go/build's matchTag sets without -tags.
func defaultTag(tag, goos, goarch string, cgo bool) bool {
	switch {
	case tag == goos || tag == goarch || tag == "gc":
		return true
	case tag == "unix":
		return unixOS[goos]
	case tag == "cgo":
		return cgo
	case tag == "linux":
		return goos == "android"
	case tag == "darwin":
		return goos == "ios"
	case tag == "solaris":
		return goos == "illumos"
	}
	return releaseTag.MatchString(tag)
}

// headerConstraint returns the file's build constraint expression, read
// the way the go tool reads it: comment lines (and blank lines) before the
// first line of code, a //go:build line winning over // +build lines.
func headerConstraint(src []byte) (constraint.Expr, bool) {
	var (
		goBuild  constraint.Expr
		plus     []constraint.Expr
		inBlock  bool
		sc       = bufio.NewScanner(bytes.NewReader(src))
		haveGoBd bool
	)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case inBlock:
			if i := strings.Index(line, "*/"); i >= 0 {
				inBlock = false
				if strings.TrimSpace(line[i+2:]) != "" {
					return result(goBuild, haveGoBd, plus)
				}
			}
			continue
		case line == "":
			continue
		case strings.HasPrefix(line, "/*"):
			if !strings.Contains(line[2:], "*/") {
				inBlock = true
			}
			continue
		case !strings.HasPrefix(line, "//"):
			return result(goBuild, haveGoBd, plus)
		case constraint.IsGoBuild(line):
			if e, err := constraint.Parse(line); err == nil && !haveGoBd {
				goBuild, haveGoBd = e, true
			}
		case constraint.IsPlusBuild(line):
			if e, err := constraint.Parse(line); err == nil {
				plus = append(plus, e)
			}
		}
	}
	return result(goBuild, haveGoBd, plus)
}

func result(goBuild constraint.Expr, haveGoBuild bool, plus []constraint.Expr) (constraint.Expr, bool) {
	if haveGoBuild {
		return goBuild, true
	}
	if len(plus) == 0 {
		return nil, false
	}
	e := plus[0]
	for _, p := range plus[1:] {
		e = &constraint.AndExpr{X: e, Y: p}
	}
	return e, true
}
