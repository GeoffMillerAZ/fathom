#!/usr/bin/env python3
"""Count SCIP v0.9.0 occurrence range encodings without protobuf dependencies."""

import argparse
import hashlib
import json
import pathlib
import sys


class WireError(ValueError):
    pass


def varint(data, pos):
    value = 0
    for shift in range(0, 70, 7):
        if pos >= len(data):
            raise WireError("truncated varint")
        byte = data[pos]
        pos += 1
        if shift == 63 and byte > 1:
            raise WireError("varint exceeds uint64")
        value |= (byte & 0x7f) << shift
        if not byte & 0x80:
            return value, pos
    raise WireError("varint exceeds ten bytes")


def fields(data):
    pos = 0
    while pos < len(data):
        tag, pos = varint(data, pos)
        number, wire = tag >> 3, tag & 7
        if number == 0:
            raise WireError("field number zero")
        if wire == 0:
            value, pos = varint(data, pos)
        elif wire == 1:
            end = pos + 8
            if end > len(data):
                raise WireError("truncated fixed64")
            value, pos = data[pos:end], end
        elif wire == 2:
            size, pos = varint(data, pos)
            end = pos + size
            if end > len(data):
                raise WireError("truncated length-delimited field")
            value, pos = data[pos:end], end
        elif wire == 5:
            end = pos + 4
            if end > len(data):
                raise WireError("truncated fixed32")
            value, pos = data[pos:end], end
        else:
            raise WireError(f"unsupported wire type {wire}")
        yield number, wire, value


def int32(value):
    value &= 0xffffffff
    return value - 0x100000000 if value & 0x80000000 else value


def legacy(values, wire, value):
    if wire == 0:
        values.append(int32(value))
    elif wire == 2:
        pos = 0
        while pos < len(value):
            number, pos = varint(value, pos)
            values.append(int32(number))
    else:
        raise WireError("legacy range has wrong wire type")


def valid_sign_order(numbers):
    if len(numbers) == 3:
        line, start, end = numbers
        return min(numbers) >= 0 and end >= start
    if len(numbers) == 4:
        sl, sc, el, ec = numbers
        return min(numbers) >= 0 and (el > sl or (el == sl and ec >= sc))
    return False


def typed_range(data, multi):
    slots = {number: 0 for number in range(1, 5 if multi else 4)}
    for number, wire, value in fields(data):
        if number in slots:
            if wire != 0:
                raise WireError("typed range integer has wrong wire type")
            slots[number] = int32(value)
    return [slots[number] for number in sorted(slots)]


def occurrence(data):
    own, enclosing = [], []
    own_typed, enclosing_typed = [], []
    roles = 0
    own_seen = enclosing_seen = False
    for number, wire, value in fields(data):
        if number in (1, 7):
            legacy(own if number == 1 else enclosing, wire, value)
            own_seen = own_seen or number == 1
            enclosing_seen = enclosing_seen or number == 7
        elif number in (8, 9, 10, 11):
            if wire != 2:
                raise WireError("typed range has wrong wire type")
            item = ("single" if number in (8, 10) else "multi", typed_range(value, number in (9, 11)))
            (own_typed if number in (8, 9) else enclosing_typed).append(item)
        elif number == 3:
            if wire != 0:
                raise WireError("symbol roles has wrong wire type")
            roles = int32(value)
    return own, own_seen, own_typed, enclosing, enclosing_seen, enclosing_typed, bool(roles & 1)


def relative_path(value):
    try:
        path = value.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise WireError("document path is not UTF-8") from exc
    parts = path.split("/")
    if not path or path.startswith("/") or (len(path) > 1 and path[1] == ":") or "\\" in path or any(p in ("", ".", "..") for p in parts):
        raise WireError("document path is not canonical relative form")
    return path


def bucket():
    return {"documents": 0, "position_encoding": {}, "occurrences": 0, "definitions": 0,
            "legacy_present": 0, "legacy_empty": 0, "legacy_valid_3": 0, "legacy_valid_4": 0,
            "legacy_invalid": 0, "typed_single": 0, "typed_multi": 0,
            "typed_invalid_sign_order": 0, "both_legacy_typed": 0,
            "both_typed_forms": 0, "no_range": 0, "zero_width": 0,
            "enclosing_legacy": 0, "enclosing_legacy_empty": 0, "enclosing_legacy_valid_3": 0,
            "enclosing_legacy_valid_4": 0, "enclosing_legacy_invalid": 0,
            "enclosing_typed_single": 0, "enclosing_typed_multi": 0,
            "enclosing_typed_invalid_sign_order": 0, "enclosing_both": 0,
            "samples": {key: [] for key in ("legacy", "typed_single", "typed_multi", "no_range",
                                                "both", "enclosing_legacy", "enclosing_typed_single",
                                                "enclosing_typed_multi")}}


def sample(stats, name, path, numbers):
    items = stats["samples"][name]
    if len(items) < 3:
        items.append({"relative_path": path, "range": numbers})


def add_occurrence(stats, path, raw):
    own, own_seen, typed, enclosing, enclosing_seen, enclosed_typed, definition = occurrence(raw)
    stats["occurrences"] += 1
    stats["definitions"] += int(definition)
    if own_seen:
        stats["legacy_present"] += 1
        stats["legacy_empty"] += int(not own)
        shape = len(own)
        key = f"legacy_valid_{shape}" if shape in (3, 4) and valid_sign_order(own) else "legacy_invalid"
        stats[key] += 1
        sample(stats, "legacy", path, own)
    kinds = {kind for kind, _ in typed}
    for kind, numbers in typed:
        stats["typed_" + kind] += 1
        stats["typed_invalid_sign_order"] += int(not valid_sign_order(numbers))
        sample(stats, "typed_" + kind, path, numbers)
    if len(kinds) > 1:
        stats["both_typed_forms"] += 1
    if own and typed:
        stats["both_legacy_typed"] += 1
        sample(stats, "both", path, typed[-1][1])
    if not own and not typed:
        stats["no_range"] += 1
        sample(stats, "no_range", path, [])
    effective = typed[-1][1] if typed else own
    if effective and (len(effective) == 3 and effective[1] == effective[2] or
                      len(effective) == 4 and effective[0] == effective[2] and effective[1] == effective[3]):
        stats["zero_width"] += 1
    if enclosing_seen:
        stats["enclosing_legacy"] += 1
        stats["enclosing_legacy_empty"] += int(not enclosing)
        shape = len(enclosing)
        key = f"enclosing_legacy_valid_{shape}" if shape in (3, 4) and valid_sign_order(enclosing) else "enclosing_legacy_invalid"
        stats[key] += 1
        sample(stats, "enclosing_legacy", path, enclosing)
    for kind, numbers in enclosed_typed:
        stats["enclosing_typed_" + kind] += 1
        stats["enclosing_typed_invalid_sign_order"] += int(not valid_sign_order(numbers))
        sample(stats, "enclosing_typed_" + kind, path, numbers)
    stats["enclosing_both"] += int(bool(enclosing and enclosed_typed))


def document(data, all_stats, kotlin_stats):
    path = None
    language = ""
    position_encoding = 0
    occurrences = []
    for number, wire, value in fields(data):
        if number == 1:
            if wire != 2:
                raise WireError("relative_path has wrong wire type")
            path = relative_path(value)
        elif number == 2:
            if wire != 2:
                raise WireError("occurrence has wrong wire type")
            occurrences.append(value)
        elif number == 4:
            if wire != 2:
                raise WireError("language has wrong wire type")
            try:
                language = value.decode("utf-8")
            except UnicodeDecodeError as exc:
                raise WireError("language is not UTF-8") from exc
        elif number == 6:
            if wire != 0:
                raise WireError("position_encoding has wrong wire type")
            position_encoding = value
    if path is None:
        raise WireError("document has no relative_path")
    groups = [all_stats]
    if path.endswith((".kt", ".kts")):
        groups.append(kotlin_stats)
    for stats in groups:
        stats["documents"] += 1
        encoding = str(position_encoding)
        stats["position_encoding"][encoding] = stats["position_encoding"].get(encoding, 0) + 1
        for raw in occurrences:
            add_occurrence(stats, path, raw)


def census(data):
    all_stats, kotlin_stats = bucket(), bucket()
    for number, wire, value in fields(data):
        if number == 2:
            if wire != 2:
                raise WireError("Index.documents has wrong wire type")
            document(value, all_stats, kotlin_stats)
    return {"all": all_stats, "kotlin": kotlin_stats}


def encode_varint(value):
    out = bytearray()
    while value > 127:
        out.append((value & 127) | 128)
        value >>= 7
    out.append(value)
    return bytes(out)


def wire(number, value, kind=2):
    return encode_varint(number << 3 | kind) + (encode_varint(len(value)) + value if kind == 2 else encode_varint(value))


def self_test():
    def packed(*numbers):
        return b"".join(encode_varint(n if n >= 0 else (1 << 64) + n) for n in numbers)

    def single(*numbers):
        return b"".join(wire(i, n if n >= 0 else (1 << 64) + n, 0) for i, n in enumerate(numbers, 1))

    def multi(*numbers):
        return single(*numbers)

    s = single(4, 2, 6)
    m = multi(5, 0, 6, 1)
    both_last = multi(9, 0, 10, 1)
    occurrences = [
        wire(1, packed(4, 2, 6)) + wire(8, s) + wire(3, 1, 0) +
        wire(7, packed(4, 2, 6)) + wire(10, s),
        wire(9, m) + wire(11, m),
        wire(9, multi(7, 0, 8, 1)) + wire(11, multi(7, 0, 8, 1)),
        wire(1, packed(9, 0, 1)) + wire(8, single(9, 2, 2)) + wire(9, both_last) +
        wire(10, single(9, 2, 3)) + wire(11, both_last),
        wire(8, single(3, 1, 1)) + wire(10, single(3, 1, 2)),
        wire(8, single(3, -1, 2)) + wire(10, single(3, -1, 2)),
        wire(9, multi(5, 4, 5, 1)) + wire(11, multi(5, 4, 5, 1)),
        wire(7, b""),
        wire(1, b""),
        wire(1, packed(1, 0, 2, 0)) + wire(7, packed(1, 0, 2, 0)),
        wire(1, packed(-1, 0, 1)) + wire(7, packed(-1, 0, 1)),
        wire(1, packed(2, 2, 1, 1)) + wire(7, packed(2, 2, 1, 1)),
        wire(1, packed(6, 1, 2)) + wire(7, packed(6, 1, 2)),
        wire(9, multi(11, 0, 12, 1)) + wire(10, single(11, 0, 1)),
    ]
    doc = wire(1, b"src/A.kt") + wire(4, b"kotlin") + wire(6, 2, 0)
    doc += b"".join(wire(2, raw) for raw in occurrences)
    other_kotlin = wire(1, b"src/B.kts")
    other = wire(1, b"src/C.txt") + wire(6, 1, 0)
    result = census(wire(2, doc) + wire(2, other_kotlin) + wire(2, other))
    k = result["kotlin"]
    expected = {
        "documents": 2, "position_encoding": {"0": 1, "2": 1},
        "occurrences": 14, "definitions": 1,
        "legacy_present": 7, "legacy_empty": 1, "legacy_valid_3": 3, "legacy_valid_4": 1,
        "legacy_invalid": 3, "typed_single": 4, "typed_multi": 5,
        "typed_invalid_sign_order": 2, "both_legacy_typed": 2,
        "both_typed_forms": 1, "no_range": 2, "zero_width": 1,
        "enclosing_legacy": 6, "enclosing_legacy_empty": 1, "enclosing_legacy_valid_3": 2,
        "enclosing_legacy_valid_4": 1, "enclosing_legacy_invalid": 3,
        "enclosing_typed_single": 5, "enclosing_typed_multi": 4,
        "enclosing_typed_invalid_sign_order": 2, "enclosing_both": 1,
    }
    assert {name: value for name, value in k.items() if name != "samples"} == expected
    assert result["all"]["documents"] == 3
    assert result["all"]["position_encoding"] == {"0": 1, "1": 1, "2": 1}
    assert result["all"]["occurrences"] == 14
    assert k["samples"]["both"][1]["range"] == [9, 0, 10, 1]
    assert k["samples"]["legacy"][0]["relative_path"] == "src/A.kt"
    assert {name: len(items) for name, items in k["samples"].items()} == {
        "legacy": 3, "typed_single": 3, "typed_multi": 3, "no_range": 2,
        "both": 2, "enclosing_legacy": 3, "enclosing_typed_single": 3,
        "enclosing_typed_multi": 3,
    }

    malformed = (
        b"\x80" * 11,                         # varint over ten bytes
        wire(2, wire(1, b"a.kt") + b"\x30" + b"\xff" * 9 + b"\x02"),  # field-6 value overflows uint64
        b"\x80",                              # truncated varint
        b"\x12\x05x",                         # truncated length-delimited field
        b"\x09x",                             # truncated fixed64
        b"\x0dx",                             # truncated fixed32
        b"\x0b",                              # unsupported wire type
        b"\x00",                              # field number zero
        wire(2, 1, 0),                         # documents wrong wire
        wire(2, wire(1, 1, 0)),               # path wrong wire
        wire(2, wire(1, b"a.kt") + wire(2, 1, 0)),  # occurrence wrong wire
        wire(2, wire(1, b"a.kt") + wire(4, 1, 0)),  # language wrong wire
        wire(2, wire(1, b"a.kt") + wire(6, b"x")),  # position encoding wrong wire
        wire(2, wire(1, b"a.kt") + wire(2, wire(8, 1, 0))),  # typed wrong wire
        wire(2, wire(1, b"a.kt") + wire(2, wire(8, wire(1, b"x")))),  # typed integer wrong wire
        wire(2, wire(1, b"a.kt") + wire(2, b"\x0dxxxx")),  # legacy wrong wire
        wire(2, wire(1, b"a.kt") + wire(2, wire(3, b"x"))),  # role wrong wire
        wire(2, wire(1, b"/absolute.kt")),
        wire(2, wire(1, b"../escape.kt")),
        wire(2, wire(1, b"a//b.kt")),
        wire(2, wire(1, b"C:\\a.kt")),
        wire(2, wire(1, b"")),
        wire(2, wire(1, b"\xff.kt")),
        wire(2, wire(1, b"a.kt") + wire(4, b"\xff")),
    )
    for raw in malformed:
        try:
            census(raw)
        except WireError:
            pass
        else:
            raise AssertionError("malformed wire accepted")
    print("SELF-TEST PASS: tiny synthetic wire/schema checks only; no corpus or grade proof")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("index", nargs="?", type=pathlib.Path)
    ap.add_argument("--out", type=pathlib.Path)
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args()
    if args.self_test:
        self_test()
        return
    if args.index is None or args.out is None:
        ap.error("index.scip and --out JSON are required")
    data = args.index.read_bytes()
    result = {"index_sha256": hashlib.sha256(data).hexdigest(), "index_bytes": len(data)}
    result.update(census(data))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"index_sha256": result["index_sha256"], "index_bytes": result["index_bytes"],
                      "documents": result["all"]["documents"], "kotlin_documents": result["kotlin"]["documents"]}))


if __name__ == "__main__":
    try:
        main()
    except WireError as exc:
        print(f"malformed SCIP wire: {exc}", file=sys.stderr)
        raise SystemExit(2)
