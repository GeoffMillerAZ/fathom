package gatereceipt

import (
	"fmt"
	"regexp"
	"strings"
)

var endpointLine = regexp.MustCompile(`^fathom gate: ([A-Za-z0-9_.:-]+) (START|PASS|FAIL)(?: exit=[0-9]+)? [^ ]+ tree=([0-9a-f]+) clean=([01]) commit=([0-9a-f]+)$`)

// logIdentity binds both endpoints to one clean source tree. Legacy logs
// without these fields are not evidence for receipt acceptance.
func logIdentity(b []byte, command, tree string) (string, error) {
	lines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	start := endpointLine.FindStringSubmatch(strings.TrimSuffix(lines[0], "\r"))
	end := endpointLine.FindStringSubmatch(strings.TrimSuffix(lines[len(lines)-1], "\r"))
	// Task's own trailer can follow a framed failure, never a success.
	if end == nil && len(lines) > 1 && taskFailureLine.MatchString(lines[len(lines)-1]) {
		end = endpointLine.FindStringSubmatch(strings.TrimSuffix(lines[len(lines)-2], "\r"))
		if end != nil && end[2] != "FAIL" {
			end = nil
		}
	}
	if start == nil || end == nil || start[1] != command || end[1] != command || start[2] != "START" || end[2] == "START" || start[3] != tree || end[3] != tree || start[4] != "1" || end[4] != "1" || start[5] != end[5] {
		return "", fmt.Errorf("gate log endpoints must name the same clean tree %s and commit", tree)
	}
	return start[5], nil
}

type environment struct{ Version, OS, Arch, Work, CGO, Flags string }

const envHeader = "fathom gate: go env GOOS GOARCH GOWORK CGO_ENABLED GOFLAGS"

func logEnvironment(b []byte) (environment, error) {
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) < 8 || !strings.HasPrefix(lines[1], "go version ") || lines[2] != envHeader || lines[3] == "" || lines[4] == "" || (lines[6] != "0" && lines[6] != "1") {
		return environment{}, fmt.Errorf("gate log has no toolchain/environment header")
	}
	return environment{Version: lines[1], OS: lines[3], Arch: lines[4], Work: lines[5], CGO: lines[6], Flags: lines[7]}, nil
}
