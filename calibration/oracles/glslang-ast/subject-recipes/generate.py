#!/usr/bin/env python3
"""Generate pinned native GLSL shader input recipes; no compiler is run."""
import json
import hashlib
import re
import subprocess
from collections import defaultdict
from pathlib import Path

BASE = Path("workspace/calibration/glsl")
CACHE_ROOT = Path("workspace/oracles/glslang-ast/subject-recipes/source-cache")
CACHE = CACHE_ROOT / "samples"
OUTPUT = Path("workspace/oracles/glslang-ast/subject-recipes/recipes.json")
SUMMARY = Path(__file__).with_name("recipes-summary.json")
PINS = {"vulkan": "1cbc8d87b731b438f47dea00a2e083ba7d59051d",
        "spirv-cross": "f11ba9f0b21ba8fc15153d50a2a1ae31ab1cf8f7",
        "vulkan-samples": "69622af06f7652c7fc9b9a9180e7142046315cbc"}
EXTS = set("glsl vert frag comp geom tesc tese rgen rchit rmiss rahit rint rcall mesh task vsh fsh".split())
STAGES = EXTS - {"glsl", "vsh", "fsh"}
RAY = set("rgen rchit rmiss rahit rint".split())
LABEL = re.compile(r"^\s*(?:[A-Z][A-Z_]+\b|\))")
QUOTED = re.compile(r'"([^"]+)"')


def files(root):
    return sorted(p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()
                  and p.suffix.lstrip(".") in EXTS and ".asm." not in p.name and ".invalid." not in p.name)


def blocks(lines, label):
    for i, line in enumerate(lines):
        if re.match(r"^\s*" + label + r"\s*$", line):
            block = []
            for item in lines[i + 1:]:
                if LABEL.match(item):
                    break
                if not item.lstrip().startswith("#"):
                    block.extend(QUOTED.findall(item))
            yield block


def sample_declarations():
    listed = subprocess.check_output(["git", "-C", str(BASE / "vulkan-samples"),
                                      "ls-tree", "-r", PINS["vulkan-samples"], "--", "samples"], text=True).splitlines()
    expected = {}
    for line in listed:
        matched = re.fullmatch(r"100644 blob ([0-9a-f]{40})\t(samples/(?:.+/)?CMakeLists\.txt)", line)
        if matched:
            expected[matched.group(2)] = matched.group(1)
    cached = {p.relative_to(CACHE_ROOT).as_posix() for p in CACHE.rglob("CMakeLists.txt")}
    if cached != set(expected):
        raise ValueError(f"pinned CMake cache mismatch: missing={len(set(expected)-cached)} extra={len(cached-set(expected))}")
    index = defaultdict(list)
    for cmake in sorted(CACHE.rglob("CMakeLists.txt")):
        raw = cmake.read_bytes()
        source = cmake.relative_to(CACHE_ROOT).as_posix()
        actual_blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        if actual_blob != expected[source]:
            raise ValueError(f"pinned CMake blob mismatch: {source}")
        lines = raw.decode("utf-8").splitlines()
        shader_blocks = list(blocks(lines, "SHADER_FILES_GLSL"))
        flag_blocks = list(blocks(lines, "GLSLC_ADDITIONAL_ARGUMENTS"))
        if len(shader_blocks) > 1 or len(flag_blocks) > 1:
            raise ValueError(f"multi-block CMake requires manual parser: {cmake}")
        flags = flag_blocks[0] if flag_blocks else []
        for rel in shader_blocks[0] if shader_blocks else []:
            if rel.startswith("${FOLDER_NAME}/"):
                rel = cmake.parent.name + rel[len("${FOLDER_NAME}"):]
            if "${" in rel:
                raise ValueError(f"unresolved CMake variable: {source}: {rel}")
            index[rel].append({"cmake": source, "glslc_additional_arguments": flags})
    return index


def build():
    for name, pin in PINS.items():
        head = subprocess.check_output(["git", "-C", str(BASE / name), "rev-parse", "HEAD"], text=True).strip()
        if head != pin:
            raise ValueError(f"{name} subject pin mismatch: {head} != {pin}")
    vulkan = []
    for rel in files(BASE / "vulkan" / "shaders" / "glsl"):
        suffix = Path(rel).suffix.lstrip(".")
        flags = ["-V"]
        if suffix in RAY or (Path(rel).parent.name == "rayquery" and suffix == "frag"):
            flags += ["--target-env", "vulkan1.2"]
        if suffix in {"mesh", "task"} or "heap" in rel:
            flags += ["--target-env", "spirv1.4"]
        vulkan.append({"path": "shaders/glsl/" + rel, "native_compiler": "glslangValidator", "flags": flags,
                       "input_stage": suffix if suffix in STAGES else None,
                       "source": "shaders/glsl/compileshaders.py"})
    cross = []
    for rel in files(BASE / "spirv-cross" / "shaders"):
        env = "vulkan1.3" if ".spv16." in rel else "spirv1.4" if ".spv14." in rel else "spirv1.5" if rel.endswith(".lib") else "vulkan1.1"
        flags = ["--amb", "--target-env", env, "-V"]
        if ".g." in rel:
            flags.append("-g")
        if ".gV." in rel:
            flags.append("-gV")
        cross.append({"path": "shaders/" + rel, "native_compiler": "glslangValidator", "flags": flags,
                      "input_stage": Path(rel).suffix.lstrip("."), "source": "test_shaders.py:cross_compile"})
    declarations = sample_declarations()
    samples = []
    for rel in files(BASE / "vulkan-samples" / "shaders"):
        paths = declarations.get(rel, [])
        samples.append({"path": "shaders/" + rel,
                        "native_compiler": "glslang" if rel.startswith("tensor_and_data_graph/") and "base" not in rel else "glslc",
                        "common_flags": ["-I", "${CMAKE_SOURCE_DIR}/shaders/includes/glsl"],
                        "cmake_declarations": paths,
                        "declaration_status": "declared" if paths else "unresolved",
                        "source": "bldsys/cmake/sample_helper.cmake"})
    return {"method": "native pinned source invocations; no compiler run or source transform",
            "pins": PINS,
            "min_oracle_scope_fraction": 0.95,
            "subjects": {"vulkan": {"population": len(vulkan), "files": vulkan},
                         "spirv-cross": {"population": len(cross), "files": cross},
                         "vulkan-samples": {"population": len(samples), "files": samples}}}


def main():
    data = build()
    encoded = (json.dumps(data, indent=2) + "\n").encode()
    summary = json.loads(SUMMARY.read_text())
    populations = {name: value["population"] for name, value in data["subjects"].items()}
    if summary != {"pins": PINS, "populations": populations,
                   "full_recipe_sha256": hashlib.sha256(encoded).hexdigest()}:
        raise ValueError("generated full recipes differ from committed pin/count/digest summary")
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_bytes(encoded)
    for name, value in data["subjects"].items():
        print(name, value["population"])
    sample = data["subjects"]["vulkan-samples"]["files"]
    print("sample declared", sum(x["declaration_status"] == "declared" for x in sample),
          "unresolved", sum(x["declaration_status"] == "unresolved" for x in sample))


if __name__ == "__main__":
    main()
