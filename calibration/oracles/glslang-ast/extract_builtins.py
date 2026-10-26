#!/usr/bin/env python3
"""Extract factual GLSL 4.60 function identifiers from official §8 tables.

Consumes the locally saved official HTML. Checks every §8.1–8.19 first-column
table cell for unrecognized function syntax before issuing complete=true.
"""
import argparse
import hashlib
import html
import json
import re
from pathlib import Path

SOURCE = "https://registry.khronos.org/OpenGL/specs/gl/GLSLangSpec.4.60.html"
REVISION = "747576a437935b79d72cc07b6cbed7136b613e71"
SECTION = re.compile(r'<h3 id="[^"]+">8\.(\d+)\. [^<]+</h3>')
FIRST_CELL = re.compile(r'<tr>\s*<td[^>]*>(.*?)</td>', re.S)
FUNCTION = re.compile(r'<strong>([A-Za-z_]\w*)</strong>\s*\(')
TYPE_CELL = re.compile(r'<tr>\s*<td[^>]*>\s*<p[^>]*>\s*<strong>([A-Za-z_]\w*)</strong>', re.S)


def extract(raw):
    start = raw.index('<h2 id="built-in-functions">')
    end = raw.index('<h2 id="shading-language-grammar">', start)
    section = raw[start:end]
    headings = list(SECTION.finditer(section))
    if [int(m.group(1)) for m in headings] != list(range(1, 20)):
        raise ValueError("expected every GLSL §8.1–8.19 heading")
    names = set()
    coverage = {}
    for i, heading in enumerate(headings):
        body = section[heading.end():headings[i + 1].start() if i + 1 < len(headings) else len(section)]
        cells = FIRST_CELL.findall(body)
        found = set()
        for cell in cells:
            matched = FUNCTION.findall(cell)
            plain = html.unescape(re.sub(r"<[^>]+>", "", cell))
            if "(" in plain and not matched:
                raise ValueError(f"unrecognized function cell in §8.{i + 1}: {plain[:80]}")
            found.update(matched)
        if not found:
            raise ValueError(f"no function names extracted from §8.{i + 1}")
        coverage[f"8.{i + 1}"] = {"first_column_cells": len(cells), "distinct_names": len(found)}
        names.update(found)
    return sorted(names), coverage


def extract_types(raw):
    """Read GLSL 4.60 basic-type tables and image-type keywords."""
    start = raw.index('<h3 id="basic-types">')
    end = raw.index('<h3 id=', start + 1)
    tables = re.findall(r'<table\b.*?</table>', raw[start:end], re.S)
    if len(tables) < 5:
        raise ValueError("expected transparent and four opaque basic-type tables")
    groups = [set(TYPE_CELL.findall(table)) for table in tables[:5]]
    if any(not group for group in groups):
        raise ValueError("empty basic-type table")
    keywords = raw[raw.index('<h3 id="keywords">'):start]
    image_types = set(re.findall(r'<strong>((?:i|u)?image[A-Za-z0-9_]*)</strong>', keywords))
    if not image_types:
        raise ValueError("no image-type keywords")
    types = set().union(*groups, image_types) - {"void"}
    return sorted(types), {"basic_type_table_counts": [len(g) for g in groups],
                           "image_type_keyword_count": len(image_types)}


def build(path):
    data = path.read_bytes()
    names, coverage = extract(data.decode("utf-8"))
    types, type_coverage = extract_types(data.decode("utf-8"))
    return {"spec": "GLSL 4.60", "complete": True, "source": SOURCE,
            "source_revision": REVISION, "source_sha256": hashlib.sha256(data).hexdigest(),
            "extraction": "Section 8.1–8.19 table first-column <strong>function</strong>( names",
            "coverage": coverage, "functions": names, "types": types,
            "type_coverage": type_coverage}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("html", type=Path)
    ap.add_argument("out", type=Path)
    args = ap.parse_args()
    out = build(args.html)
    args.out.write_text(json.dumps(out, indent=2) + "\n")
    print(f"sections={len(out['coverage'])} names={len(out['functions'])}")


if __name__ == "__main__":
    main()
