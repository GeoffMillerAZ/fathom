"""python -m unittest test_terragrunt_attest (TERRAGRUNT=<terragrunt 1.1.6>; skipped without it)."""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
TG = os.environ.get("TERRAGRUNT") or shutil.which("terragrunt")

UNIT = """locals {
  used   = "example-org"
  unused = "never read"
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "./module"
}

inputs = {
  name = local.used
}
"""

ROOT = """locals {
  region      = "us-east-1"
  unused_root = "nothing reads it"
}

inputs = {
  region = local.region
}
"""


@unittest.skipUnless(TG, "terragrunt not installed")
class AttestTest(unittest.TestCase):
    def test_tool_evidence_and_ablation(self):
        # A unit's own locals are in its render (found_in_file), used or not;
        # an included file's locals are evaluated away, so only ablation can
        # evidence one: root.hcl's unused_root is left out.
        with tempfile.TemporaryDirectory() as d:
            subj = os.path.join(d, "subject")
            os.makedirs(os.path.join(subj, "app", "module"))
            open(os.path.join(subj, "root.hcl"), "w").write(ROOT)
            open(os.path.join(subj, "app", "terragrunt.hcl"), "w").write(UNIT)
            open(os.path.join(subj, "app", "module", "main.tf"), "w").write('variable "name" {}\n')
            files = os.path.join(d, "files.txt")
            open(files, "w").write("app/terragrunt.hcl\nroot.hcl\n")
            out = os.path.join(d, "out.json")
            r = subprocess.run([sys.executable, os.path.join(HERE, "terragrunt_attest.py"), "-subject", subj,
                                "-scratch", os.path.join(d, "scratch"), "-terragrunt", TG, "-files", files, "-out", out],
                               capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            got = json.load(open(out))
            defs = sorted((x["path"], x["name"], x["kind"]) for x in got["definitions"])
            self.assertEqual(defs, [
                ("app/terragrunt.hcl", "include.root", "import"),
                ("app/terragrunt.hcl", "input.name", "key"),
                ("app/terragrunt.hcl", "local.unused", "variable"),
                ("app/terragrunt.hcl", "local.used", "variable"),
                ("app/terragrunt.hcl", "terraform", "module"),
                ("app/terragrunt.hcl", "terragrunt.hcl", "package"),
                ("root.hcl", "input.region", "key"),
                ("root.hcl", "local.region", "variable"),
                ("root.hcl", "root.hcl", "package"),
            ])
            refs = sorted((x["path"], x["from"], x["to"]) for x in got["references"])
            self.assertEqual(refs, [
                ("app/terragrunt.hcl", "input.name", "local.used"),
                ("root.hcl", "input.region", "local.region"),
            ])
            self.assertEqual(got["tier"], "A")


if __name__ == "__main__":
    unittest.main()
