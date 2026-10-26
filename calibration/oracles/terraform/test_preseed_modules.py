import importlib.util
import pathlib
import tempfile
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "preseed_modules", pathlib.Path(__file__).with_name("preseed_modules.py"))
preseed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preseed)


class PreseedTests(unittest.TestCase):
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
