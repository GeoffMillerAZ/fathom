"""Refusal and bounded-command checks for the private owner-tool probe."""

import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock


HERE = Path(__file__).resolve().parent
if str(HERE) not in sys.path:
    sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location("crossplane_owner_probes", HERE / "owner_probes.py")
owner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(owner)
TMP = HERE.parents[2] / "workspace/tmp"


def setUpModule():
    TMP.mkdir(parents=True, exist_ok=True)


class OwnerProbeTest(unittest.TestCase):
    @mock.patch.object(owner.time, "sleep")
    @mock.patch.object(owner.os, "getloadavg", side_effect=[(10.5, 0, 0), (8.5, 0, 0)])
    @mock.patch.object(owner.os, "getpriority", return_value=10)
    def test_launch_waits_and_records_admitted_load(self, priority, load, sleep):
        self.assertEqual(owner.launch_load(), 8.5)
        self.assertEqual(priority.call_count, 1)
        self.assertEqual(load.call_count, 2)
        sleep.assert_called_once_with(20)

    @mock.patch.object(owner.os, "getloadavg")
    @mock.patch.object(owner.os, "getpriority", return_value=0)
    def test_launch_refuses_without_nice(self, _priority, load):
        with self.assertRaisesRegex(owner.probe.Refusal, "nice 10"):
            owner.launch_load()
        load.assert_not_called()

    @mock.patch.object(owner, "launch_load", return_value=1.0)
    def test_timeout_kills_whole_command(self, _load):
        with tempfile.TemporaryDirectory(dir=TMP) as directory:
            scratch = Path(directory)
            result = owner.run_tool([sys.executable, "-c", "import time; time.sleep(10)"],
                                    "timeout", scratch, timeout=0.05)
            self.assertEqual(result["status"], "timeout")
            self.assertIsNotNone(result["exit"])
            self.assertLess(result["seconds"], 2)
            self.assertEqual(result["stdout_bytes"], 0)

    @mock.patch.object(owner, "launch_load", return_value=1.0)
    def test_output_cap_classifies_and_truncates_private_bytes(self, _load):
        with tempfile.TemporaryDirectory(dir=TMP) as directory:
            scratch = Path(directory)
            result = owner.run_tool([sys.executable, "-c",
                                     "import sys; sys.stdout.buffer.write(b'x' * 8192); sys.stdout.flush()"],
                                    "output-cap", scratch, cap=128)
            self.assertEqual(result["status"], "output-cap")
            self.assertEqual(result["stdout_bytes"], 128)
            self.assertEqual(result["stderr_bytes"], 0)
            self.assertEqual((scratch / "raw/output-cap.out").stat().st_size, 128)
            self.assertEqual(result["stdout_sha256"], owner.sha(b"x" * 128))

    def test_pin_refusal_precedes_owner_commands(self):
        with tempfile.TemporaryDirectory(dir=TMP) as directory:
            scratch = Path(directory)
            tool = scratch / "tool"
            tool.write_bytes(b"local fake")
            manifest = scratch / "manifest.json"
            manifest.write_text(json.dumps({"tool_sha256": {"crossplane": "wrong"},
                                            "subjects": []}))
            out = scratch / "receipt.json"
            argv = ["owner_probes.py", "--manifest", str(manifest), "--crossplane", str(tool),
                    "--out", str(out), "--scratch", str(scratch / "raw-work")]
            with mock.patch.object(sys, "argv", argv), \
                 mock.patch.object(owner.os, "getpriority", return_value=10), \
                 mock.patch.object(owner, "run_tool") as commands:
                with self.assertRaisesRegex(owner.probe.Refusal, "digest differs"):
                    owner.main()
                commands.assert_not_called()
            self.assertFalse(out.exists())
        with self.assertRaisesRegex(owner.probe.Refusal, "nine pinned subjects"):
            owner.check_subjects({"subjects": []}, TMP)

    def test_nine_self_declared_subject_pins_are_refused_before_checkout(self):
        frozen = json.loads((HERE / "pins-r1.json").read_text())["subjects"]
        original = [{"id": pin.get("id", pin["repo"] + "-legacy" if pin.get("prior_probe")
                           else pin["repo"]), "sha": pin["sha"],
                     "scope": pin.get("scope", ["*.yaml", "*.yml"]), "root": "/unused"}
                    for pin in frozen]
        for field, wrong in (("id", "self-declared/id"), ("sha", "0" * 40),
                             ("scope", ["accepted-only.yaml"])):
            with self.subTest(field=field), mock.patch.object(owner.probe, "git_identity") as git_identity:
                rows = json.loads(json.dumps(original))
                rows[0][field] = wrong
                with self.assertRaisesRegex(owner.probe.Refusal, "frozen pins"):
                    owner.check_subjects({"subjects": rows}, TMP)
                git_identity.assert_not_called()

    @mock.patch.object(owner, "launch_load", return_value=1.0)
    def test_lsp_initialize_error_opens_zero_files(self, _load):
        with tempfile.TemporaryDirectory(dir=TMP) as directory:
            scratch = Path(directory)
            root = scratch / "subject"
            root.mkdir()
            file = root / "composition.yaml"
            file.write_text("kind: Composition\n")
            server = scratch / "lsp-error"
            server.write_text(
                f"#!{sys.executable}\n"
                "import json, os\n"
                "os.read(0, 4096)\n"
                "payload = json.dumps({'jsonrpc':'2.0','id':1,'error':{'code':-32603,'message':'bad'}}).encode()\n"
                "os.write(1, b'Content-Length: ' + str(len(payload)).encode() + b'\\r\\n\\r\\n' + payload)\n")
            server.chmod(0o755)
            result = owner.lsp_subject(str(server), {"id": "fixed/subject"}, root, [file], scratch)
            self.assertEqual(result["status"], "initialize-error")
            self.assertEqual(result["selected_files"], 1)
            self.assertEqual(result["opened_files"], 0)
            self.assertFalse(result["initialized"])

    @mock.patch.object(owner, "launch_load", return_value=1.0)
    def test_lsp_null_diagnostics_are_empty(self, _load):
        with tempfile.TemporaryDirectory(dir=TMP) as directory:
            scratch = Path(directory)
            root = scratch / "subject"
            root.mkdir()
            file = root / "composition.yaml"
            file.write_text("kind: Composition\n")
            server = scratch / "lsp-null-diagnostics"
            server.write_text(
                f"#!{sys.executable}\n"
                "import json, os\n"
                "os.read(0, 4096)\n"
                "def send(message):\n"
                "    payload = json.dumps(message).encode()\n"
                "    os.write(1, b'Content-Length: ' + str(len(payload)).encode() + b'\\r\\n\\r\\n' + payload)\n"
                "send({'jsonrpc':'2.0','id':1,'error':{'code':-32603,'message':'bad'}})\n"
                "send({'jsonrpc':'2.0','method':'textDocument/publishDiagnostics',"
                "'params':{'uri':'file:///composition.yaml','diagnostics':None}})\n")
            server.chmod(0o755)
            result = owner.lsp_subject(str(server), {"id": "fixed/subject"}, root, [file], scratch)
            self.assertEqual(result["status"], "initialize-error")
            self.assertEqual(result["opened_files"], 0)
            self.assertEqual(result["diagnostic_notifications"], 1)
            self.assertEqual(result["diagnostic_entries"], 0)


if __name__ == "__main__":
    unittest.main()
