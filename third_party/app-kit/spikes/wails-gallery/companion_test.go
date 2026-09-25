package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCompanionPinsPrivateExecutable(t *testing.T) {
	source := companionFixture(t, []byte("synthetic companion\n"), 0700)
	digest := fileSHA256(t, source)
	prepared, err := prepareCompanion(source, digest)
	if err != nil {
		t.Fatal(err)
	}
	path, dir := prepared.path, prepared.dir
	if path == source || filepath.Base(path) != "gallery-native" || filepath.Dir(path) != dir {
		t.Fatal("companion was not staged at its private fixed path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0500 {
		t.Fatal("staged companion does not have the required private executable mode")
	}
	if got := fileSHA256(t, path); got != digest {
		t.Fatal("staged companion identity changed")
	}
	prepared.close()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("staged executable survived cleanup")
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("private staging directory survived cleanup")
	}
}

func TestPrepareCompanionRejectsUntrustedIdentity(t *testing.T) {
	executable := companionFixture(t, []byte("synthetic companion\n"), 0700)
	digest := fileSHA256(t, executable)
	symlink := filepath.Join(t.TempDir(), "companion-link")
	if err := os.Symlink(executable, symlink); err != nil {
		t.Fatal(err)
	}
	notExecutable := companionFixture(t, []byte("not executable\n"), 0600)
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name, path, digest string
	}{
		{"digest mismatch", executable, strings.Repeat("0", 64)},
		{"missing", missing, digest},
		{"symlink", symlink, digest},
		{"not executable", notExecutable, fileSHA256(t, notExecutable)},
		{"invalid SHA length", executable, "abcd"},
		{"invalid SHA case", executable, strings.ToUpper(digest)},
		{"relative path", filepath.Base(executable), digest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prepared, err := prepareCompanion(test.path, test.digest)
			if prepared != nil || err == nil {
				if prepared != nil {
					prepared.close()
				}
				t.Fatal("accepted untrusted companion identity")
			}
			if strings.Contains(err.Error(), test.path) || strings.Contains(err.Error(), test.digest) {
				t.Fatal("identity error exposed selected material")
			}
		})
	}
}

func TestPrepareCompanionSourceChangeCannotChangePrivateCopy(t *testing.T) {
	original := bytes.Repeat([]byte("a"), 4096)
	source := companionFixture(t, original, 0700)
	digest := fileSHA256(t, source)
	prepared, err := prepareCompanion(source, digest)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.close()
	if err := os.WriteFile(source, bytes.Repeat([]byte("b"), len(original)), 0700); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(t, prepared.path); got != digest || got == fileSHA256(t, source) {
		t.Fatal("private copy followed a later source mutation")
	}
}

func TestRunRejectsMutuallyExclusiveCompanionFlagsBeforeLaunch(t *testing.T) {
	digest := strings.Repeat("0", 64)
	tests := [][]string{
		{"-native-host", "/missing"},
		{"-native-host-sha256", digest},
		{"-browser", "-native-host", "/missing", "-native-host-sha256", digest},
		{"-browser", "-native-child"},
		{"-native-child", "-native-host", "/missing", "-native-host-sha256", digest},
		{"unexpected-positional"},
	}
	for _, args := range tests {
		var diagnostics strings.Builder
		if err := run(args, &diagnostics); err == nil {
			t.Fatalf("accepted conflicting flags %q", args)
		}
		output := diagnostics.String()
		if strings.Contains(output, "token=") || strings.Contains(output, "Experimental synthetic Gallery at") {
			t.Fatalf("flag rejection launched or exposed authority: %q", output)
		}
	}
}

func TestCompanionRepliesAcceptOnlyMatchingOutstandingReply(t *testing.T) {
	tests := []struct {
		request string
		reply   string
	}{
		{"launch-request", "launch"},
		{"browser", "browser-ok"},
		{"browser", "browser-failed"},
	}
	for _, test := range tests {
		t.Run(test.request+"_"+test.reply, func(t *testing.T) {
			var replies companionReplies
			if !replies.begin(test.request) || !replies.accept(test.reply) {
				t.Fatal("rejected the matching request/reply pair")
			}
			if replies.accept(test.reply) {
				t.Fatal("accepted a duplicate reply")
			}
		})
	}
}

func TestCompanionRepliesRejectUnsolicitedAndInvalidStateTransitions(t *testing.T) {
	for _, reply := range []string{"launch", "browser-ok", "browser-failed", "ready", "closed"} {
		var replies companionReplies
		if replies.accept(reply) {
			t.Fatalf("accepted unsolicited %q reply", reply)
		}
	}
	for _, request := range []string{"", "launch", "ready", "closed", "browser-ok"} {
		var replies companionReplies
		if replies.begin(request) {
			t.Fatalf("accepted invalid request kind %q", request)
		}
	}

	var replies companionReplies
	if !replies.begin("launch-request") {
		t.Fatal("could not begin valid launch request")
	}
	if replies.begin("browser") {
		t.Fatal("accepted a second outstanding request")
	}
	if replies.accept("browser-ok") {
		t.Fatal("accepted the wrong reply kind")
	}
	if !replies.accept("launch") {
		t.Fatal("wrong reply cleared the pending request")
	}
	if !replies.begin("browser") || !replies.accept("browser-failed") {
		t.Fatal("completed request did not release the reply gate")
	}
}

func companionFixture(t *testing.T, content []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "selected-private-binary-canary")
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
