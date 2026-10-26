import importlib.util
import pathlib
import json
import subprocess
import tempfile
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "preseed_modules", pathlib.Path(__file__).with_name("preseed_modules.py"))
preseed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preseed)


class PreseedTests(unittest.TestCase):
    def test_scratch_tf_path_set_and_bytes_are_checked(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp)
            subject, scratch = base / "subject", base / "scratch"
            subject.mkdir()
            scratch.mkdir()
            (subject / "main.tf").write_text('output "a" { value = 1 }\n')
            (scratch / "main.tf").write_text('output "a" { value = 2 }\n')
            self.assertNotEqual(preseed.tf_contents(subject), preseed.tf_contents(scratch))
            (scratch / "main.tf").write_bytes((subject / "main.tf").read_bytes())
            self.assertEqual(preseed.tf_contents(subject), preseed.tf_contents(scratch))
            (scratch / "extra.tf").write_text('output "b" { value = 2 }\n')
            self.assertNotEqual(preseed.tf_contents(subject), preseed.tf_contents(scratch))

    def test_preflight_discards_wrong_cached_local_before_native_init(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject, scratch = base / "subject", base / "scratch"
            child = subject / "modules" / "child"
            child.mkdir(parents=True)
            (subject / "main.tf").write_text('module "child" { source = "./modules/child" }\n')
            (child / "main.tf").write_text('output "value" { value = "a" }\n')
            path = scratch / "data" / "000" / "modules" / "modules.json"
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps({"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                                    {"Key": "child", "Source": "./modules/child",
                                                     "Dir": str(base / "wrong")}]}) + "\n")
            calls = {subject: [{"name": "child", "source": "./modules/child"}], child: []}
            def native_init(*args, **kwargs):
                seeded = json.loads(path.read_text())
                self.assertEqual([record["Key"] for record in seeded["Modules"]], [""])
                path.write_text(json.dumps({"Modules": [seeded["Modules"][0],
                    {"Key": "child", "Source": "./modules/child", "Dir": "modules/child"}]}) + "\n")
                return subprocess.CompletedProcess(args, 0, "", "")
            with mock.patch.object(preseed, "roots", return_value=["."]), \
                 mock.patch.object(preseed, "scan", side_effect=lambda _, directory: calls[directory]), \
                 mock.patch.object(preseed.subprocess, "run", side_effect=native_init):
                result = preseed.preseed(base, pathlib.Path("scanner"), subject, scratch,
                                         pathlib.Path("tofu"), base / "mirror")
            self.assertEqual(result[0]["native_local_agreements"], 1)
            self.assertEqual(json.loads(path.read_text())["Modules"][1]["Dir"], "modules/child")

    def test_partial_native_local_survives_registry_restore_and_second_init(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject, scratch = base / "subject", base / "scratch"
            child, remote = subject / "modules" / "child", base / "remote"
            child.mkdir(parents=True)
            remote.mkdir()
            (subject / "main.tf").write_text('module "child" { source = "./modules/child" }\n')
            (child / "main.tf").write_text('module "kms" { source = "terraform-aws-modules/kms/aws" version = "4.0.0" }\n')
            path = scratch / "data" / "000" / "modules" / "modules.json"
            calls = {subject: [{"name": "child", "source": "./modules/child"}],
                     child: [{"name": "kms", "source": "terraform-aws-modules/kms/aws", "version": "4.0.0"}],
                     remote: []}
            attempts = []
            def native_init(*args, **kwargs):
                records = json.loads(path.read_text())["Modules"]
                attempts.append([record["Key"] for record in records])
                if len(attempts) == 1:
                    self.assertEqual(attempts[-1], ["", "child.kms"])
                    path.write_text(json.dumps({"Modules": [records[0],
                        {"Key": "child", "Source": "./modules/child", "Dir": "modules/child"}]}) + "\n")
                    return subprocess.CompletedProcess(args, 1, "", "registry unavailable")
                self.assertEqual(attempts[-1], ["", "child", "child.kms"])
                self.assertEqual(records[2]["Source"], "registry.opentofu.org/terraform-aws-modules/kms/aws")
                return subprocess.CompletedProcess(args, 0, "", "")
            with mock.patch.object(preseed, "roots", return_value=["."]), \
                 mock.patch.object(preseed, "scan", side_effect=lambda _, directory: calls[directory]), \
                 mock.patch.object(preseed, "pinned_module", return_value=(remote, "4.0.0")), \
                 mock.patch.object(preseed.subprocess, "run", side_effect=native_init):
                result = preseed.preseed(base, pathlib.Path("scanner"), subject, scratch,
                                         pathlib.Path("tofu"), base / "mirror")
            self.assertEqual(len(attempts), 2)
            self.assertEqual(result[0]["native_local_agreements"], 1)

    def test_partial_init_without_new_local_progress_refuses(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject, scratch = base / "subject", base / "scratch"
            subject.mkdir()
            remote = base / "remote"
            remote.mkdir()
            (subject / "main.tf").write_text('module "kms" { source = "terraform-aws-modules/kms/aws" version = "4.0.0" }\n')
            path = scratch / "data" / "000" / "modules" / "modules.json"
            def native_init(*args, **kwargs):
                root = json.loads(path.read_text())["Modules"][0]
                path.write_text(json.dumps({"Modules": [root]}) + "\n")
                return subprocess.CompletedProcess(args, 1, "", "registry unavailable")
            with mock.patch.object(preseed, "roots", return_value=["."]), \
                 mock.patch.object(preseed, "scan", side_effect=lambda _, directory:
                                   [{"name": "kms", "source": "terraform-aws-modules/kms/aws", "version": "4.0.0"}]
                                   if directory == subject else []), \
                 mock.patch.object(preseed, "pinned_module", return_value=(remote, "4.0.0")), \
                 mock.patch.object(preseed.subprocess, "run", side_effect=native_init):
                with self.assertRaisesRegex(RuntimeError, "attempt=2, native_locals=0, prior_locals=0, restored_registry=1"):
                    preseed.preseed(base, pathlib.Path("scanner"), subject, scratch,
                                    pathlib.Path("tofu"), base / "mirror")

    def test_repeated_link_restoration_refuses_after_seventeen_attempts(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject, scratch, remote = base / "subject", base / "scratch", base / "remote"
            subject.mkdir()
            remote.mkdir()
            (subject / "main.tf").write_text('module "kms" { source = "terraform-aws-modules/kms/aws" version = "4.0.0" }\n')
            path = scratch / "data" / "000" / "modules" / "modules.json"
            attempts = []
            def failing_init(*args, **kwargs):
                attempts.append(1)
                link = path.parent / "kms"
                self.assertTrue(link.is_symlink())
                link.unlink()
                root = json.loads(path.read_text())["Modules"][0]
                path.write_text(json.dumps({"Modules": [root]}) + "\n")
                return subprocess.CompletedProcess(args, 1, "", "registry unavailable")
            with mock.patch.object(preseed, "roots", return_value=["."]), \
                 mock.patch.object(preseed, "scan", side_effect=lambda _, directory:
                                   [{"name": "kms", "source": "terraform-aws-modules/kms/aws", "version": "4.0.0"}]
                                   if directory == subject else []), \
                 mock.patch.object(preseed, "pinned_module", return_value=(remote, "4.0.0")), \
                 mock.patch.object(preseed.subprocess, "run", side_effect=failing_init):
                with self.assertRaisesRegex(RuntimeError, "exceeded depth bound"):
                    preseed.preseed(base, pathlib.Path("scanner"), subject, scratch,
                                    pathlib.Path("tofu"), base / "mirror")
            self.assertEqual(len(attempts), 17)

    def test_registry_link_diagnostic_counts_missing_paths(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp)
            proposal = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                    {"Key": "kms", "Source": "terraform-aws-modules/kms/aws",
                                     "Dir": str(base / "kms"), "Version": "4.0.0"}]}
            self.assertEqual(preseed.registry_link_counts(proposal), (0, 1, 0))

    def test_missing_registry_link_restored_from_exact_pin(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            modules, target = base / "modules", base / "pinned"
            modules.mkdir()
            target.mkdir()
            link = modules / "kms"
            proposal = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                    {"Key": "local", "Source": "./local", "Dir": str(base / "local")},
                                    {"Key": "kms", "Source": "terraform-aws-modules/kms/aws",
                                     "Dir": str(link), "Version": "4.0.0"}]}
            with mock.patch.object(preseed, "pinned_module", return_value=(target, "4.0.0")):
                self.assertEqual(preseed.pinned_registry_links(base, proposal, modules, restore=True), 1)
                self.assertEqual(preseed.pinned_registry_links(base, proposal, modules, restore=False), 0)
            self.assertTrue(link.is_symlink())
            self.assertEqual(link.resolve(), target)
            self.assertFalse((base / "local").exists())

    def test_wrong_existing_registry_link_refuses(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            modules, target, wrong = base / "modules", base / "pinned", base / "wrong"
            modules.mkdir()
            target.mkdir()
            wrong.mkdir()
            link = modules / "kms"
            link.symlink_to(wrong, target_is_directory=True)
            proposal = {"Modules": [{"Key": "kms", "Source": "terraform-aws-modules/kms/aws",
                                     "Dir": str(link), "Version": "4.0.0"}]}
            with mock.patch.object(preseed, "pinned_module", return_value=(target, "4.0.0")):
                with self.assertRaisesRegex(ValueError, "link differs from exact pin"):
                    preseed.pinned_registry_links(base, proposal, modules, restore=True)
            self.assertEqual(link.resolve(), wrong)

    def test_native_manifest_requires_full_population_and_lexical_local_agreement(self):
        with tempfile.TemporaryDirectory() as temp:
            working = pathlib.Path(temp) / "src"
            proposal = {"Modules": [
                {"Key": "", "Source": "", "Dir": "."},
                {"Key": "child", "Source": "./modules/child", "Dir": str(working / "modules/child")},
                {"Key": "remote", "Source": "terraform-aws-modules/kms/aws", "Dir": str(working / "cache/remote"), "Version": "4.0.0"}]}
            native = {"Modules": [dict(record) for record in proposal["Modules"]]}
            native["Modules"][1]["Dir"] = "modules/child"
            self.assertEqual(preseed.check_native_manifest(proposal, native, working), 1)
            native["Modules"][1]["Dir"] = "modules/wrong"
            with self.assertRaisesRegex(ValueError, "destination differs"):
                preseed.check_native_manifest(proposal, native, working)
            native["Modules"] = [record for record in native["Modules"] if record["Key"] != "child"]
            with self.assertRaisesRegex(ValueError, "population differs"):
                preseed.check_native_manifest(proposal, native, working)

    def test_default_registry_host_canonicalization_is_narrow(self):
        with tempfile.TemporaryDirectory() as temp:
            working = pathlib.Path(temp)
            proposal = {"Modules": [
                {"Key": "", "Source": "", "Dir": "."},
                {"Key": "kms", "Source": "terraform-aws-modules/kms/aws",
                 "Version": "4.0.0", "Dir": str(working / "kms")}]}
            native = {"Modules": [dict(record) for record in proposal["Modules"]]}
            native["Modules"][1]["Source"] = "registry.opentofu.org/terraform-aws-modules/kms/aws"
            self.assertEqual(preseed.check_native_manifest(proposal, native, working), 0)
            native["Modules"][1]["Source"] = "registry.terraform.io/terraform-aws-modules/kms/aws"
            with self.assertRaisesRegex(ValueError, "source/version differs"):
                preseed.check_native_manifest(proposal, native, working)
            native["Modules"][1]["Source"] = "registry.opentofu.org/terraform-aws-modules/kms/aws"
            native["Modules"][1]["Version"] = "4.0.1"
            with self.assertRaisesRegex(ValueError, "source/version differs"):
                preseed.check_native_manifest(proposal, native, working)
            self.assertEqual(preseed.canonical_registry_source("./modules/kms"), "./modules/kms")

    def test_local_source_trailing_slash_normalizes_without_loosening_destination(self):
        with tempfile.TemporaryDirectory() as temp:
            working = pathlib.Path(temp).resolve()
            proposal = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                    {"Key": "child", "Source": "../../", "Dir": str(working / "child")}]}
            native = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                  {"Key": "child", "Source": "../..", "Dir": str(working / "child")}]}
            self.assertEqual(preseed.check_native_manifest(proposal, native, working), 1)
            native["Modules"][1]["Dir"] = str(working / "wrong")
            with self.assertRaisesRegex(ValueError, "destination differs"):
                preseed.check_native_manifest(proposal, native, working)
            native["Modules"][1]["Dir"] = str(working / "child")
            native["Modules"][1]["Source"] = "../other"
            with self.assertRaisesRegex(ValueError, "source/version differs"):
                preseed.check_native_manifest(proposal, native, working)

    def test_registry_child_physical_alias_requires_same_target(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            physical = base / "pinned"
            child = physical / "modules" / "child"
            wrong = physical / "modules" / "wrong"
            child.mkdir(parents=True)
            wrong.mkdir()
            link = base / "registry"
            link.symlink_to(physical, target_is_directory=True)
            proposal = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                    {"Key": "registry", "Source": "terraform-aws-modules/eks/aws",
                                     "Version": "21.29.0", "Dir": str(link)},
                                    {"Key": "registry.child", "Source": "./modules/child",
                                     "Dir": str(link / "modules" / "child")}]}
            native = {"Modules": [dict(record) for record in proposal["Modules"]]}
            native["Modules"][2]["Dir"] = str(child)
            self.assertEqual(preseed.check_native_manifest(proposal, native, base), 1)
            self.assertEqual(preseed.count_registry_aliases(proposal, native, base), 1)
            native["Modules"][2]["Dir"] = str(wrong)
            with self.assertRaisesRegex(ValueError, "destination differs"):
                preseed.check_native_manifest(proposal, native, base)

    def test_subject_local_physical_alias_still_refuses(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            physical = base / "physical"
            physical.mkdir()
            link = base / "local"
            link.symlink_to(physical, target_is_directory=True)
            proposal = {"Modules": [{"Key": "", "Source": "", "Dir": "."},
                                    {"Key": "child", "Source": "./local", "Dir": str(link)}]}
            native = {"Modules": [dict(record) for record in proposal["Modules"]]}
            native["Modules"][1]["Dir"] = str(physical)
            with self.assertRaisesRegex(ValueError, "destination differs"):
                preseed.check_native_manifest(proposal, native, base)

    def test_registry_subdirectory_source_is_preserved(self):
        match = preseed.REGISTRY.fullmatch(
            "terraform-aws-modules/cloudwatch/aws//modules/log-group")
        self.assertIsNotNone(match)
        self.assertEqual(match.groups(), ("cloudwatch", "modules/log-group"))

    def test_version_constraints_select_exact_or_compatible_pin(self):
        self.assertTrue(preseed.satisfies("4.0.0", "4.0.0"))
        self.assertFalse(preseed.satisfies("4.2.2", "4.0.0"))
        self.assertTrue(preseed.satisfies("4.2.2", "~> 4.0"))
        self.assertFalse(preseed.satisfies("5.0.0", "~> 4.0"))
        self.assertTrue(preseed.satisfies("1.12.1", "~> 1.6"))
        self.assertFalse(preseed.satisfies("2.0.0", "~> 1.6"))
        self.assertTrue(preseed.satisfies("6.67.0", ">= 6.59"))

    def test_nested_manifest_keeps_original_sources_and_exact_versions(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject = base / "subject"
            scratch = base / "scratch"
            child = subject / "modules" / "local"
            remote = base / "git-modules" / "kms"
            child.mkdir(parents=True)
            remote.mkdir(parents=True)
            calls = {
                subject: [{"name": "local", "source": "./modules/local"},
                          {"name": "kms", "source": "terraform-aws-modules/kms/aws", "version": "4.0.0"}],
                child: [{"name": "nested", "source": "terraform-aws-modules/kms/aws", "version": "~> 4.0"}],
                remote: [],
            }
            def pinned(_, source, constraint):
                self.assertEqual(source, "terraform-aws-modules/kms/aws")
                return remote, "4.0.0" if constraint == "4.0.0" else "4.2.2"
            with mock.patch.object(preseed, "scan", side_effect=lambda _, directory: calls[directory]):
                with mock.patch.object(preseed, "pinned_module", side_effect=pinned):
                    result = preseed.module_manifest(base, pathlib.Path("scanner"), subject, scratch, ".", 0)
            records = {entry["Key"]: entry for entry in result["Modules"]}
            self.assertEqual(set(records), {"", "local", "local.nested", "kms"})
            self.assertEqual(records["local"]["Source"], "./modules/local")
            self.assertEqual(records["local"]["Dir"], str(scratch / "src" / "modules" / "local"))
            self.assertEqual(records["kms"]["Version"], "4.0.0")
            self.assertEqual(records["local.nested"]["Version"], "4.2.2")
            self.assertEqual(pathlib.Path(records["kms"]["Dir"]).resolve(), remote)

    def test_module_instance_cap_accepts_511_and_rejects_512(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject = base / "subject"
            child = subject / "child"
            child.mkdir(parents=True)

            def scan_calls(_, directory):
                if directory == subject:
                    return [{"name": f"child_{i}", "source": "./child"}
                            for i in range(instance_count)]
                return []

            with mock.patch.object(preseed, "scan", side_effect=scan_calls):
                instance_count = 511
                accepted = preseed.module_manifest(base, pathlib.Path("scanner"), subject,
                                                   base / "scratch", ".", 0)
                self.assertEqual(len(accepted["Modules"]), 512)  # root + 511 calls
                instance_count = 512
                with self.assertRaisesRegex(ValueError, "module fan-out/depth cap exceeded"):
                    preseed.module_manifest(base, pathlib.Path("scanner"), subject,
                                            base / "scratch", ".", 0)

    def test_module_depth_cap_accepts_15_and_rejects_16(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            subject = base / "subject"
            subject.mkdir()

            def build_chain(length):
                calls = {}
                current = subject
                for i in range(length):
                    child = current / f"child_{i}"
                    child.mkdir(exist_ok=True)
                    calls[current] = [{"name": f"child_{i}", "source": f"./child_{i}"}]
                    current = child
                calls[current] = []
                return calls

            calls = build_chain(15)
            with mock.patch.object(preseed, "scan", side_effect=lambda _, directory: calls[directory]):
                accepted = preseed.module_manifest(base, pathlib.Path("scanner"), subject,
                                                   base / "scratch", ".", 0)
            self.assertEqual(len(accepted["Modules"]), 16)  # root + 15 calls
            calls = build_chain(16)
            with mock.patch.object(preseed, "scan", side_effect=lambda _, directory: calls[directory]):
                with self.assertRaisesRegex(ValueError, "module fan-out/depth cap exceeded"):
                    preseed.module_manifest(base, pathlib.Path("scanner"), subject,
                                            base / "scratch", ".", 0)


if __name__ == "__main__":
    unittest.main()
