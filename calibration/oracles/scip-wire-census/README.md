# SCIP range wire census

`census.py` reads a SCIP v0.9.0 protobuf index with Python's standard library. It counts every document and occurrence, then reports the `.kt` and `.kts` subset separately. It counts `Document.position_encoding` values (field 6, including absent/default 0), deprecated packed/unpacked 3- and 4-integer ranges, typed single- and multi-line ranges, sign/order validity, no range, zero width, both encodings, definition-role occurrences, and all enclosing-range forms. Each encoding has at most three samples containing only a relative path and numbers. The JSON records the input SHA-256 and byte count; it does not copy `Metadata.project_root`, symbols, source text, or absolute paths.

```sh
python3 calibration/oracles/scip-wire-census/census.py --self-test
python3 calibration/oracles/scip-wire-census/census.py /scratch/exposed/index.scip --out /scratch/exposed/scip-census.json
```

`--self-test` checks every reported counter on distinct synthetic range forms, typed-over-legacy precedence, position encodings, and malformed-wire rejection cases. It never examines a corpus and proves no language grade. Run the census on the real pinned Exposed index after the producer completes; keep the producer's source commit and index digest beside this JSON. A malformed wire exits 2. The parser rejects truncated fields, unsupported wire types, overlong/overflowing varints, wrong wire types for fields it interprets, and noncanonical document paths.

Sign/order validity checks only nonnegative coordinates and ordering. It does **not** check line count, line length, line-base choice, or whether a character column matches the document's declared UTF-8/UTF-16/UTF-32 position encoding. Range-to-source alignment is a separate required check on the first real index before claiming reader compatibility.
