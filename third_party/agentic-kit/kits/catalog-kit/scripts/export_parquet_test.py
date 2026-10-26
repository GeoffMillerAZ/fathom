import json
from pathlib import Path
import tempfile
import unittest

import export_parquet as adapter


class ExportTests(unittest.TestCase):
    def row(self):
        return dict(id="review", parent_id="", title="Evidence", synopsis="", order=0,
                    stable_key="review", hidden=False, facets={"language": ["go", "rust"]},
                    source="general.cue", extractor="", watermark="", layers=["general"])

    def test_roundtrip_and_empty_projection(self):
        import pyarrow.parquet as pq
        with tempfile.TemporaryDirectory() as root:
            source, target = Path(root) / "rows.ndjson", Path(root) / "rows.parquet"
            for rows in [[self.row()], []]:
                source.write_text("".join(json.dumps(row) + "\n" for row in rows))
                adapter.export(source, target, "a" * 64)
                result = pq.read_table(target)
                self.assertEqual(result.num_rows, len(rows))
                self.assertEqual(result.schema.metadata[b"catalog.bundle_digest"], b"a" * 64)
                if rows:
                    row = result.to_pylist()[0]
                    row["facets"] = dict(row["facets"])
                    self.assertEqual(row, rows[0])

    def test_rejection_preserves_prior_output(self):
        with tempfile.TemporaryDirectory() as root:
            source, target = Path(root) / "rows.ndjson", Path(root) / "rows.parquet"
            target.write_bytes(b"prior-output")
            for content in ["{}\n", '{"id":"a","id":"b"}\n', json.dumps(self.row()) + "\n" + json.dumps(self.row()) + "\n"]:
                source.write_text(content)
                with self.assertRaises(ValueError):
                    adapter.export(source, target, "a" * 64)
                self.assertEqual(target.read_bytes(), b"prior-output")


if __name__ == "__main__":
    unittest.main()
