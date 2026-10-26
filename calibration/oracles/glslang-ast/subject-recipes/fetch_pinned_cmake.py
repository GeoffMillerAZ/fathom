#!/usr/bin/env python3
"""Fetch only pinned Vulkan-Samples sample CMake texts into ignored workspace."""
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
import subprocess
from urllib.request import urlopen

PIN = "69622af06f7652c7fc9b9a9180e7142046315cbc"
ROOT = Path("workspace/calibration/glsl/vulkan-samples")
DEST = Path("workspace/oracles/glslang-ast/subject-recipes/source-cache")
MAX_BYTES = 32768


def fetch(rel):
    url = f"https://raw.githubusercontent.com/KhronosGroup/Vulkan-Samples/{PIN}/{rel}"
    with urlopen(url, timeout=20) as response:
        data = response.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise ValueError(f"source too large: {rel}")
    out = DEST / rel
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(data)
    return rel, len(data)


def main():
    head = subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip()
    if head != PIN:
        raise SystemExit(f"subject pin mismatch: {head} != {PIN}")
    paths = subprocess.check_output(["git", "-C", str(ROOT), "ls-tree", "-r", "--name-only", "HEAD"], text=True).splitlines()
    cmake = [p for p in paths if p.startswith("samples/") and p.endswith("/CMakeLists.txt")]
    errors = []
    with ThreadPoolExecutor(max_workers=6) as pool:
        for future in as_completed(pool.submit(fetch, p) for p in cmake):
            try:
                future.result()
            except Exception as exc:
                errors.append(str(exc))
    print(f"requested={len(cmake)} downloaded={len(cmake)-len(errors)} errors={len(errors)}")
    for error in errors:
        print(error)
    if errors:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
