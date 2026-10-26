"""Independent AST oracle controls. Run from the repository root with -pythonpath."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

args_parser = argparse.ArgumentParser()
args_parser.add_argument("-pythonpath", required=True)
args = args_parser.parse_args()


class CallNames(unittest.TestCase):
    def test_namespaces_and_receiver_shapes(self):
        scratch = Path("workspace/tmp")
        scratch.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=scratch) as directory:
            root = Path(directory)
            (root / "calls.j2").write_text("""{{ direct() }}
{{ forms.input() }} {{ other.input() }} {{ forms.group.input() }}
{{ factory().input() }} {{ values[0].input() }} {{ forms.range() }}
{{ range(3) }}
""")
            result = subprocess.run([
                sys.executable, "-I", str(Path(__file__).with_name("jinja2_meta_dump.py")),
                "-subject", str(root), "-pythonpath", args.pythonpath,
            ], check=True, capture_output=True, text=True)
            calls = json.loads(result.stdout)["calls"]
            self.assertEqual({call["to"] for call in calls}, {
                "direct", "forms.input", "other.input", "forms.group.input",
                "factory", "factory().input", "values[].input", "forms.range",
            })
            self.assertTrue(all(call["from"] == "<module>" for call in calls))

    def test_dynamic_direct_callees_fail_closed(self):
        scratch = Path("workspace/tmp")
        scratch.mkdir(parents=True, exist_ok=True)
        for source in ("{{ handlers[0]() }}", "{{ factory()() }}"):
            with self.subTest(source=source), tempfile.TemporaryDirectory(dir=scratch) as directory:
                root = Path(directory)
                (root / "dynamic.j2").write_text(source)
                result = subprocess.run([
                    sys.executable, "-I", str(Path(__file__).with_name("jinja2_meta_dump.py")),
                    "-subject", str(root), "-pythonpath", args.pythonpath,
                ], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("unsupported Jinja direct call callee", result.stderr)
                self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main(argv=[sys.argv[0]])
