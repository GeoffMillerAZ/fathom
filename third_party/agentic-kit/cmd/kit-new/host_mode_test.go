package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestHostInstallPreservesModeUnderRestrictiveUmask(t *testing.T) {
	if os.Getenv("AGENTIC_KIT_TEST_UMASK") != "child" {
		shell, err := exec.LookPath("sh")
		if err != nil {
			t.Skip("POSIX shell unavailable for native-host umask test")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, shell, "-c", `umask 077; exec "$@"`, "sh", os.Args[0], "-test.run=^TestHostInstallPreservesModeUnderRestrictiveUmask$")
		cmd.Env = append(os.Environ(), "AGENTIC_KIT_TEST_UMASK=child")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("restrictive-umask subprocess: %v\n%s", err, out)
		}
		return
	}
	f := newHostFixture(t, "claude")
	f.write(t, ".claude/settings.json", []byte(`{"foreign":true}`))
	if err := f.root.Chmod(".claude/settings.json", 0644); err != nil {
		t.Fatal(err)
	}
	f.install(t)
	info, err := f.root.Stat(".claude/settings.json")
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("existing permissions changed: %v %v", info, err)
	}
}
