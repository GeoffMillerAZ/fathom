#!/usr/bin/env python3
"""Convert catalogctl NDJSON rows to a typed Parquet analytics projection.

Optional adapter dependency: Apache PyArrow. The Go core has no format dependency.
Source bytes and a caller-supplied bundle digest are recorded, not authenticated.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import tempfile


FIELDS = {"id", "parent_id", "title", "synopsis", "order", "stable_key", "hidden",
          "facets", "source", "extractor", "watermark", "layers"}


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_rows(data):
    if len(data) > 16 * 1024 * 1024:
        raise ValueError("NDJSON exceeds 16 MiB")
    rows = []
    seen = set()
    for line in data.decode("utf-8").splitlines():
        if not line.strip():
            raise ValueError("blank NDJSON row")
        row = json.loads(line, object_pairs_hook=unique_object)
        if not isinstance(row, dict) or set(row) != FIELDS:
            raise ValueError("unexpected row fields")
        for field in FIELDS - {"order", "hidden", "facets", "layers"}:
            if not isinstance(row[field], str):
                raise ValueError(f"{field} must be text")
        if type(row["order"]) is not int or type(row["hidden"]) is not bool:
            raise ValueError("order/hidden types are invalid")
        if not row["id"] or row["id"] in seen:
            raise ValueError("empty or duplicate row id")
        seen.add(row["id"])
        if not isinstance(row["facets"], dict):
            raise ValueError("facets must be an object")
        values = list(row["facets"].values()) + [row["layers"]]
        if any(not isinstance(v, list) or any(not isinstance(x, str) for x in v) for v in values):
            raise ValueError("facet values and layers must be text lists")
        rows.append(row)
        if len(rows) > 100000:
            raise ValueError("row limit exceeded")
    return sorted(rows, key=lambda row: row["id"])


def export(source, destination, bundle_digest):
    if not re.fullmatch(r"[a-f0-9]{64}", bundle_digest):
        raise ValueError("bundle digest must be lowercase SHA-256")
    if Path(source).resolve() == Path(destination).resolve():
        raise ValueError("source and destination must differ")
    # Read at most the documented bound plus one byte, before parsing.
    with Path(source).open("rb") as stream:
        data = stream.read(16 * 1024 * 1024 + 1)
    rows = load_rows(data)
    try:
        import pyarrow as pa
        import pyarrow.parquet as pq
    except ImportError as exc:
        raise RuntimeError("This optional adapter requires pyarrow") from exc
    schema = pa.schema([
        pa.field("id", pa.string(), nullable=False),
        pa.field("parent_id", pa.string(), nullable=False),
        pa.field("title", pa.string(), nullable=False),
        pa.field("synopsis", pa.string(), nullable=False),
        pa.field("order", pa.int64(), nullable=False),
        pa.field("stable_key", pa.string(), nullable=False),
        pa.field("hidden", pa.bool_(), nullable=False),
        pa.field("facets", pa.map_(pa.string(), pa.list_(pa.string())), nullable=False),
        pa.field("source", pa.string(), nullable=False),
        pa.field("extractor", pa.string(), nullable=False),
        pa.field("watermark", pa.string(), nullable=False),
        pa.field("layers", pa.list_(pa.string()), nullable=False),
    ], metadata={b"catalog.projection_version": b"1",
                 b"catalog.bundle_digest": bundle_digest.encode("ascii"),
                 b"catalog.ndjson_sha256": hashlib.sha256(data).hexdigest().encode("ascii")})
    table = pa.Table.from_pylist(rows, schema=schema)
    destination = Path(destination)
    # Validate and write completely before replacing a prior output.
    with tempfile.NamedTemporaryFile(dir=destination.parent, suffix=".parquet", delete=False) as stream:
        temporary = Path(stream.name)
    try:
        pq.write_table(table, temporary, compression="zstd", version="2.6")
        temporary.replace(destination)
    finally:
        temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("destination", type=Path)
    parser.add_argument("--bundle-digest", required=True)
    args = parser.parse_args()
    export(args.source, args.destination, args.bundle_digest)


if __name__ == "__main__":
    main()
