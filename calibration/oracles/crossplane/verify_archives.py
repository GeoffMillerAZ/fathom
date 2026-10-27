"""Explicit calibration setup: fetch each archive by its own repo/SHA and verify pins.

No extraction or subject metadata is published. Without --fetch this is offline.
"""

import argparse
import hashlib
import json
from pathlib import Path
import re
import tarfile
import urllib.request

PINS = Path(__file__).with_name("pins-r1.json")
MAX_BYTES = 256 * 1024 * 1024


def archive_identity(row):
    repo, sha = row["repo"], row["sha"]
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", repo) or not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("invalid repository or commit pin")
    return f"{repo.replace('/', '--')}--{sha}.tar.gz"


def verify(row, path):
    archive_identity(row)
    size = path.stat().st_size
    if size > MAX_BYTES:
        raise ValueError("archive exceeds byte cap")
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    # GitHub redirects renamed repositories (provider-aws -> provider-upjet-aws).
    # The fetched URL binds the repository; require one root with the exact SHA.
    root = None
    # Inspect only; never extract subject contents, names or people into a receipt.
    with tarfile.open(path, "r:gz") as archive:
        for member in archive:
            current = member.name.split("/", 1)[0]
            if not re.fullmatch(r"[\w.-]+-" + row["sha"], current) or (root and current != root):
                raise ValueError("archive root does not match its subject commit")
            root = current
    if root is None:
        raise ValueError("empty source archive")
    result = {"repo": row["repo"], "sha": row["sha"], "archive_sha256": digest,
              "archive_bytes": size}
    result["matches_pin"] = (digest == row["archive_sha256"] and size == row["archive_bytes"])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pins", type=Path, default=PINS)
    parser.add_argument("--archives", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--fetch", action="store_true", help="explicitly fetch all archives from codeload")
    args = parser.parse_args()
    args.out.unlink(missing_ok=True)
    pins = json.loads(args.pins.read_text())
    rows = pins["subjects"] + pins["providers"]
    args.archives.mkdir(parents=True, exist_ok=True)
    results = []
    for row in rows:
        path = args.archives / archive_identity(row)
        if args.fetch:
            url = f"https://codeload.github.com/{row['repo']}/tar.gz/{row['sha']}"
            partial = path.with_suffix(".part")
            try:
                with urllib.request.urlopen(url, timeout=120) as source, partial.open("wb") as target:
                    size = 0
                    while block := source.read(1024 * 1024):
                        size += len(block)
                        if size > MAX_BYTES:
                            raise ValueError("download exceeds byte cap")
                        target.write(block)
                partial.replace(path)
            finally:
                partial.unlink(missing_ok=True)
        results.append(verify(row, path))
    receipt = {"pins_sha256": hashlib.sha256(args.pins.read_bytes()).hexdigest(),
               "archives": results, "checked": len(results),
               "matched": sum(row["matches_pin"] for row in results)}
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"checked": receipt["checked"], "matched": receipt["matched"]}))
    return 0 if receipt["checked"] == receipt["matched"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
