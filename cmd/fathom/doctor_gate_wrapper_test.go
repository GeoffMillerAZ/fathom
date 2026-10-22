package main

import (
	"strings"
	"testing"
)

func TestDoctorComposedPushGate(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       doctorStatus
	}{
		{"both", "task validate\nfathom verify --strict --since origin/main\n", doctorPass},
		{"missing-verify", "task validate\n", doctorFail},
		{"missing-validate", "fathom verify --strict\n", doctorFail},
		{"non-strict", "task validate\nfathom verify\n", doctorFail},
		{"neither", "echo gate\n", doctorFail},
		{"comment-only", "# task validate\n# fathom verify --strict\n", doctorFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    gate:\n      run: sh scripts/push.sh\n")
			doctorWriteFixture(t, root, "scripts/push.sh", "#!/bin/sh\nexec ./scripts/gate.sh\n")
			doctorWriteFixture(t, root, "scripts/gate.sh", tc.body)
			got := checkLefthookGates(root)
			if got.Status != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
			if tc.want == doctorPass && !strings.Contains(got.Reason, "scripts/gate.sh") {
				t.Fatalf("proof file absent: %+v", got)
			}
		})
	}
}

func TestDoctorNativePushGateWrapper(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\nexec ./scripts/gate.sh\n")
	doctorWriteFixture(t, root, "scripts/gate.sh", "task validate\nfathom verify --strict\n")
	got := checkLefthookGates(root)
	if got.Status != doctorPass || !strings.Contains(got.Reason, "scripts/gate.sh") {
		t.Fatalf("native wrapper = %+v", got)
	}
}
