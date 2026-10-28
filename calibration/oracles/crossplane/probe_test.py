import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock


SPEC = importlib.util.spec_from_file_location("crossplane_probe", Path(__file__).with_name("probe.py"))
probe = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe)


class ProbeTest(unittest.TestCase):
    def test_report_requires_identity_and_reconciles(self):
        expected = [("apiextensions.crossplane.io/v1", "Composition", "one")]
        report = {"summary": {"total": 1, "valid": 1, "invalid": 0, "missingSchemas": 0},
                  "resources": [{"apiVersion": expected[0][0], "kind": "Composition", "name": "one", "status": "valid"}]}
        self.assertEqual(probe.report_counts(json.dumps(report).encode(), expected),
                         {"valid": 1, "invalid": 0, "missingSchemas": 0, "extraRows": 0})
        wrong_group = json.loads(json.dumps(report))
        wrong_group["resources"][0]["apiVersion"] = "other.example/v1"
        wrong_summary = json.loads(json.dumps(report))
        wrong_summary["summary"]["total"] = 2
        wrong_status = json.loads(json.dumps(report))
        wrong_status["resources"][0]["status"] = "mystery"
        for bad in ({"resources": []}, wrong_group, wrong_summary, wrong_status):
            with self.assertRaises(probe.Refusal):
                probe.report_counts(json.dumps(bad).encode(), expected)
        with self.assertRaises(probe.Refusal):
            probe.report_counts(json.dumps(report).encode(), expected, exitcode=1)
        invalid = json.loads(json.dumps(report))
        invalid["resources"][0]["status"] = "invalid"
        invalid["summary"].update(valid=0, invalid=1)
        with self.assertRaises(probe.Refusal):
            probe.report_counts(json.dumps(invalid).encode(), expected, exitcode=0)
        self.assertEqual(probe.report_counts(json.dumps(invalid).encode(), expected, exitcode=1)["invalid"], 1)
        input_row = {"apiVersion": "fn.example/v1", "kind": "Input", "name": "", "status": "missingSchema"}
        with_input = json.loads(json.dumps(report))
        with_input["resources"].append(input_row)
        with_input["summary"].update(total=2, missingSchemas=1)
        extras = probe.report_counts(json.dumps(with_input).encode(), expected, exitcode=1)
        self.assertEqual(extras["missingSchemas"], 1)
        self.assertEqual(extras["extraRows"], 1)
        with_input["resources"].append(input_row.copy())
        with_input["summary"].update(total=3, missingSchemas=2)
        self.assertEqual(probe.report_counts(json.dumps(with_input).encode(), expected, exitcode=1)["extraRows"], 2)

    def test_schema_lookup_and_direction(self):
        crd = {"spec": {"group": "provider.example.org", "names": {"kind": "Bucket"},
                        "versions": [{"name": "v1", "served": True, "schema": {"openAPIV3Schema": {
                            "properties": {"spec": {"properties": {"region": {"type": "string"}}}}}}}]}}
        index = probe.SchemaIndex()
        index.add(crd, "opaque")
        self.assertEqual(index.match("provider.example.org/v1", "Bucket")[0], "found")
        self.assertEqual(index.match("provider.example.org/v2", "Bucket")[0], "missing")
        schema = index.match("provider.example.org/v1", "Bucket")[1]
        self.assertEqual(probe.field_verdict(schema, "spec.region"), "declared")
        self.assertEqual(probe.field_verdict(schema, "spec.other"), "notDeclared")
        self.assertEqual(probe.field_verdict(schema, "metadata.name"), "unmodeled")
        self.assertEqual(probe.field_verdict({"properties": {"spec": {"x-kubernetes-preserve-unknown-fields": True}}},
                                             "spec.anything"), "unmodeled")
        index.add(crd, "another")
        self.assertEqual(index.match("provider.example.org/v1", "Bucket")[0], "ambiguous")

    def test_subprocess_exit_and_output_rules(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "tool"
            p.write_text("#!/usr/bin/env python3\nimport sys\nprint(sys.argv[-1])\nsys.exit(1)\n")
            p.chmod(0o755)
            out = probe.call([str(p), "ok"], Path(tmp), allow_semantic_one=True)
            self.assertEqual(out.strip(), b"ok")
            with self.assertRaises(probe.Refusal):
                probe.call([str(p), "ok"], Path(tmp), allow_semantic_one=False)
            p.write_text("#!/usr/bin/env python3\nimport sys\nprint('x'*17)\nsys.exit(2)\n")
            with self.assertRaises(probe.Refusal):
                probe.call([str(p)], Path(tmp), cap=16, allow_semantic_one=True)
            p.write_text("#!/usr/bin/env python3\nimport sys\nprint('{}')\nsys.exit(2)\n")
            with self.assertRaises(probe.Refusal):
                probe.call([str(p)], Path(tmp), allow_semantic_one=True)
            p.write_text("#!/usr/bin/env python3\nimport time\ntime.sleep(2)\n")
            with self.assertRaises(probe.Refusal):
                probe.call([str(p)], Path(tmp), timeout=0.05)
            p.write_text("#!/usr/bin/env python3\nimport os\nwhile True: os.write(1, b'x'*4096)\n")
            with self.assertRaises(probe.Refusal):
                probe.call([str(p)], Path(tmp), cap=4096, timeout=1)

    def test_git_identity_requires_exact_clean_revision(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = (tmp + "\n").encode()
            with mock.patch.object(probe, "call", side_effect=[root, b"abc\n", b""]):
                probe.git_identity(tmp, "abc", tmp)
            with mock.patch.object(probe, "call", side_effect=[b"/other\n"]), self.assertRaises(probe.Refusal):
                probe.git_identity(tmp, "abc", tmp)
            with mock.patch.object(probe, "call", side_effect=[root, b"other\n"]), self.assertRaises(probe.Refusal):
                probe.git_identity(tmp, "abc", tmp)
            with mock.patch.object(probe, "call", side_effect=[root, b"abc\n", b"?? dirty\n"]), self.assertRaises(probe.Refusal):
                probe.git_identity(tmp, "abc", tmp)

    def test_invalid_manifest_invalidates_previous_receipt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            receipt = root / "receipt.json"
            receipt.write_text("old")
            manifest = root / "manifest.json"
            manifest.write_text("not JSON")
            with self.assertRaises(ValueError):
                probe.run(manifest, receipt, root / "scratch", root / "xp", root / "yq")
            self.assertFalse(receipt.exists())

    def test_file_and_byte_bounds_at_plus_one(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "one.yaml").write_text("x")
            (root / "two.yaml").write_text("x")
            with self.assertRaises(probe.Refusal):
                probe.safe_files(root, limit=1)
            with mock.patch.object(probe, "MAX_FILE", 0):
                with self.assertRaises(probe.Refusal):
                    probe.checked_size(root / "one.yaml")

    def test_atomic_receipt_on_missing_report(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subject = root / "subject"
            subject.mkdir()
            (subject / "composition.yaml").write_text("kind: Composition\n")
            crds = root / "core"
            crds.mkdir()
            yq = root / "yq"
            yq.write_text("#!/usr/bin/env python3\nimport json\nprint(json.dumps({'apiVersion':'apiextensions.crossplane.io/v1','kind':'Composition','metadata':{'name':'one'},'spec':{}}))\n")
            yq.chmod(0o755)
            xp = root / "crossplane"
            xp.write_text("#!/usr/bin/env python3\nprint('{}')\n")
            xp.chmod(0o755)
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"subjects": [{"id": "s", "root": str(subject), "sha": "a"*40,
                                                          "scope": ["*.yaml"]}], "provider_roots": [], "core_crds": str(crds),
                                            "tool_sha256": {"crossplane": probe.digest(xp), "yq": probe.digest(yq)}}))
            receipt = root / "receipt.json"
            receipt.write_text("old")
            with mock.patch.object(probe, "git_identity"), self.assertRaises(probe.Refusal):
                probe.run(manifest, receipt, root / "scratch", xp, yq)
            self.assertFalse(receipt.exists())

    def test_provider_control_original_and_unknown_field(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            provider = root / "provider"
            provider.mkdir()
            (provider / "package" / "crds").mkdir(parents=True)
            (provider / "package" / "crds" / "crd.yaml").write_text("fixture\n")
            (provider / "example.yaml").write_text("fixture\n")
            core = root / "core"
            core.mkdir()
            yq = root / "yq"
            yq.write_text("#!/usr/bin/env python3\nimport json,sys\n"
                          "name=sys.argv[-1].split('/')[-1]\n"
                          "crd={'kind':'CustomResourceDefinition','spec':{'group':'provider.example.org','names':{'kind':'Bucket'},'versions':[{'name':'v1','served':True,'schema':{'openAPIV3Schema':{'properties':{'spec':{'properties':{'forProvider':{'properties':{'region':{'type':'string'}}}}}}}}}]}}\n"
                          "example={'apiVersion':'provider.example.org/v1','kind':'Bucket','metadata':{'name':'b'},'spec':{'forProvider':{'region':'west'}}}\n"
                          "if name=='crd.yaml': print(json.dumps(crd))\n"
                          "else:\n"
                          " first=dict(example); first['metadata']={'name':'first'}\n"
                          " print(json.dumps(first)); print(json.dumps(example))\n")
            yq.chmod(0o755)
            xp = root / "crossplane"
            xp.write_text("#!/usr/bin/env python3\nimport json,sys\n"
                          "file=sys.argv[-1]\n"
                          "try: resource=json.load(open(file))\n"
                          "except Exception: resource={'kind':'Bucket','metadata':{'name':'b'}}\n"
                          "bad='iacProbeUnknownField' in resource.get('spec',{}).get('forProvider',{})\n"
                          "print(json.dumps({'summary':{'total':1,'valid':0 if bad else 1,'invalid':1 if bad else 0,'missingSchemas':0},'resources':[{'apiVersion':'provider.example.org/v1','kind':'Bucket','name':'b','status':'invalid' if bad else 'valid'}]}))\n"
                          "sys.exit(1 if bad else 0)\n")
            xp.chmod(0o755)
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"subjects": [], "provider_roots": [
                {"id": "provider", "root": str(provider), "sha": "b" * 40,
                 "crd_sha256": probe.tree_digest(provider / "package" / "crds", [provider / "package" / "crds" / "crd.yaml"])}],
                "core_crds": str(core), "control_resources": [
                    {"provider_id": "provider", "path": "example.yaml", "document_index": 1,
                     "source_sha256": probe.digest(provider / "example.yaml")}],
                "tool_sha256": {"crossplane": probe.digest(xp), "yq": probe.digest(yq)}}))
            result = probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertEqual(result["controls"]["originalValid"], 1)
            self.assertEqual(result["controls"]["mutationInvalid"], 1)
            self.assertEqual(result["controls"]["mutationOther"], 0)
            self.assertEqual(result["controlSourceSha256"], [probe.digest(provider / "example.yaml")])
            self.assertEqual(result["controls"]["nativeCalls"], 2)
            self.assertEqual(len(list((root / "scratch" / "raw").glob("native-*.json"))), 2)
            self.assertEqual(len(result["usedProviderCrdSha256"]), 1)
            original_truth = result["rawTruthSha256"]
            xp.write_text(xp.read_text().replace("bad='iacProbeUnknownField' in resource.get('spec',{}).get('forProvider',{})", "bad=True"))
            changed_manifest = json.loads(manifest.read_text())
            changed_manifest["tool_sha256"]["crossplane"] = probe.digest(xp)
            manifest.write_text(json.dumps(changed_manifest))
            second = probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertEqual(second["controls"]["originalInvalid"], 1)
            self.assertNotEqual(second["rawTruthSha256"], original_truth)
            yq.write_text(yq.read_text() + "\n# changed\n")
            with self.assertRaises(probe.Refusal):
                probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertFalse((root / "receipt.json").exists())

    def test_whole_receipt_bounds_at_plus_one(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            provider = root / "provider"
            crds = provider / "package" / "crds"
            crds.mkdir(parents=True)
            (crds / "bucket.yaml").write_text("crd")
            subject = root / "subject"
            subject.mkdir()
            (subject / "composition.yaml").write_text("composition")
            core = root / "core"
            core.mkdir()
            xp, yq = root / "crossplane", root / "yq"
            xp.write_text("tool")
            yq.write_text("tool")
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"subjects": [{"id": "s", "root": str(subject), "sha": "a",
                                                          "scope": ["*.yaml"]}],
                                            "provider_roots": [{"id": "p", "root": str(provider), "sha": "b",
                                                                "crd_sha256": probe.tree_digest(crds, [crds / "bucket.yaml"])}],
                                            "core_crds": str(core), "tool_sha256": {
                                                "crossplane": probe.digest(xp), "yq": probe.digest(yq)}}))
            crd = {"kind": "CustomResourceDefinition", "spec": {"group": "provider.example.org",
                   "names": {"kind": "Bucket"}, "versions": [{"name": "v1", "served": True,
                   "schema": {"openAPIV3Schema": {"properties": {"spec": {"properties": {}}}}}}]}}
            composition = {"apiVersion": "apiextensions.crossplane.io/v1", "kind": "Composition",
                           "metadata": {"name": "c"}, "spec": {"resources": [{"base": {
                               "apiVersion": "provider.example.org/v1", "kind": "Bucket",
                               "metadata": {"name": "b"}}, "patches": [{"type": "ToCompositeFieldPath",
                                                                 "fromFieldPath": "spec.region"}]}]}}

            def fake_docs(_yq, path, _scratch):
                return [crd if Path(path).name == "bucket.yaml" else composition]

            def fake_call(argv, _scratch, **_kwargs):
                file = Path(argv[-1])
                if file.suffix == ".json":
                    resource = json.loads(file.read_text())
                    api, kind, name = "provider.example.org/v1", "Bucket", resource.get("metadata", {}).get("name", "")
                else:
                    api, kind, name = "apiextensions.crossplane.io/v1", "Composition", "c"
                report = {"summary": {"total": 1, "valid": 1, "invalid": 0, "missingSchemas": 0},
                          "resources": [{"apiVersion": api, "kind": kind, "name": name, "status": "valid"}]}
                return json.dumps(report).encode(), 0

            for limit_name, value in (("MAX_PROVIDER", 0), ("MAX_FILES", 0), ("MAX_TOTAL", 0),
                                      ("MAX_BASES", 0), ("MAX_PATCHES", 0), ("MAX_CALLS", 1)):
                with self.subTest(limit=limit_name), mock.patch.object(probe, limit_name, value), \
                     mock.patch.object(probe, "docs", side_effect=fake_docs), \
                     mock.patch.object(probe, "call", side_effect=fake_call), \
                     mock.patch.object(probe, "git_identity"):
                    with self.assertRaises(probe.Refusal):
                        probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
                    self.assertFalse((root / "receipt.json").exists())
            composition["spec"]["patchSets"] = [{"name": "unused", "patches": []}]
            unnamed = json.loads(json.dumps(composition))
            del unnamed["spec"]["resources"][0]["base"]["metadata"]

            def unnamed_docs(_yq, path, _scratch):
                return [crd if Path(path).name == "bucket.yaml" else unnamed]

            with mock.patch.object(probe, "docs", side_effect=unnamed_docs), \
                 mock.patch.object(probe, "call", side_effect=fake_call), \
                 mock.patch.object(probe, "git_identity"):
                result = probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertEqual(result["exclusions"]["baseWithoutName"], 1)
            self.assertEqual(result["exclusions"]["patchSetDefinitions"], 1)
            self.assertEqual(result["exclusions"]["unsupportedPatches"], 0)
            self.assertEqual(result["aggregate"]["nativeCalls"], 2)
            self.assertEqual(result["aggregate"]["native"]["valid"], 2)
            self.assertEqual(result["subjects"][0]["counts"]["nativeProviderBases"],
                             {"valid": 1, "invalid": 0, "missingSchemas": 0, "extraRows": 0})
            self.assertEqual(result["aggregate"]["nativeProviderBases"]["valid"], 1)
            self.assertEqual(result["aggregate"]["fieldPaths"]["notDeclared"], 1)

            def mutating_docs(_yq, path, _scratch):
                if Path(path).name == "composition.yaml":
                    (crds / "bucket.yaml").write_text("changed")
                    return [composition]
                return [crd]

            with mock.patch.object(probe, "docs", side_effect=mutating_docs), \
                 mock.patch.object(probe, "call", side_effect=fake_call), \
                 mock.patch.object(probe, "git_identity"), self.assertRaises(probe.Refusal):
                probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertFalse((root / "receipt.json").exists())

    def test_mixed_file_xrd_does_not_supply_composition_schema(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subject = root / "subject"
            subject.mkdir()
            (subject / "mixed.yaml").write_text("mixed")
            (subject / "composition.yaml").write_text("composition")
            core = root / "core"
            core.mkdir()
            xp, yq = root / "crossplane", root / "yq"
            xp.write_text("tool")
            yq.write_text("tool")
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"subjects": [{"id": "s", "root": str(subject),
                "sha": "a", "scope": ["*.yaml"]}], "provider_roots": [], "core_crds": str(core),
                "tool_sha256": {"crossplane": probe.digest(xp), "yq": probe.digest(yq)}}))
            xrd = {"apiVersion": "apiextensions.crossplane.io/v1", "kind": "CompositeResourceDefinition",
                   "metadata": {"name": "widgets.example.org"}, "spec": {"group": "example.org",
                   "names": {"kind": "Widget"}, "versions": [{"name": "v1", "schema": {
                       "openAPIV3Schema": {"properties": {"spec": {"properties": {
                           "region": {"type": "string"}}}}}}}]}}
            composition = {"apiVersion": "apiextensions.crossplane.io/v1", "kind": "Composition",
                           "metadata": {"name": "c"}, "spec": {"compositeTypeRef": {
                               "apiVersion": "example.org/v1", "kind": "Widget"}, "resources": [{
                                   "base": {"apiVersion": "provider.example.org/v1", "kind": "Bucket",
                                            "metadata": {"name": "b"}}, "patches": [{
                                       "type": "FromCompositeFieldPath", "fromFieldPath": "spec.region"}]}]}}

            def fake_docs(_yq, path, _scratch):
                return [xrd, {"apiVersion": "v1", "kind": "ConfigMap"}] if Path(path).name == "mixed.yaml" else [composition]

            def fake_call(_argv, _scratch, **_kwargs):
                report = {"summary": {"total": 1, "valid": 1, "invalid": 0, "missingSchemas": 0},
                          "resources": [{"apiVersion": "apiextensions.crossplane.io/v1",
                                         "kind": "Composition", "name": "c", "status": "valid"}]}
                return json.dumps(report).encode(), 0

            with mock.patch.object(probe, "docs", side_effect=fake_docs), \
                 mock.patch.object(probe, "call", side_effect=fake_call), \
                 mock.patch.object(probe, "git_identity"):
                result = probe.run(manifest, root / "receipt.json", root / "scratch", xp, yq)
            self.assertEqual(result["exclusions"]["mixedFiles"], 1)
            self.assertEqual(result["aggregate"]["xrds"], 0)
            self.assertEqual(result["aggregate"]["nativeCalls"], 1)
            self.assertEqual(result["aggregate"]["fieldPaths"]["unmodeled"], 1)


if __name__ == "__main__":
    unittest.main()
