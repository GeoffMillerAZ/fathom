"""Opt-in, pinned P01 font experiment. Run a retained copy in a private output root."""
import hashlib
from copy import deepcopy
import json
import os
from pathlib import Path
import sys

import brotli
import fontTools
from fontTools import subset
from fontTools.ttLib import TTFont

SOURCE_SHA = "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"
RANGES = [(0, 0x24F), (0x300, 0x36F), (0x2000, 0x206F), (0x20A0, 0x20CF), (0x2190, 0x21FF)]
MAX_BYTES = 196608


def digest(data):
    return hashlib.sha256(data).hexdigest()


def names(font):
    return sorted((r.nameID, r.platformID, r.platEncID, r.langID, r.string.hex()) for r in font["name"].names)


def generate(source, destination):
    if fontTools.__version__ != "4.65.0" or brotli.__version__ != "1.2.0":
        raise ValueError("requires private pinned FontTools4.65.0 and Brotli1.2.0")
    if not source.is_absolute() or not destination.is_absolute() or source.is_symlink() or not source.is_file():
        raise ValueError("absolute real regular source and new absolute destination required")
    raw = source.read_bytes()
    if len(raw) != 352240 or digest(raw) != SOURCE_SHA:
        raise ValueError("retained original font identity mismatch")
    destination.mkdir(mode=0o700)  # Existing output is refused, never repaired.
    src = TTFont(source, recalcTimestamp=False)
    font = TTFont(source, recalcTimestamp=False)
    original_cmap = src.getBestCmap()
    selected = sorted(cp for cp in original_cmap if any(lo <= cp <= hi for lo, hi in RANGES))
    outside = sorted(cp for cp in original_cmap if cp not in selected and not 0xD800 <= cp <= 0xDFFF)
    if not selected or not outside:
        raise ValueError("expected both selected and original-only coverage")
    options = subset.Options()
    options.layout_features = ["*"]
    options.layout_scripts = ["*"]
    options.name_IDs = ["*"]
    options.name_languages = ["*"]
    options.name_legacy = True
    options.glyph_names = True
    options.hinting = True
    options.notdef_outline = True
    options.recalc_timestamp = False
    options.drop_tables = ["DSIG"]  # A source signature cannot sign a derived font.
    options.passthrough_tables = True
    options.harfbuzz_repacker = False
    worker = subset.Subsetter(options=options)
    worker.populate(unicodes=selected)
    worker.subset(font)
    # FontTools prunes names referenced only by removed layout features even
    # with name_IDs=['*']. Retain all source metadata, including license and
    # feature labels; the exact post-serialization equality guard stays below.
    font["name"] = deepcopy(src["name"])
    font.flavor = "woff2"
    font.save(destination / "inter-subset.woff2")
    actual = TTFont(destination / "inter-subset.woff2", recalcTimestamp=False)
    if actual.getBestCmap() != {cp: original_cmap[cp] for cp in selected}:
        raise ValueError("subset cmap differs from selected original mappings")
    if names(src) != names(actual):
        raise ValueError("font name/license records changed")
    if src["head"].unitsPerEm != actual["head"].unitsPerEm:
        raise ValueError("units per em changed")
    table_evidence = {}
    for tag in ["fvar", "gvar", "avar", "STAT", "HVAR", "MVAR", "GSUB", "GPOS", "GDEF", "cvt ", "fpgm", "prep"]:
        if (tag in src) != (tag in actual):
            raise ValueError("table presence changed: " + tag)
        table_evidence[tag] = "present" if tag in src else "absent-in-original"
        if tag in src and tag in ["fvar", "avar", "STAT", "MVAR", "cvt ", "fpgm", "prep"]:
            if src[tag].compile(src) != actual[tag].compile(actual):
                raise ValueError("unchanged axis/metrics/hint table changed: " + tag)
            table_evidence[tag] = "byte-identical"
    retained = actual.getGlyphOrder()
    for name in retained:
        if src["glyf"][name].getCoordinates(src["glyf"]) != actual["glyf"][name].getCoordinates(actual["glyf"]):
            raise ValueError("glyph outline differs: " + name)
        if src["hmtx"][name] != actual["hmtx"][name]:
            raise ValueError("glyph advance/bearing differs: " + name)
        if getattr(src["glyf"][name], "program", None) != getattr(actual["glyf"][name], "program", None):
            raise ValueError("glyph hint program differs: " + name)
        if src["gvar"].variations.get(name, []) != actual["gvar"].variations.get(name, []):
            raise ValueError("glyph variation deltas differ: " + name)
    src.flavor = None
    src.save(destination / "original.ttf")
    actual.flavor = None
    actual.save(destination / "subset.ttf")
    outputs = {}
    for filename in ["inter-subset.woff2", "original.ttf", "subset.ttf"]:
        target = destination / filename
        os.chmod(target, 0o600)
        data = target.read_bytes()
        outputs[filename] = {"sha256": digest(data), "bytes": len(data)}
    if outputs["inter-subset.woff2"]["bytes"] > MAX_BYTES:
        raise ValueError("subset exceeds prospective 192KiB font allocation")
    axes = [{"tag": a.axisTag, "min": a.minValue, "default": a.defaultValue, "max": a.maxValue} for a in src["fvar"].axes]
    result = {"schema": 1, "status": "experimental-not-certified", "sourceSHA": SOURCE_SHA,
              "scriptSHA": digest(Path(__file__).read_bytes()), "fonttools": fontTools.__version__,
              "brotli": brotli.__version__, "ranges": RANGES, "selectedCodepoints": selected,
              "originalOnlyCodepoints": outside, "retainedGlyphs": len(retained), "originalGlyphs": len(src.getGlyphOrder()),
              "axes": axes, "tables": table_evidence, "outputs": outputs}
    with (destination / "font-receipt.json").open("x") as handle:
        json.dump(result, handle, indent=2)
        handle.write("\n")
    os.chmod(destination / "font-receipt.json", 0o600)
    if digest(source.read_bytes()) != SOURCE_SHA:
        raise ValueError("original font changed during generation")
    print(json.dumps({"destination": str(destination), "subset": outputs["inter-subset.woff2"], "glyphs": len(retained)}))


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("usage: font-subset.py ABS_ORIGINAL_WOFF2 NEW_PRIVATE_DIRECTORY")
    generate(Path(sys.argv[1]), Path(sys.argv[2]))
