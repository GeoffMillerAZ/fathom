#!/usr/bin/env python3
"""Native provider-free smoke: the installer must correct a wrong local Dir."""

import argparse
import json
import pathlib
import shutil
import subprocess

from preseed_modules import preseed


def invoke(oracle, tofu, subject, scratch, mirror):
    return subprocess.run((str(oracle), "-subject", str(subject), "-scratch", str(scratch),
                           "-mirror", str(mirror), "-tofu", str(tofu)),
                          capture_output=True, text=True, check=False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--scanner-bin", type=pathlib.Path, required=True)
    parser.add_argument("--oracle-bin", type=pathlib.Path, required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    base = cache / "native" / "preseed-smoke"
    shutil.rmtree(base, ignore_errors=True)
    subject = base / "subject"
    scratch = base / "scratch"
    child = subject / "modules" / "child"
    child.mkdir(parents=True)
    (subject / "main.tf").write_text('module "child" { source = "./modules/child" }\noutput "value" { value = module.child.value }\n')
    (child / "main.tf").write_text('output "value" { value = "ok" }\n')
    mirror = base / "empty-mirror"
    mirror.mkdir()
    tofu = cache / "tools" / "tofu-1.13.0"

    summary = preseed(cache, args.scanner_bin.resolve(), subject, scratch, tofu, mirror)
    if len(summary) != 2 or summary[0]["modules"] != 1:
        raise RuntimeError("provider-free fixture did not preseed its required child")
    good = invoke(args.oracle_bin.resolve(), tofu, subject, scratch, mirror)
    if good.returncode or not json.loads(good.stdout)["definitions"]:
        raise RuntimeError("complete preseeded native fixture failed: " + good.stderr[-1200:])

    manifest = scratch / "data" / "000" / "modules" / "modules.json"
    data = json.loads(manifest.read_text())
    wrong = base / "wrong"
    wrong.mkdir()
    (wrong / "main.tf").write_text('output "only_wrong" { value = "wrong" }\n')
    for record in data["Modules"]:
        if record["Key"] == "child":
            record["Dir"] = str(wrong)
    manifest.write_text(json.dumps(data) + "\n")
    preseed(cache, args.scanner_bin.resolve(), subject, scratch, tofu, mirror)
    corrected = {record["Key"]: record for record in json.loads(manifest.read_text())["Modules"]}
    if pathlib.Path(corrected["child"]["Dir"]).name != "child":
        raise RuntimeError("preflight retained a wrong local Dir")
    corrected_truth = invoke(args.oracle_bin.resolve(), tofu, subject, scratch, mirror)
    if corrected_truth.returncode or any(definition["name"] == "output.only_wrong"
                                         for definition in json.loads(corrected_truth.stdout)["definitions"]):
        raise RuntimeError("oracle accepted the wrong local module after native install")
    data = json.loads(manifest.read_text())
    data["Modules"] = [record for record in data["Modules"] if record["Key"] != "child"]
    manifest.write_text(json.dumps(data) + "\n")
    bad = invoke(args.oracle_bin.resolve(), tofu, subject, scratch, mirror)
    if bad.returncode == 0 or "root .: init failed" not in bad.stderr:
        raise RuntimeError("init -get=false did not reject missing preseed record: " + bad.stderr[-1200:])
    print(json.dumps({"complete_fixture_passed": True,
                      "wrong_local_dir_corrected_by_native_init": True,
                      "missing_manifest_entry_rejected_by_init": True,
                      "roots": len(summary)}, sort_keys=True))


if __name__ == "__main__":
    main()
