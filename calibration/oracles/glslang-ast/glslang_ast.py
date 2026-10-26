#!/usr/bin/env python3
"""GLSL L2 oracle from pinned glslang 16.6.0 -i output."""
import argparse
from collections import Counter, defaultdict
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

EXTS = frozenset("glsl vert frag comp geom tesc tese rgen rchit rmiss rahit rint rcall mesh task vsh fsh".split())
STAGES = frozenset("vert frag comp geom tesc tese rgen rchit rmiss rahit rint rcall mesh task".split())
NAME = re.compile(r"^\s*(?:\d+:\d+|\d+)(\s+)Function (Definition|Call):\s+([A-Za-z_]\w*)\(")
LINKED = re.compile(r"^\s*Linked\s+\w+\s+stage:")
LINKER_OBJECTS = re.compile(r"^\s*\d+:\?\s+Linker Objects\b")
LINE_DIRECTIVE = re.compile(r'^\s*#\s*line\s+(\d+)\s+"([^"]+)"')
ANY_LINE_DIRECTIVE = re.compile(r'^\s*#\s*line\b')
INCLUDE_DIRECTIVE = re.compile(r'^\s*#\s*include\b', re.M)
INDEX_SHA256 = "6f6681d1b422c5bb11c54c97fa2550f6c3893b9da1971aecb0a7b59e91bbd458"
SOURCE_SHA256 = "ce264edf9b2417f7f7f6ab42eedf2b4671a81557e0b5ed5e89fd9205bf557540"
SOURCE_REVISION = "747576a437935b79d72cc07b6cbed7136b613e71"
SHADERC_REVISION = "2737994b552137a85545f956e303d620e6ff232c"
SHADERC_INCLUDE_PREAMBLE = "#extension GL_GOOGLE_include_directive : enable\n"
SHADERC_INCLUDE_SOURCE = "https://github.com/google/shaderc/blob/" + SHADERC_REVISION + "/libshaderc_util/src/compiler.cc#L223-L227"
COMPILER_RELEASE_URL = "https://github.com/KhronosGroup/glslang/releases/download/16.6.0/glslang-16.6.0-macos-universal-release.tar.gz"
COMPILER_ARCHIVE_SHA256 = "4f8c05f2888a73ed0b7b274bc524af9130e09ae94b43c0f3f9ed83cfae852834"
COMPILER_BINARY_SHA256 = "18c23a68d6f4a21e822a8dcee212a2a448a66494b8f58aaf1655ef6328543d11"
RECIPES_PATH = Path("workspace/oracles/glslang-ast/subject-recipes/recipes.json")
RECIPES_SUMMARY = Path(__file__).parent / "subject-recipes" / "recipes-summary.json"
CATALOG_MODES = ("baseline-core", "core-plus-types", "full-registry")
DEFAULT_EXTENSION_INDEX = Path(__file__).with_name("registered-extensions.json")
DEFAULT_EXTENSION_REVIEW = Path(__file__).with_name("registered-extensions-review.json")


def compiler_identity(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    actual = digest.hexdigest()
    if actual != COMPILER_BINARY_SHA256:
        raise ValueError(f"compiler binary digest mismatch: {actual}")
    return {"compiler_binary_sha256": actual, "compiler_release_asset_url": COMPILER_RELEASE_URL,
            "compiler_release_archive_sha256": COMPILER_ARCHIVE_SHA256,
            "compiler_platform": "macOS universal release"}


def load_recipe_manifest(path):
    raw = path.read_bytes()
    data = json.loads(raw)
    summary = json.loads(RECIPES_SUMMARY.read_text())
    populations = {name: subject["population"] for name, subject in data["subjects"].items()}
    if summary != {"pins": data["pins"], "populations": populations,
                   "full_recipe_sha256": hashlib.sha256(raw).hexdigest()}:
        raise ValueError("full recipe manifest differs from committed summary")
    return raw, data


def core_constructor_names():
    """Pinned GLSL 4.60 basic type identifiers, including opaque/image types."""
    path = Path(__file__).with_name("glsl460-functions.json")
    builtin_index(path)
    return set(json.loads(path.read_bytes())["types"])


def legacy_constructor_names():
    """Original 44 G2 scalar, vector, and matrix constructor exclusions."""
    names = {"bool", "int", "uint", "float", "double"}
    for prefix in ("bvec", "ivec", "uvec", "vec", "dvec"):
        names.update(f"{prefix}{size}" for size in (2, 3, 4))
    for prefix in ("mat", "dmat"):
        names.update(f"{prefix}{size}" for size in (2, 3, 4))
        names.update(f"{prefix}{columns}x{rows}"
                     for columns in (2, 3, 4) for rows in (2, 3, 4))
    return names


def _catalog_fingerprint(mode, functions, constructors):
    raw = json.dumps({"mode": mode, "functions": sorted(functions),
                      "constructors": sorted(constructors)}, separators=(",", ":")).encode()
    return hashlib.sha256(raw).hexdigest()


def _registered_catalog(path, review_path):
    raw = path.read_bytes()
    data = json.loads(raw)
    review = json.loads(review_path.read_text())
    digest = hashlib.sha256(raw).hexdigest()
    if (review.get("verdict") != "accepted" or
            review.get("extension_catalog_sha256") != digest):
        raise ValueError("full extension catalog lacks an accepted review pin")
    if (data.get("complete") is not True or data.get("sources") != review.get("accepted_extension_sources") or
            data.get("audit") != {"unresolved_candidates": 0, "challenger_gaps": 0}):
        raise ValueError("full extension catalog source or audit proof mismatch")
    values = []
    for field, mapping_field in (("functions", "functions_by_extension"),
                                 ("types", "types_by_extension")):
        names = data.get(field)
        mapping = data.get(mapping_field)
        if (not isinstance(names, list) or names != sorted(set(names)) or
                not all(re.fullmatch(r"[A-Za-z_]\w*", name) for name in names) or
                not isinstance(mapping, dict) or
                set(names) != {name for rows in mapping.values() for name in rows}):
            raise ValueError("invalid full extension catalog " + field)
        values.append(set(names))
    return values[0], values[1], digest


def catalog_for_mode(mode, core_index, extension_index=None, extension_review=None):
    """Resolve one explicit exclusion catalog mode and its receipt metadata."""
    if mode not in CATALOG_MODES:
        raise ValueError("unknown GLSL catalog mode")
    core_functions, core_hash = builtin_index(core_index)
    core_types = set(json.loads(core_index.read_text())["types"])
    if mode == "baseline-core":
        functions, constructors = core_functions, legacy_constructor_names()
        extension_hash = None
    elif mode == "core-plus-types":
        functions, constructors = core_functions, core_types
        extension_hash = None
    else:
        ext_functions, ext_types, extension_hash = _registered_catalog(
            extension_index or DEFAULT_EXTENSION_INDEX,
            extension_review or DEFAULT_EXTENSION_REVIEW)
        functions, constructors = core_functions | ext_functions, core_types | ext_types
    return functions, constructors, {
        "catalog_mode": mode, "catalog_fingerprint_sha256": _catalog_fingerprint(
            mode, functions, constructors), "core_catalog_sha256": core_hash,
        "extension_catalog_sha256": extension_hash,
        "catalog_function_count": len(functions),
        "catalog_constructor_count": len(constructors)}


def files(root):
    return sorted(p for p in root.rglob("*") if p.is_file() and p.suffix.lstrip(".") in EXTS
                  and ".git" not in p.parts and ".asm." not in p.name and ".invalid." not in p.name)


def has_include_directive(source):
    return bool(INCLUDE_DIRECTIVE.search(source))


def invocation(kind, rel, compiler, root, artifact=None, recipe=None, has_include=None):
    """Replay pinned native inputs; glslc inputs are translated to glslang."""
    suffix = Path(rel).suffix.lstrip(".")
    if suffix not in STAGES:
        return None, "fragment_or_unknown_stage"
    cmd = [str(compiler), "-i"]
    if recipe is None:
        raise ValueError("pinned per-file recipe required")
    if kind in ("vulkan", "spirv-cross"):
        cmd += recipe["flags"]
    elif kind == "vulkan-samples":
        if recipe["declaration_status"] != "declared":
            return None, "no_pinned_cmake_declaration"
        cmd += ["-V", "-I" + str(root / "shaders" / "includes" / "glsl")]
        if has_include is None:
            source = root / rel
            has_include = source.exists() and has_include_directive(source.read_text(errors="replace"))
        if recipe.get("native_compiler", "glslc") == "glslc" and has_include:
            # shaderc's native glslc preamble enables this extension before
            # preprocessing; glslang needs its equivalent -P input.
            cmd += ["-P" + SHADERC_INCLUDE_PREAMBLE]
        alternatives = {tuple(d["glslc_additional_arguments"]) for d in recipe["cmake_declarations"]}
        if len(alternatives) != 1:
            return None, "conflicting_pinned_target_flags"
        for flag in next(iter(alternatives)):
            if flag.startswith("--target-spv=spv1."):
                cmd += ["--target-env", "spirv" + flag.removeprefix("--target-spv=spv")]
            elif flag == "-I ..":
                cmd += ["-I" + str(root.parent)]
            else:
                return None, "untranslated_pinned_glslc_flag:" + flag
    else:
        raise ValueError(kind)
    if artifact is not None:
        cmd += ["-o", str(artifact)]
    return cmd + [str(root / rel)], None


def invocation_variants(kind, rel, compiler, root, artifact, recipe, has_include):
    """Enumerate every distinct pinned target input for one source file."""
    if kind != "vulkan-samples" or recipe.get("declaration_status") != "declared":
        command, reason = invocation(kind, rel, compiler, root, artifact, recipe, has_include)
        return [{"command": command, "reason": reason, "cmake_sources": []}]
    grouped = defaultdict(list)
    for item in recipe["cmake_declarations"]:
        grouped[tuple(item["glslc_additional_arguments"])].append(item["cmake"])
    variants = []
    for index, (flags, sources) in enumerate(sorted(grouped.items())):
        one = dict(recipe, cmake_declarations=[{"glslc_additional_arguments": list(flags)}])
        output = artifact.with_name(artifact.stem + f"-v{index}" + artifact.suffix)
        command, reason = invocation(kind, rel, compiler, root, output, one, has_include)
        variants.append({"command": command, "reason": reason, "cmake_sources": sorted(sources)})
    return variants


def parse_ast(output, rel):
    """Return written function call pairs, before any builtin exclusions.

    The text before Linker Objects or the first Linked stage is the parsed region. The
    indentation stack identifies the enclosing Function Definition node.
    """
    lines = output.splitlines()
    stop = next((i for i, line in enumerate(lines) if LINKED.match(line) or LINKER_OBJECTS.match(line)), len(lines))
    lines = lines[:stop]
    stack = []
    definitions, sites = [], []
    for line in lines:
        match = NAME.match(line)
        if not match:
            if "Function Definition:" in line or "Function Call:" in line:
                raise ValueError("unrecognized Function Definition/Call AST line: " + line[:160])
            continue
        indent = len(match.group(1))
        while stack and stack[-1][0] >= indent:
            stack.pop()
        _, kind, name = match.groups()
        if kind == "Definition":
            source_line = int(re.search(r"\d+:(\d+)", line).group(1)) if re.search(r"\d+:(\d+)", line) else 0
            definitions.append({"path": rel, "name": name, "line": source_line, "end": source_line, "kind": "function"})
            stack.append((indent, name))
        else:
            if not stack:
                raise ValueError("Function Call outside Function Definition")
            sites.append((stack[-1][1], name))
    if not definitions:
        raise ValueError("no Function Definition in pre-linked AST")
    return definitions, sites


def definition_origins(preprocessed, source_path, definitions, root=None):
    """Resolve included definitions by compiler #line paths; fail on ambiguity."""
    root = (root or source_path.parent).resolve()
    current_path = source_path.resolve()
    current_line = 1
    candidates = defaultdict(set)
    for text in preprocessed.splitlines():
        directive = LINE_DIRECTIVE.match(text)
        if directive:
            current_line = int(directive.group(1))
            raw = Path(directive.group(2))
            current_path = (raw if raw.is_absolute() else root / raw).resolve()
            continue
        for d in definitions:
            signature = r"^\s*(?:[A-Za-z_]\w*\s+)+" + re.escape(d["name"]) + r"\s*\("
            if d["line"] == current_line and re.search(signature, text):
                candidates[(d["name"], d["line"])].add(current_path)
        current_line += 1
    origins = {}
    for d in definitions:
        found = candidates[(d["name"], d["line"])]
        if len(found) != 1:
            raise ValueError(f"function source origin unresolved: {d['name']}:{d['line']} ({len(found)} candidates)")
        origins[d["name"]] = origins.get(d["name"], set()) | found
    if any(len(paths) != 1 for paths in origins.values()):
        raise ValueError("overloaded function has mixed root/include origins")
    return {name: next(iter(paths)) == source_path.resolve() for name, paths in origins.items()}


def builtin_index(path, source_html=None):
    raw_index = path.read_bytes()
    data = json.loads(raw_index)
    if data.get("spec") != "GLSL 4.60" or data.get("complete") is not True or data.get("source") != "https://registry.khronos.org/OpenGL/specs/gl/GLSLangSpec.4.60.html":
        raise ValueError("requires a complete, provenance-marked GLSL 4.60 builtin index")
    names = data.get("functions")
    if not isinstance(names, list) or len(names) < 197 or not all(re.fullmatch(r"[A-Za-z_]\w*", n) for n in names):
        raise ValueError("invalid builtin index")
    if set(data.get("coverage", {})) != {f"8.{i}" for i in range(1, 20)}:
        raise ValueError("builtin index does not cover GLSL §8.1–8.19")
    if data.get("source_sha256") != SOURCE_SHA256 or data.get("source_revision") != SOURCE_REVISION:
        raise ValueError("builtin index pinned source metadata mismatch")
    types = data.get("types")
    if (not isinstance(types, list) or len(types) != 126 or
            data.get("type_coverage", {}).get("image_type_keyword_count") != 33 or
            not all(re.fullmatch(r"[A-Za-z_]\w*", name) for name in types)):
        raise ValueError("incomplete GLSL 4.60 type index")
    if source_html is not None and hashlib.sha256(source_html.read_bytes()).hexdigest() != SOURCE_SHA256:
        raise ValueError("builtin index source HTML hash mismatch")
    digest = hashlib.sha256(raw_index).hexdigest()
    if digest != INDEX_SHA256:
        raise ValueError("builtin catalog digest mismatch")
    return set(names), digest


def exclusions_for_file(rel, builtins, constructors):
    return ({(rel, name, "selected catalog built-in function") for name in builtins} |
            {(rel, name, "selected catalog type constructor") for name in constructors - builtins})


def oracle_payload(version, _internal_definitions, calls, exclusions, scope, builtin_hash, total_files,
                   subject_kind):
    """L2-only neutral file; ctags-C remains the independent L1 oracle."""
    disclosures = [
        f"glslangValidator {version} -i parsed {len(scope)}/{total_files} GLSL files; see per-file invocation and diagnostics receipt",
        "GLSL exclusion catalog fingerprint sha256=" + builtin_hash,
    ]
    if subject_kind == "vulkan-samples":
        disclosures.append("Vulkan-Samples glslc/per-target inputs are translated to glslang; tier B")
    return {"tool": "glslangValidator -i", "version": version, "tier": "B",
            "definitions": [],
            "calls": [{"path": p, "from": f, "to": t} for p, f, t in sorted(calls)],
            "call_exclusions": [{"path": p, "to": t, "reason": why} for p, t, why in sorted(exclusions)],
            "scope": scope,
            "disclose": disclosures}


def validate_preprocessed_inputs(preprocessed, root, tracked):
    """Reject compiler input paths outside the pinned, tracked shader tree."""
    root = root.resolve()
    shader_root = (root / "shaders").resolve()
    seen = set()
    for line in preprocessed.splitlines():
        if not ANY_LINE_DIRECTIVE.match(line):
            continue
        directive = LINE_DIRECTIVE.match(line)
        if not directive:
            raise ValueError("unrecognized preprocessor #line path: " + line[:160])
        raw = Path(directive.group(2))
        path = (raw if raw.is_absolute() else root / raw).resolve()
        if not path.is_relative_to(shader_root):
            raise ValueError("preprocessed input outside pinned shader tree: " + str(path))
        rel = path.relative_to(root).as_posix()
        if rel not in tracked:
            raise ValueError("untracked preprocessed shader input: " + rel)
        seen.add(rel)
    if not seen:
        raise ValueError("no verifiable preprocessor #line inputs")
    return seen


def execute_variant(path, command, root, raw_dir, stem, builtins, constructors, has_include, tracked):
    detail = {"invocation": command}
    run = subprocess.run(command, cwd=root, capture_output=True, text=True, errors="replace")
    ast_path = raw_dir / (stem + ".ast")
    stdout_path = raw_dir / (stem + ".stdout")
    ast_path.write_text(run.stderr)
    stdout_path.write_text(run.stdout)
    detail.update(exit_code=run.returncode, ast_path=str(ast_path), stdout_path=str(stdout_path),
                  ast_sha256=hashlib.sha256(run.stderr.encode()).hexdigest(),
                  diagnostics=(run.stderr or run.stdout)[:500] if run.returncode else "")
    if any(arg.startswith("-P") for arg in command):
        detail["include_preamble_translation"] = {"from": "glslc implicit GL_GOOGLE_include_directive",
                                                   "to": "glslang -P", "source": SHADERC_INCLUDE_SOURCE}
    if run.returncode:
        detail["status"] = "compile_failed"
        return detail, None
    try:
        ast = run.stderr if "Function Definition:" in run.stderr else run.stdout
        defs, sites = parse_ast(ast, path.relative_to(root).as_posix())
        shadow = {d["name"] for d in defs} & (builtins | constructors)
        if shadow:
            raise ValueError("builtin name shadowed by user definition: " + ", ".join(sorted(shadow)))
        if has_include:
            include_args = [arg for arg in command if arg.startswith("-I")]
            preamble = [arg for arg in command if arg.startswith("-P")]
            precommand = [command[0], "-E", *include_args, *preamble, str(path)]
            pre = subprocess.run(precommand, cwd=root, capture_output=True, text=True, errors="replace")
            pre_path = raw_dir / (stem + ".preprocessed")
            pre_path.write_text(pre.stdout)
            detail.update(preprocess_invocation=precommand, preprocess_exit_code=pre.returncode,
                          preprocess_path=str(pre_path), preprocess_diagnostics=pre.stderr[:500])
            if pre.returncode:
                raise ValueError("preprocessing failed for include origin mapping")
            detail["verified_preprocessed_inputs"] = sorted(validate_preprocessed_inputs(pre.stdout, root, tracked))
            local = definition_origins(pre.stdout, path, defs, root)
        else:
            local = {d["name"]: True for d in defs}
    except ValueError as exc:
        detail.update(status="parse_failed", diagnostic=str(exc))
        return detail, None
    detail.update(status="parsed", definitions=len(defs), included_definitions=sum(not x for x in local.values()),
                  call_sites=len(sites))
    return detail, (defs, sites, local)


def validated_variant_calls(rel, details, results, builtins, constructors):
    """Union calls only when every native target declaration parsed."""
    if len(details) != len(results) or any(d["status"] != "parsed" for d in details):
        raise ValueError("not every native target input parsed")
    return {(rel, caller, callee) for _, sites, local in results for caller, callee in sites
            if local[caller] and callee not in builtins and callee not in constructors}


def subject_identity(root, expected_sha):
    """Fail before compilation if the pinned checkout drifted; hash shader inputs."""
    env = os.environ.copy()
    env["GIT_OPTIONAL_LOCKS"] = "0"
    head = subprocess.run(["git", "-C", str(root), "rev-parse", "HEAD"],
                          capture_output=True, text=True, env=env, check=True).stdout.strip()
    if head != expected_sha:
        raise ValueError(f"subject pin mismatch: {head} != {expected_sha}")
    status = subprocess.run(["git", "-C", str(root), "status", "--porcelain", "--untracked-files=no"],
                            capture_output=True, text=True, env=env, check=True).stdout
    if status:
        raise ValueError("tracked subject files are dirty: " + status[:500])
    untracked = subprocess.run(["git", "-C", str(root), "ls-files", "--others", "--", "shaders"],
                               capture_output=True, text=True, env=env, check=True).stdout
    if untracked:
        raise ValueError("untracked shader input paths: " + untracked[:500])
    tree = root / "shaders"
    inputs = sorted(p for p in tree.rglob("*") if p.is_file() or p.is_symlink())
    digest = hashlib.sha256()
    for path in inputs:
        if path.is_symlink():
            raise ValueError("symlink in shader input tree: " + str(path))
        relative = path.relative_to(root).as_posix().encode()
        digest.update(len(relative).to_bytes(4, "big"))
        digest.update(relative)
        digest.update(hashlib.sha256(path.read_bytes()).digest())
    return {"subject_sha": head, "tracked_clean": True, "shader_tree_sha256": digest.hexdigest(),
            "shader_tree_file_count": len(inputs), "shader_tree_scope": "all present files under shaders/ (includes included headers)"}


def tracked_shader_paths(root):
    env = os.environ.copy()
    env["GIT_OPTIONAL_LOCKS"] = "0"
    raw = subprocess.run(["git", "-C", str(root), "ls-files", "-z", "--cached", "--", "shaders"],
                         capture_output=True, env=env, check=True).stdout
    return {os.fsdecode(p) for p in raw.split(b"\0") if p}


def publish_oracle(path, payload, parsed, total):
    """Never leave a usable oracle file when this subject misses 95% scope."""
    path.unlink(missing_ok=True)
    if total == 0 or parsed / total < 0.95:
        raise ValueError("oracle coverage below 95%; no grade is authorized")
    path.write_text(json.dumps(payload, indent=2) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--subject-kind", required=True, choices=["vulkan", "spirv-cross", "vulkan-samples"])
    ap.add_argument("--compiler", required=True)
    ap.add_argument("--builtin-index", required=True, type=Path)
    ap.add_argument("--catalog-mode", choices=CATALOG_MODES, default="core-plus-types")
    ap.add_argument("--extension-index", type=Path, default=DEFAULT_EXTENSION_INDEX)
    ap.add_argument("--extension-review", type=Path, default=DEFAULT_EXTENSION_REVIEW)
    ap.add_argument("--builtin-source", type=Path, help="optional pinned GLSL 4.60 HTML for source-hash verification")
    ap.add_argument("--recipes", type=Path, default=RECIPES_PATH)
    ap.add_argument("root", type=Path)
    ap.add_argument("out", type=Path)
    ap.add_argument("receipt", type=Path)
    args = ap.parse_args()
    args.out.unlink(missing_ok=True)
    root = args.root.resolve()
    compiler = Path(args.compiler).resolve()
    compiler_pin = compiler_identity(compiler)
    artifact_dir = args.receipt.parent.resolve() / "artifacts"
    raw_dir = args.receipt.parent.resolve() / "raw-ast"
    artifact_dir.mkdir(parents=True, exist_ok=True)
    raw_dir.mkdir(parents=True, exist_ok=True)
    if args.builtin_source is not None:
        builtin_index(args.builtin_index, args.builtin_source)
    builtins, constructors, catalog_metadata = catalog_for_mode(
        args.catalog_mode, args.builtin_index, args.extension_index, args.extension_review)
    builtin_hash = catalog_metadata["catalog_fingerprint_sha256"]
    recipe_bytes, recipe_manifest = load_recipe_manifest(args.recipes)
    subject = recipe_manifest["subjects"][args.subject_kind]
    identity = subject_identity(root, recipe_manifest["pins"][args.subject_kind])
    tracked = tracked_shader_paths(root)
    recipes = {r["path"]: r for r in subject["files"]}
    discovered = files(root)
    if len(discovered) != subject["population"] or {p.relative_to(root).as_posix() for p in discovered} != set(recipes):
        raise SystemExit("pinned recipe population differs from discovered corpus")
    version_run = subprocess.run([str(compiler), "--version"], capture_output=True, text=True)
    version = version_run.stdout + version_run.stderr
    if version_run.returncode or "16.6.0" not in version:
        raise SystemExit("compiler must report glslang 16.6.0")
    rows, definitions, calls, exclusions, scope = [], [], set(), set(), []
    for index, path in enumerate(discovered, 1):
        rel = path.relative_to(root).as_posix()
        stem = hashlib.sha256(rel.encode()).hexdigest()
        artifact = artifact_dir / (stem + ".spv")
        has_include = has_include_directive(path.read_text(errors="replace"))
        variants = invocation_variants(args.subject_kind, rel, compiler, root, artifact, recipes[rel], has_include)
        row = {"path": rel, "status": "pending", "variants": []}
        rows.append(row)
        results = []
        for variant_index, variant in enumerate(variants):
            if variant["reason"]:
                detail, parsed = {"status": variant["reason"], "invocation": variant["command"]}, None
            else:
                detail, parsed = execute_variant(path, variant["command"], root, raw_dir, stem + f"-v{variant_index}",
                                                 builtins, constructors, has_include, tracked)
            detail["cmake_sources"] = variant["cmake_sources"]
            row["variants"].append(detail)
            if parsed is not None:
                results.append(parsed)
        failures = [v["status"] for v in row["variants"] if v["status"] != "parsed"]
        if failures:
            row["status"] = failures[0] if len(set(failures)) == 1 else "variant_failed"
            row["failure_classes"] = failures
            continue
        row["status"] = "parsed"
        row.update(variant_count=len(results), definitions=max(len(defs) for defs, _, _ in results),
                   call_sites=sum(len(sites) for _, sites, _ in results))
        scope.append(rel)
        # indexbench exclusions are per file and callee name, so include the
        # full catalog even when glslang lowers a call to an AST operation.
        exclusions.update(exclusions_for_file(rel, builtins, constructors))
        for defs, sites, local in results:
            definitions.extend(defs)
        calls.update(validated_variant_calls(rel, row["variants"], results, builtins, constructors))
        if index % 50 == 0:
            print(f"{args.subject_kind}: {index}/{len(discovered)}", file=sys.stderr, flush=True)
    counts = dict(Counter(r["status"] for r in rows))
    receipt = {"subject_kind": args.subject_kind, "compiler": str(compiler), "version": version.strip(),
               **compiler_pin,
               **catalog_metadata, "builtin_index_sha256": builtin_hash,
               "population": len(rows), "counts": counts,
               "coverage_fraction": len(scope) / len(rows), "recipe_manifest_sha256": hashlib.sha256(recipe_bytes).hexdigest(),
               **identity, "files": rows}
    args.receipt.write_text(json.dumps(receipt, indent=2) + "\n")
    oracle = oracle_payload("16.6.0", definitions, calls, exclusions, scope, builtin_hash, len(rows),
                            args.subject_kind)
    print(f"files={len(rows)} parsed={len(scope)} status={counts}", file=sys.stderr)
    try:
        publish_oracle(args.out, oracle, len(scope), len(rows))
    except ValueError as exc:
        raise SystemExit(str(exc))


if __name__ == "__main__":
    main()
