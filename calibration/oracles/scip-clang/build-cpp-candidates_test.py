import argparse
import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("cpp_builder", HERE / "build-cpp-candidates.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


class SourceBoundaryTests(unittest.TestCase):
    def test_exact_archive_size_and_growth_boundary(self):
        with tempfile.TemporaryDirectory(dir=builder.builder.ROOT / "workspace") as temp:
            target = Path(temp) / "archive.tar.gz"
            data = b"x" * 100
            with mock.patch.object(builder.urllib.request, "urlopen", return_value=io.BytesIO(data)):
                builder.fetch("https://example.invalid/archive", hashlib.sha256(data).hexdigest(), 100, target)
            self.assertEqual(target.read_bytes(), data)
            target.unlink()
            class Growing(io.BytesIO):
                def read(self, size=-1):
                    self.requested = size
                    return super().read(size)
            response = Growing(data + b"y" * 10000)
            with mock.patch.object(builder.urllib.request, "urlopen", return_value=response):
                with self.assertRaises(builder.audit.Refusal):
                    builder.fetch("https://example.invalid/archive", hashlib.sha256(data).hexdigest(), 100, target)
            self.assertEqual(response.requested, 101)
            self.assertFalse(target.exists())

    def test_runtime_fetch_uses_exact_commit(self):
        with tempfile.TemporaryDirectory(dir=builder.builder.ROOT / "workspace") as temp:
            work, out = Path(temp) / "work", Path(temp) / "out"
            urls, prefixes = [], []

            def fetch(url, _sha, _size, _archive):
                urls.append(url)

            def extract(_archive, dest, _prefix):
                prefixes.append(_prefix)
                (dest / "LICENSE").write_text("MIT")
                return (1, 1, 3)

            def digest(path):
                if path.name == "LICENSE":
                    if path.parent.name == "runtime":
                        return builder.CORE[4]
                    return builder.CANDIDATES["official-head"][3]
                self.fail(f"unexpected digest: {path}")

            class StopAfterOpt(Exception):
                pass

            calls = []

            def check_run(argv, cwd, stdout, stderr, _timeout, _cap, env=None):
                calls.append((argv, cwd, stdout, stderr, env))
                self.assertEqual(cwd, work)
                self.assertEqual(env["ZIG_GLOBAL_CACHE_DIR"], str(work / "zig-global-cache"))
                self.assertEqual(env["ZIG_LOCAL_CACHE_DIR"], str(work / "zig-local-cache"))
                if len(calls) == 2:
                    raise StopAfterOpt

            with mock.patch.object(builder.builder, "tool", side_effect=lambda name, *_: (name, "hash")), \
                    mock.patch.object(builder, "fetch", side_effect=fetch), \
                    mock.patch.object(builder.builder, "extract", side_effect=extract), \
                    mock.patch.object(builder.audit, "digest", side_effect=digest), \
                    mock.patch.object(builder.audit, "run", side_effect=check_run):
                with self.assertRaises(StopAfterOpt):
                    builder.build(argparse.Namespace(work_dir=work, out_dir=out,
                                                     candidate="official-head", gzip=False))
            self.assertEqual(urls[0], "https://codeload.github.com/tree-sitter/tree-sitter/tar.gz/" + builder.CORE[2])
            self.assertEqual(prefixes[0], "tree-sitter-64402de2857cc197ecc4ca3bc144ea91fda7e72e")
            self.assertEqual(builder.CORE[3], "fc2a4d846c36f7d19714085bbad76a68c5ff9d0fe7b1e2a6a62a22446efe966e")
            self.assertEqual(builder.CORE[5], 920896)
            self.assertEqual(len(calls), 2)
            self.assertEqual(calls[0][0][:2], ["zig", "cc"])
            self.assertEqual(calls[0][2:4], (work / "zig.stdout", work / "zig.stderr"))
            self.assertEqual(calls[1][0][0], "wasm-opt")
            self.assertEqual(calls[1][2:4], (work / "opt.stdout", work / "opt.stderr"))

    def test_corrupt_archive_refused_without_path(self):
        with tempfile.TemporaryDirectory(dir=builder.builder.ROOT / "workspace") as temp:
            work = Path(temp)
            with mock.patch.object(builder, "fetch", side_effect=lambda _url, _sha, _size, archive: archive.write_bytes(b"corrupt")):
                with self.assertRaises(tarfile.TarError):
                    builder.source(work, "runtime", *builder.CORE[:2], builder.CORE[3],
                                   builder.CORE[4], builder.CORE[5], builder.CORE[6], "https://example.invalid")
            with mock.patch.object(builder.builder, "tool", side_effect=lambda name, *_: (name, "hash")), \
                    mock.patch.object(builder, "fetch", side_effect=lambda _url, _sha, _size, archive: archive.write_bytes(b"corrupt")):
                with self.assertRaises(SystemExit) as caught, mock.patch("sys.stderr", new_callable=io.StringIO) as stderr:
                    builder.main(["--candidate", "official-head", "--work-dir", str(work / "cli-work"),
                                  "--out-dir", str(work / "cli-out")])
            self.assertEqual(caught.exception.code, 1)
            self.assertEqual(stderr.getvalue(), "build-cpp-candidates: corrupt source archive\n")


if __name__ == "__main__":
    unittest.main()
