package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

const pyStrict = `["fathom", "verify", "--strict"]`

// pyGate writes a native pre-push hook that runs `task validate`, whose
// Taskfile runs step, plus files, and returns the lefthook_gates row.
func pyGate(t *testing.T, step string, files map[string]string) doctorCheck {
	t.Helper()
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - "+step+"\n")
	for p, c := range files {
		doctorWriteFixture(t, root, p, c)
	}
	return checkLefthookGates(root)
}

func pyWant(t *testing.T, got doctorCheck, status doctorStatus, contains ...string) {
	t.Helper()
	if got.Status != status {
		t.Fatalf("status %s, want %s: %+v", got.Status, status, got)
	}
	for _, c := range contains {
		if !strings.Contains(got.Reason, c) {
			t.Fatalf("reason %q does not name %q", got.Reason, c)
		}
	}
	if status == doctorUnknown && doctorExitCode([]doctorCheck{got}) != 0 {
		t.Fatal("an uncertain Python step must fail open")
	}
	if status == doctorUnknown && strings.Contains(got.Reason, "non-shell script doctor cannot read further") {
		t.Fatalf("a read Python file kept the node/ruby/perl wording: %q", got.Reason)
	}
}

// R1: a closed spawn set with literal argv reads PASS and names the file.
func TestDoctorPythonGateFindsLiteralSpawns(t *testing.T) {
	for name, body := range map[string]string{
		"list":             "import subprocess\nsubprocess.run(" + pyStrict + ")\n",
		"tuple":            "import subprocess\nsubprocess.run((\"fathom\", \"verify\", \"--strict\"))\n",
		"alias":            "import subprocess as sp\nsp.check_call(" + pyStrict + ")\n",
		"from import":      "from subprocess import check_call\ncheck_call(" + pyStrict + ")\n",
		"from import as":   "from subprocess import run as r\nr(" + pyStrict + ", check=True)\n",
		"args keyword":     "import subprocess\nsubprocess.run(args=" + pyStrict + ", check=True)\n",
		"execvp":           "import os\nos.execvp(\"fathom\", " + pyStrict + ")\n",
		"execlp":           "import os\nos.execlp(\"fathom\", \"fathom\", \"verify\", \"--strict\")\n",
		"os.system":        "import os\nos.system(\"fathom verify --strict\")\n",
		"shell=True":       "import subprocess\nsubprocess.run(\"fathom verify --strict\", shell=True)\n",
		"which or literal": "import shutil, subprocess\nFATHOM = shutil.which(\"fathom\") or \"fathom\"\nsubprocess.run([FATHOM, \"verify\", \"--strict\"])\n",
		"environ default":  "import os, subprocess\nFATHOM = os.environ.get(\"FATHOM\", \"fathom\")\nsubprocess.run([FATHOM, \"verify\", \"--strict\"])\n",
		"dynamic tail":     "import subprocess, sys\nFATHOM = \"fathom\"\nplans = sys.argv[1:]\nsubprocess.run([FATHOM, \"verify\", \"--strict\"] + plans)\n",
		"implicit concat":  "import subprocess\nsubprocess.run([\n    \"fathom\",  # the binary\n    \"verify\", \"--str\" \"ict\",\n])\n",
		"argv name":        "import subprocess\nCMD = " + pyStrict + "\nsubprocess.run(CMD)\n",
		"docstring first":  "\"\"\"Gate: never runs on import.\"\"\"\nimport subprocess\n\n\ndef main():\n    subprocess.run(" + pyStrict + ")\n\n\nif __name__ == \"__main__\":\n    main()\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": body})
			pyWant(t, got, doctorPass, "scripts/gate.py", ".git/hooks/pre-push")
		})
	}
}

// The same bytes in a docstring, comment, f-string, print, dead assignment
// or a non-fathom program never PASS: they read FAIL and name the file.
func TestDoctorPythonGateNoFalsePass(t *testing.T) {
	for name, body := range map[string]string{
		"docstring":  "\"\"\"fathom verify --strict\"\"\"\nimport subprocess\nsubprocess.run([\"true\"])\n",
		"comment":    "# subprocess.run([\"fathom\", \"verify\", \"--strict\"])\nimport subprocess\n",
		"f-string":   "x = 1\nprint(f\"fathom verify --strict {x}\")\n",
		"print":      "print(" + pyStrict + ")\n",
		"dead argv":  "argv = ['fathom', 'verify', '--strict']\n",
		"echo":       "import subprocess\nsubprocess.run([\"echo\", \"fathom\", \"verify\", \"--strict\"])\n",
		"non-strict": "import subprocess\nsubprocess.run([\"fathom\", \"verify\"])\n",
		"no import":  "os.execvp(\"fathom\", " + pyStrict + ")\n",
		"echo shell": "import os\nos.system(\"echo fathom verify --strict\")\n",
		"inert only": "import json, sys\nprint(json.dumps(sys.argv))\n",
		"re.compile": "import re\nre.compile(\"fathom\")\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": body})
			pyWant(t, got, doctorFail, "scripts/gate.py")
			if got.Remedy != "wire both commands in the repository-local pre-push wrapper" {
				t.Fatalf("remedy changed: %+v", got)
			}
		})
	}
}

// R3: what static reading cannot settle reads UNKNOWN naming file:line and
// the cause. Every fixture mentions fathom.
func TestDoctorPythonGateUncertainNamesFileLine(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"argv from a call", "import subprocess\ndef build():\n    return " + pyStrict + "\nsubprocess.run(build())\n", "scripts/gate.py:4"},
		{"f-string program", "import subprocess\nBIN = \"x\"\nsubprocess.run([f\"{BIN}/fathom\", \"verify\", \"--strict\"])\n", "scripts/gate.py:3"},
		{"dynamic tail without strict", "import subprocess, sys\nsubprocess.run([\"fathom\", \"verify\"] + sys.argv[1:])\n", "--strict is dynamic"},
		{"bound twice", "import subprocess\nFATHOM = \"fathom\"\nFATHOM = \"x\"\nsubprocess.run([FATHOM, \"verify\", \"--strict\"])\n", "scripts/gate.py:4"},
		{"for target", "import subprocess\nfor FATHOM in [\"fathom\"]:\n    subprocess.run([FATHOM, \"verify\", \"--strict\"])\n", "scripts/gate.py:3"},
		{"mutated argv", "import subprocess\nc = " + pyStrict + "\nc.pop()\nsubprocess.run(c)\n", "scripts/gate.py:4"},
		{"getattr", "import subprocess\ngetattr(subprocess, \"run\")(" + pyStrict + ")\n", "getattr"},
		{"passed by reference", "import subprocess\nrunner = subprocess.run\nrunner(" + pyStrict + ")\n", "without being called"},
		{"exec", "exec(open(\"x.py\").read())  # fathom\n", "exec("},
		{"cwd", "import subprocess\nsubprocess.run(" + pyStrict + ", cwd=\"sub\")\n", "cwd="},
		{"shell list", "import subprocess\nsubprocess.run(" + pyStrict + ", shell=True)\n", "shell=True"},
		{"shell not literal", "import subprocess\nflag = True\nsubprocess.run(" + pyStrict + ", shell=flag)\n", "shell="},
		{"pty.spawn", "import pty\npty.spawn(" + pyStrict + ")\n", "spawn form not read"},
		{"loop.subprocess_exec", "import asyncio\nasync def f(loop):\n    await loop.subprocess_exec(None, \"fathom\", \"verify\", \"--strict\")\n", "spawn form not read"},
		{"sh -c", "import subprocess\nsubprocess.run([\"sh\", \"-c\", \"fathom verify --strict\"])\n", "scripts/gate.py:2"},
		{"project-local import", "import helpers\nhelpers.run()  # fathom verify --strict\n", "import helpers"},
		{"third-party import", "import requests  # fathom\n", "import requests"},
		{"unterminated string", "x = \"fathom verify --strict\n", "unterminated string"},
		{"parameter", "import subprocess\ndef run(cmd):\n    subprocess.run(cmd)\nrun(" + pyStrict + ")\n", "scripts/gate.py:3"},
		{"star kwargs", "import subprocess\nopts = {}\nsubprocess.run(" + pyStrict + ", **opts)\n", "**kwargs"},
		{"import star", "from subprocess import *\nrun(" + pyStrict + ")\n", "import *"},
		{"executable", "import subprocess\nsubprocess.run(" + pyStrict + ", executable=\"/bin/echo\")\n", "executable="},
		{"escape", "import subprocess\nsubprocess.run([\"fath\\x6fm\", \"verify\", \"--strict\"])\n", "scripts/gate.py:2"},
		{"rebound alias", "import subprocess\nsubprocess = None\nsubprocess.run(" + pyStrict + ")\n", "rebound"},
		{"shell substitution", "import os\nos.system(\"fathom verify --strict $(echo x)\")\n", "backtick"},
		{"spawn then bad string", "import subprocess\nsubprocess.run(" + pyStrict + ")\nx = \"abc\n", "scripts/gate.py:3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": tc.body})
			pyWant(t, got, doctorUnknown, "reaches task validate", tc.want)
		})
	}
}

// P7's mention rule keeps unrelated Python work out of the gate's verdict.
func TestDoctorPythonGateMentionRule(t *testing.T) {
	lint := map[string]string{"scripts/lint.py": "import yaml\nprint(1)\n"}
	pyWant(t, pyGate(t, "python3 scripts/lint.py", lint), doctorFail)
	pyWant(t, pyGate(t, "python3 scripts/lint.py verify", lint), doctorUnknown, "import yaml")
	pyWant(t, pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": "import requests\nimport subprocess\nsubprocess.run(" + pyStrict + ")\n"}), doctorPass)
}

// The adopter's chain: a literal-spawn script behind two Taskfile hops is a
// depth cut while D1 is none, named at its stopping point and its target.
func TestDoctorPythonAdopterChainIsDepthCut(t *testing.T) {
	root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - task fathom:audit\n      - task fathom:verify\n  fathom:audit:\n    cmds: [python3 Tools/audit.py]\n  fathom:verify:\n    cmds: [python3 Tools/project.py fathom-verify]\n")
	doctorWriteFixture(t, root, "Tools/audit.py", "print('audit')\n")
	doctorWriteFixture(t, root, "Tools/project.py", "import subprocess\nsubprocess.run("+pyStrict+")\n")
	pyWant(t, checkLefthookGates(root), doctorUnknown, "depth limit", "Taskfile.yml (task fathom:verify)", "Tools/project.py")
}

// A script the hook runs directly may hand off to a task; the same spawn one
// level deeper is a cut naming both ends.
func TestDoctorPythonSpawnsTask(t *testing.T) {
	pre := "import subprocess\nsubprocess.run([\"task\", \"fathom:verify\"])\n"
	tasks := "version: '3'\ntasks:\n  validate:\n    cmds: [CMD]\n  fathom:verify:\n    cmds: [fathom verify --strict]\n"
	t.Run("from the hook", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\npython3 scripts/pre.py\n")
		doctorWriteFixture(t, root, "Taskfile.yml", strings.Replace(tasks, "CMD", "echo ok", 1))
		doctorWriteFixture(t, root, "scripts/pre.py", pre)
		pyWant(t, checkLefthookGates(root), doctorPass, "scripts/pre.py", "Taskfile.yml")
	})
	t.Run("from task validate", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
		doctorWriteFixture(t, root, "Taskfile.yml", strings.Replace(tasks, "CMD", "python3 scripts/pre.py", 1))
		doctorWriteFixture(t, root, "scripts/pre.py", pre)
		pyWant(t, checkLefthookGates(root), doctorUnknown, "depth limit", "scripts/pre.py", "Taskfile.yml (task fathom:verify)")
	})
}

// Doctor reads a wrapper and never runs it.
func TestDoctorPythonNeverExecutes(t *testing.T) {
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [python3 scripts/gate.py verify]\n")
	doctorWriteFixture(t, root, "scripts/gate.py", "import subprocess\nopen(\"executed\", \"w\").close()\nsubprocess.run("+pyStrict+")\n")
	if err := os.Chmod(filepath.Join(root, "scripts/gate.py"), 0o755); err != nil {
		t.Fatal(err)
	}
	pyWant(t, checkLefthookGates(root), doctorPass)
	if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
		t.Fatalf("the Python step ran: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "executed")); !os.IsNotExist(err) {
		t.Fatalf("the Python step ran: %v", err)
	}
}

func TestDoctorPythonBounds(t *testing.T) {
	taskfile := "version: '3'\ntasks:\n  validate:\n    cmds:\n      - python3 scripts/gate.py verify\n"
	spawn := "import subprocess\nsubprocess.run(" + pyStrict + ")\n"
	budget := doctorWrapperBytes - len(taskfile) // a probe reads the Taskfile, then the script
	t.Run("fits the budget exactly", func(t *testing.T) {
		body := "#" + strings.Repeat("x", budget-len(spawn)-2) + "\n" + spawn
		pyWant(t, pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": body}), doctorPass)
	})
	t.Run("over the remaining budget", func(t *testing.T) {
		body := "#" + strings.Repeat("x", budget) + "\n" + spawn
		pyWant(t, pyGate(t, "python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": body}), doctorUnknown, "budget")
	})
	t.Run("FIFO", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
		doctorWriteFixture(t, root, "Taskfile.yml", taskfile)
		if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(root, "scripts/gate.py"), 0o600); err != nil {
			t.Skip(err)
		}
		pyWant(t, checkLefthookGates(root), doctorUnknown, "scripts/gate.py")
	})
	t.Run("symlink out of the root", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
		doctorWriteFixture(t, root, "Taskfile.yml", taskfile)
		outside := filepath.Join(t.TempDir(), "gate.py")
		if err := os.WriteFile(outside, []byte(spawn), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "scripts/gate.py")); err != nil {
			t.Fatal(err)
		}
		pyWant(t, checkLefthookGates(root), doctorUnknown, "scripts/gate.py")
	})
	t.Run("expired deadline reads nothing", func(t *testing.T) {
		root := t.TempDir()
		doctorWriteFixture(t, root, "scripts/x.py", spawn)
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
		defer cancel()
		r := doctorWrapperReader{ctx: ctx, root: root, remaining: 100}
		got := r.command("python3 scripts/x.py verify", "prompt-hook", 0)
		if got.found || !got.unknown || r.remaining != 100 {
			t.Fatalf("expired walk = %+v, remaining %d", got, r.remaining)
		}
		doctorWriteFixture(t, root, "scripts/mcp.py", "import os\nos.execvp(\"fathom\", [\"fathom\", \"serve-mcp\"])\n")
		if row := doctorStdioRegistration(ctx, root, filepath.Join(root, ".mcp.json"), "python3", []string{"scripts/mcp.py"}); row.Status != doctorTimeout {
			t.Fatalf("expired stdio walk = %+v", row)
		}
	})
}

func TestDoctorPythonDeterministic(t *testing.T) {
	files := map[string]string{"scripts/gate.py": "import requests\nimport helpers  # fathom\nexec('x')\n"}
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [python3 scripts/gate.py verify]\n")
	for p, c := range files {
		doctorWriteFixture(t, root, p, c)
	}
	a, b := checkLefthookGates(root), checkLefthookGates(root)
	if !reflect.DeepEqual(a, b) || !strings.Contains(a.Reason, "scripts/gate.py:1") {
		t.Fatalf("rows differ or name a later cause: %+v / %+v", a, b)
	}
}

// S6: module wrappers, launchers, interpreter flags and shebangs (P1, M1).
func TestDoctorPythonModulesLaunchersAndShebangs(t *testing.T) {
	gate := "import subprocess\nsubprocess.run(" + pyStrict + ")\n"
	for _, tc := range []struct {
		name, step string
		files      map[string]string
		want       doctorStatus
		names      string
	}{
		{"module file", "python3 -m tools.gate verify", map[string]string{"tools/gate.py": gate}, doctorPass, "tools/gate.py"},
		{"module package", "python3 -m tools.gate verify", map[string]string{"tools/gate/__main__.py": gate}, doctorPass, "tools/gate/__main__.py"},
		{"module both forms", "python3 -m tools.gate verify", map[string]string{"tools/gate.py": "x = 1\n", "tools/gate/__main__.py": gate}, doctorUnknown, "tools.gate"},
		{"src layout", "python3 -m tools.gate verify", map[string]string{"src/tools/gate.py": gate}, doctorUnknown, "src/"},
		{"installed module", "python3 -m pytest verify", nil, doctorFail, ""},
		{"uv run python", "uv run python scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"poetry run python", "poetry run python scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"uv run module", "uv run python -m tools.gate verify", map[string]string{"tools/gate.py": gate}, doctorPass, "tools/gate.py"},
		{"interpreter -W", "python3 -W ignore scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"interpreter flags", "python3 -B -u -X dev scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"venv interpreter", ".venv/bin/python3.12 scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"executable .py", "./scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorPass, "scripts/gate.py"},
		{"launcher option", "uv run --project x python scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorUnknown, "--project"},
		{"entry point", "uv run gate verify", nil, doctorUnknown, "entry point"},
		{"python -c", "python3 -c pass verify", nil, doctorUnknown, "-c"},
		{"unknown flag", "python3 -Bu scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorUnknown, "-Bu"},
		{"unrelated launcher", "poetry run pytest", nil, doctorFail, ""},
		{"absolute interpreter", "/usr/bin/python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorFail, ""},
		{"env interpreter", "/usr/bin/env python3 scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorFail, ""},
		{"absent script", "python3 scripts/absent.py verify", nil, doctorFail, ""},
		{"python shebang", "./scripts/gate verify", map[string]string{"scripts/gate": "#!/usr/bin/env python3\n" + gate}, doctorPass, "scripts/gate"},
		{"python shebang with ()", "./scripts/gate verify", map[string]string{"scripts/gate": "#!/usr/bin/env python3\n" + gate + "def main():\n    pass\nmain()\n"}, doctorPass, "scripts/gate"},
		{"python shebang docstring", "./scripts/gate verify", map[string]string{"scripts/gate": "#!/usr/bin/python3\n\"\"\"\nfathom verify --strict\n\"\"\"\n"}, doctorFail, "scripts/gate"},
		{"node shebang", "./scripts/gate verify", map[string]string{"scripts/gate": "#!/usr/bin/env node\nrequire('x')\n"}, doctorUnknown, "non-shell script doctor cannot read further"},
		{"other python", "pythonw scripts/gate.py verify", map[string]string{"scripts/gate.py": gate}, doctorUnknown, "non-shell script doctor cannot read further"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := pyGate(t, tc.step, tc.files)
			if got.Status != tc.want || !strings.Contains(got.Reason, tc.names) {
				t.Fatalf("got %+v, want %s naming %q", got, tc.want, tc.names)
			}
			if tc.want == doctorFail && got.Remedy != "wire both commands in the repository-local pre-push wrapper" {
				t.Fatalf("remedy changed: %+v", got)
			}
		})
	}
}
