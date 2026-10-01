package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// retryIO runs op, retrying a "operation not permitted" a few times: on the
// machine this oracle was written on, a long run under an external volume can
// have its filesystem access revoked mid-run (a TCC verdict that a reset
// clears). With ORACLE_TCC_RESET set to a command line's argv0 and args
// (ORACLE_TCC_RESET="tccutil reset SystemPolicyRemovableVolumes <bundle>"),
// that reset is run between attempts. A read or write that still fails is
// returned, never swallowed: a silently missing file would be a silently
// missing truth.
func retryIO(op func() error) error {
	var err error
	for i := 0; i < 6; i++ {
		if err = op(); err == nil || !(errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrPermission)) {
			return err
		}
		if cmdline := os.Getenv("ORACLE_TCC_RESET"); cmdline != "" {
			_ = exec.Command("sh", "-c", cmdline).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return err
}

func readFile(p string) (b []byte, err error) {
	err = retryIO(func() error { var e error; b, e = os.ReadFile(p); return e })
	return b, err
}

func writeFile(p string, b []byte) error {
	return retryIO(func() error { return os.WriteFile(p, b, 0o644) })
}
