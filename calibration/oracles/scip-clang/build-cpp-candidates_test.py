import hashlib
import importlib.util
import io
from pathlib import Path
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


if __name__ == "__main__":
    unittest.main()
