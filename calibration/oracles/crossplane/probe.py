#!/usr/bin/env python3
"""Bounded, offline Crossplane schema calibration. No tier-A extraction claims."""

import argparse
import copy
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time


MAX_FILES = 1000
MAX_FILE = 2 * 1024 * 1024
MAX_TOTAL = 32 * 1024 * 1024
MAX_PROVIDER = 10000
MAX_BASES = 512
MAX_PATCHES = 512
MAX_CALLS = 4096
MAX_OUTPUT = 16 * 1024 * 1024


class Refusal(Exception):
    pass


def obj(value):
    return value if isinstance(value, dict) else {}


def seq(value):
    return value if isinstance(value, list) else []


def at(value, *keys):
    for key in keys:
        value = obj(value).get(key)
    return value


def digest(path):
    h = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def call(argv, scratch, cap=MAX_OUTPUT, timeout=30, allow_semantic_one=False, with_code=False):
    """Capture to bounded local files. Never put tool stderr in the receipt."""
    scratch = Path(scratch)
    scratch.mkdir(parents=True, exist_ok=True)
    env = {"PATH": "/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin",
           "TMPDIR": str(scratch), "HOME": str(scratch), "KUBECONFIG": "/dev/null",
           "HTTP_PROXY": "http://127.0.0.1:9", "HTTPS_PROXY": "http://127.0.0.1:9",
           "ALL_PROXY": "http://127.0.0.1:9", "NO_PROXY": ""}
    with tempfile.TemporaryFile(dir=str(scratch)) as stdout, tempfile.TemporaryFile(dir=str(scratch)) as stderr:
        try:
            p = subprocess.Popen([str(x) for x in argv], stdout=stdout, stderr=stderr,
                                 env=env, start_new_session=True)
        except OSError as exc:
            raise Refusal("tool execution failed") from exc
        deadline = time.monotonic() + timeout
        try:
            while p.poll() is None:
                if os.fstat(stdout.fileno()).st_size > cap or os.fstat(stderr.fileno()).st_size > cap:
                    raise Refusal("tool output cap exceeded")
                if time.monotonic() >= deadline:
                    raise Refusal("tool timed out")
                time.sleep(0.01)
        except Refusal:
            try:
                os.killpg(p.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            p.wait()
            raise
        if os.fstat(stdout.fileno()).st_size > cap or os.fstat(stderr.fileno()).st_size > cap:
            raise Refusal("tool output cap exceeded")
        if p.returncode != 0 and not (allow_semantic_one and p.returncode == 1):
            raise Refusal("tool exit refused")
        stdout.seek(0)
        result = stdout.read(cap + 1)
        if len(result) > cap:
            raise Refusal("tool output cap exceeded")
        return (result, p.returncode) if with_code else result


def docs(yq, file, scratch):
    raw = call([yq, "-o=json", "-I=0", ".", file], scratch)
    if not raw.strip():
        raise Refusal("empty parser output")
    result = []
    for line in raw.splitlines():
        try:
            value = json.loads(line)
        except (ValueError, UnicodeError) as exc:
            raise Refusal("invalid parser output") from exc
        if value is not None:
            if not isinstance(value, dict):
                raise Refusal("non-object YAML document")
            result.append(value)
    return result


def report_counts(raw, expected, exitcode=0):
    if len(set(expected)) != len(expected):
        raise Refusal("duplicate expected resource identity")
    try:
        report = json.loads(raw)
    except (ValueError, UnicodeError) as exc:
        raise Refusal("missing or invalid native report") from exc
    rows = obj(report).get("resources")
    summary = obj(report).get("summary")
    if not isinstance(rows, list) or not rows:
        raise Refusal("missing native report resources")
    if not isinstance(summary, dict):
        raise Refusal("missing native report summary")
    counts = {"valid": 0, "invalid": 0, "missingSchemas": 0, "extraRows": 0}
    seen = set()
    for row in rows:
        row = obj(row)
        key = (row.get("apiVersion"), row.get("kind"), row.get("name"))
        if not all(isinstance(part, str) for part in key) or not key[0] or not key[1]:
            raise Refusal("invalid or duplicate native report identity")
        status = row.get("status")
        if key in seen and not (key[2] == "" and status == "missingSchema"):
            raise Refusal("invalid or duplicate native report identity")
        seen.add(key)
        if status == "missingSchema":
            counts["missingSchemas"] += 1
        elif status in ("valid", "invalid"):
            counts[status] += 1
        else:
            raise Refusal("unknown native report status")
    if not set(expected).issubset(seen) or len(rows) != sum(counts[k] for k in ("valid", "invalid", "missingSchemas")):
        raise Refusal("native report identity mismatch")
    counts["extraRows"] = len(rows) - len(expected)
    for key, value in {"total": len(rows), **{k: counts[k] for k in ("valid", "invalid", "missingSchemas")}}.items():
        if type(summary.get(key)) is not int or summary[key] != value:
            raise Refusal("native report summary mismatch")
    if exitcode == 0 and counts["invalid"] or exitcode == 1 and not (counts["invalid"] or counts["missingSchemas"]):
        raise Refusal("native report exit mismatch")
    return counts


class SchemaIndex:
    def __init__(self):
        self.entries = {}

    def add(self, crd, source):
        spec = obj(obj(crd).get("spec"))
        group, kind = spec.get("group"), at(spec, "names", "kind")
        if not isinstance(group, str) or not isinstance(kind, str):
            return
        for version in seq(spec.get("versions")):
            version = obj(version)
            if version.get("served") is False:
                continue
            name = version.get("name")
            schema = at(version, "schema", "openAPIV3Schema")
            if isinstance(name, str) and isinstance(schema, dict):
                self.entries.setdefault((group + "/" + name, kind), []).append((source, schema))

    def match(self, api_version, kind):
        if not isinstance(api_version, str) or not isinstance(kind, str):
            return "missing", None, None
        matches = self.entries.get((api_version, kind), [])
        if len(matches) == 1:
            return "found", matches[0][1], matches[0][0]
        return ("ambiguous" if matches else "missing"), None, None


def field_verdict(schema, field_path):
    if not isinstance(schema, dict) or not isinstance(field_path, str) or not field_path:
        return "unmodeled"
    if field_path.startswith("metadata.") or field_path == "metadata":
        return "unmodeled"
    node = schema
    for part in field_path.split("."):
        if not part or "[" in part or "]" in part:
            return "unmodeled"
        properties = obj(node).get("properties")
        if not isinstance(properties, dict):
            return "unmodeled" if obj(node).get("additionalProperties") or obj(node).get("x-kubernetes-preserve-unknown-fields") is True else "notDeclared"
        if part not in properties:
            return "unmodeled" if obj(node).get("additionalProperties") or obj(node).get("x-kubernetes-preserve-unknown-fields") is True else "notDeclared"
        node = properties[part]
    return "declared"


def safe_files(root, scopes=None, limit=MAX_FILES):
    root = Path(root)
    if not root.is_dir() or root.is_symlink():
        raise Refusal("invalid source root")
    result = []
    for base, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if not (Path(base) / d).is_symlink() and d != ".git")
        for name in sorted(names):
            path = Path(base) / name
            if path.is_symlink() or not path.is_file() or path.suffix.lower() not in (".yaml", ".yml"):
                continue
            rel = path.relative_to(root).as_posix()
            if scopes is not None and not any(fnmatch.fnmatchcase(rel, pattern) for pattern in scopes):
                continue
            result.append(path)
            if len(result) > limit:
                raise Refusal("file count cap exceeded")
    return result


def checked_size(path):
    size = path.stat().st_size
    if size > MAX_FILE:
        raise Refusal("file size cap exceeded")
    return size


def tree_digest(root, files):
    h = hashlib.sha256()
    for file in files:
        h.update(file.relative_to(root).as_posix().encode() + b"\0")
        h.update(bytes.fromhex(digest(file)))
    return h.hexdigest()


def git_identity(root, expected_sha, scratch):
    top = call(["git", "-C", root, "rev-parse", "--show-toplevel"], scratch).decode().strip()
    if Path(top).resolve() != Path(root).resolve():
        raise Refusal("subject root mismatch")
    head = call(["git", "-C", root, "rev-parse", "HEAD"], scratch).decode().strip()
    if head != expected_sha:
        raise Refusal("subject revision mismatch")
    if call(["git", "-C", root, "status", "--porcelain"], scratch).strip():
        raise Refusal("subject working tree is dirty")


def identity(document, allow_unnamed=False):
    api, kind, name = document.get("apiVersion"), document.get("kind"), at(document, "metadata", "name")
    if not all(isinstance(part, str) and part for part in (api, kind)):
        raise Refusal("native resource lacks identity")
    if allow_unnamed and name in (None, ""):
        name = ""
    elif not isinstance(name, str) or not name:
        raise Refusal("native resource lacks identity")
    return api, kind, name


def add_counts(target, source):
    for key, value in source.items():
        target[key] = target.get(key, 0) + value


def run(manifest_path, out_path, scratch, crossplane, yq):
    out_path = Path(out_path)
    out_path.unlink(missing_ok=True)
    manifest = json.loads(Path(manifest_path).read_text())
    subjects, providers = manifest.get("subjects"), manifest.get("provider_roots")
    controls = manifest.get("control_resources", [])
    core = Path(manifest.get("core_crds", ""))
    if not isinstance(subjects, list) or not isinstance(providers, list) or not isinstance(controls, list) or not core.is_dir():
        raise Refusal("invalid manifest")
    scratch = Path(scratch)
    scratch.mkdir(parents=True, exist_ok=True)
    crossplane, yq = Path(crossplane), Path(yq)
    if not all(p.is_file() and not p.is_symlink() for p in (crossplane, yq)):
        raise Refusal("tool unavailable")
    tool_pins = manifest.get("tool_sha256")
    if not isinstance(tool_pins, dict) or any(tool_pins.get(name) != digest(path)
                                                for name, path in (("crossplane", crossplane), ("yq", yq))):
        raise Refusal("tool hash mismatch")
    core_hash = hashlib.sha256()
    core_files = safe_files(core, limit=MAX_PROVIDER)
    for file in core_files:
        checked_size(file)
        core_hash.update(file.relative_to(core).as_posix().encode() + b"\0")
        core_hash.update(bytes.fromhex(digest(file)))
    core_digest = core_hash.hexdigest()
    subject_snapshots = []
    preflight_files = preflight_bytes = 0
    for subject in subjects:
        if not isinstance(subject, dict) or not all(isinstance(subject.get(k), str) and subject[k] for k in ("id", "root", "sha")):
            raise Refusal("invalid subject pin")
        scopes = subject.get("scope")
        if not isinstance(scopes, list) or not scopes or not all(isinstance(s, str) and s for s in scopes):
            raise Refusal("invalid scope")
        git_identity(subject["root"], subject["sha"], scratch)
        files = safe_files(subject["root"], scopes)
        preflight_files += len(files)
        if preflight_files > MAX_FILES:
            raise Refusal("subject file cap exceeded")
        for file in files:
            preflight_bytes += checked_size(file)
            if preflight_bytes > MAX_TOTAL:
                raise Refusal("subject byte cap exceeded")
        subject_snapshots.append((subject, len(files), tree_digest(Path(subject["root"]), files)))
    index = SchemaIndex()
    provider_count = 0
    provider_by_id = {}
    scanned_crd_hash = hashlib.sha256()
    provider_snapshots = []
    provider_evidence = []
    for provider in providers:
        if not isinstance(provider, dict) or not all(isinstance(provider.get(k), str) and provider[k] for k in ("id", "root", "sha")):
            raise Refusal("invalid provider pin")
        if provider["id"] in provider_by_id:
            raise Refusal("duplicate provider id")
        provider_root = Path(provider["root"])
        if provider_root.is_symlink() or (provider_root / "package").is_symlink():
            raise Refusal("symlinked provider root")
        provider_by_id[provider["id"]] = provider_root
        crd_root = provider_root / "package" / "crds"
        files = safe_files(crd_root, limit=MAX_PROVIDER)
        provider_count += len(files)
        if provider_count > MAX_PROVIDER:
            raise Refusal("provider cap exceeded")
        for file in files:
            checked_size(file)
        actual = tree_digest(crd_root, files)
        if provider.get("crd_sha256") != actual:
            raise Refusal("provider CRD digest mismatch")
        provider_snapshots.append((crd_root, len(files), actual))
        provider_evidence.append({"id": provider["id"], "sha": provider["sha"],
                                  "crdSha256": actual, "crdFiles": len(files)})
        for file in files:
            checked_size(file)
            for document in docs(yq, file, scratch):
                if document.get("kind") == "CustomResourceDefinition":
                    index.add(document, str(file))
    total_bytes = total_files = bases = patches = calls = 0
    totals = {}
    subject_rows = []
    exclusions = {"nonCrossplaneFiles": 0, "mixedFiles": 0, "nonCompositionDocuments": 0,
                  "basesWithoutSchema": 0, "basesAmbiguousSchema": 0, "baseWithoutName": 0,
                  "unsupportedPatches": 0, "patchSetDefinitions": 0}
    control_counts = {"examples": 0, "nativeCalls": 0, "originalValid": 0, "originalInvalid": 0,
                      "originalMissingSchemas": 0, "mutationInvalid": 0, "mutationOther": 0}
    control_sources = []
    control_snapshots = []
    raw_truth_hash = hashlib.sha256()
    used_crd_hashes = set()

    def native(crd, file, expected):
        nonlocal calls
        calls += 1
        if calls > MAX_CALLS:
            raise Refusal("native call cap exceeded")
        raw, code = call([crossplane, "resource", "validate", "--crossplane-image=", "--cache-dir",
                          scratch / "cache", "-o", "json", crd, file], scratch,
                         allow_semantic_one=True, with_code=True)
        counts = report_counts(raw, expected, code)
        canonical = json.dumps(json.loads(raw), sort_keys=True, separators=(",", ":")).encode()
        raw_truth_hash.update(len(canonical).to_bytes(8, "big"))
        raw_truth_hash.update(canonical)
        raw_truth_hash.update(bytes([code]))
        raw_dir = scratch / "raw"
        raw_dir.mkdir(exist_ok=True)
        (raw_dir / ("native-%04d.json" % calls)).write_bytes(raw)
        if Path(crd).is_file():
            used_crd_hashes.add(digest(crd))
        return counts

    for control in controls:
        if not isinstance(control, dict) or not isinstance(control.get("provider_id"), str) or not isinstance(control.get("path"), str):
            raise Refusal("invalid control")
        if not isinstance(control.get("source_sha256"), str) or len(control["source_sha256"]) != 64:
            raise Refusal("missing control source hash")
        document_index = control.get("document_index", 0)
        if type(document_index) is not int or document_index < 0:
            raise Refusal("invalid control document index")
        root = provider_by_id.get(control["provider_id"])
        if root is None:
            raise Refusal("unknown control provider")
        relative = Path(control["path"])
        if relative.is_absolute() or ".." in relative.parts or not relative.parts:
            raise Refusal("control path outside provider")
        file = root / relative
        if file.is_symlink() or not file.is_file() or file.suffix.lower() not in (".yaml", ".yml"):
            raise Refusal("invalid control file")
        if any((root / parent).is_symlink() for parent in relative.parents if parent != Path(".")):
            raise Refusal("symlinked control path")
        checked_size(file)
        if digest(file) != control["source_sha256"]:
            raise Refusal("control source hash mismatch")
        control_snapshots.append((file, control["source_sha256"]))
        control_sources.append(control["source_sha256"])
        documents = docs(yq, file, scratch)
        if document_index >= len(documents):
            raise Refusal("control document index out of range")
        original = documents[document_index]
        status, _, crd_file = index.match(original.get("apiVersion"), original.get("kind"))
        if status != "found":
            raise Refusal("control schema missing or ambiguous")
        expected = [identity(original)]
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", dir=str(scratch), delete=False) as original_file:
            json.dump(original, original_file)
            original_path = Path(original_file.name)
        try:
            original_counts = native(crd_file, original_path, expected)
        finally:
            original_path.unlink()
        control_counts["examples"] += 1
        control_counts["nativeCalls"] += 1
        for key, status_key in (("originalValid", "valid"), ("originalInvalid", "invalid"),
                                ("originalMissingSchemas", "missingSchemas")):
            control_counts[key] += original_counts[status_key]
        changed = copy.deepcopy(original)
        if not isinstance(changed.get("spec"), dict) or not isinstance(changed["spec"].get("forProvider"), dict):
            raise Refusal("control lacks spec.forProvider object")
        changed["spec"]["forProvider"]["iacProbeUnknownField"] = True
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", dir=str(scratch), delete=False) as control_file:
            json.dump(changed, control_file)
            control_path = Path(control_file.name)
        try:
            mutation_counts = native(crd_file, control_path, expected)
        finally:
            control_path.unlink()
        control_counts["mutationInvalid"] += mutation_counts["invalid"]
        control_counts["mutationOther"] += mutation_counts["valid"] + mutation_counts["missingSchemas"]
        control_counts["nativeCalls"] += 1

    for subject in subjects:
        if not all(isinstance(subject.get(k), str) and subject[k] for k in ("id", "root", "sha")):
            raise Refusal("invalid subject pin")
        scopes = subject.get("scope")
        if not isinstance(scopes, list) or not scopes or not all(isinstance(s, str) and s for s in scopes):
            raise Refusal("invalid scope")
        files = safe_files(subject["root"], scopes)
        total_files += len(files)
        if total_files > MAX_FILES:
            raise Refusal("subject file cap exceeded")
        subject_counts = {"yamlFiles": len(files), "crossplaneFiles": 0, "compositions": 0,
                          "xrds": 0, "bases": 0, "patches": 0, "nativeCalls": 0,
                          "native": {"valid": 0, "invalid": 0, "missingSchemas": 0, "extraRows": 0},
                          "nativeProviderBases": {"valid": 0, "invalid": 0, "missingSchemas": 0, "extraRows": 0},
                          "fieldPaths": {"declared": 0, "notDeclared": 0, "unmodeled": 0}}
        file_hash = hashlib.sha256()
        parsed = []
        for file in files:
            total_bytes += checked_size(file)
            if total_bytes > MAX_TOTAL:
                raise Refusal("subject byte cap exceeded")
            file_hash.update(file.relative_to(subject["root"]).as_posix().encode() + b"\0")
            file_hash.update(bytes.fromhex(digest(file)))
            documents = docs(yq, file, scratch)
            cross = [d for d in documents if str(d.get("apiVersion", "")).split("/")[0] in
                     ("apiextensions.crossplane.io", "pkg.crossplane.io", "meta.pkg.crossplane.io")]
            parsed.append((file, documents, cross, bool(cross) and len(cross) == len(documents)))
        xrd_index = SchemaIndex()
        for _, _, cross, admitted in parsed:
            if not admitted:
                continue
            for document in cross:
                if document.get("kind") != "CompositeResourceDefinition":
                    continue
                spec = obj(document.get("spec"))
                group, kind = spec.get("group"), at(spec, "names", "kind")
                if isinstance(group, str) and isinstance(kind, str):
                    xrd_index.add({"spec": {"group": group, "names": {"kind": kind},
                                            "versions": spec.get("versions")}}, "xrd")
        for file, documents, cross, admitted in parsed:
            if not cross:
                exclusions["nonCrossplaneFiles"] += 1
                continue
            if not admitted:
                exclusions["mixedFiles"] += 1
                continue
            subject_counts["crossplaneFiles"] += 1
            subject_counts["xrds"] += sum(d.get("kind") == "CompositeResourceDefinition" for d in cross)
            subject_counts["compositions"] += sum(d.get("kind") == "Composition" for d in cross)
            add_counts(subject_counts["native"], native(core, file, [identity(d) for d in cross]))
            subject_counts["nativeCalls"] += 1
            for document in cross:
                kind = document.get("kind")
                if kind == "CompositeResourceDefinition":
                    continue
                if kind != "Composition":
                    exclusions["nonCompositionDocuments"] += 1
                    continue
                spec = obj(document.get("spec"))
                exclusions["patchSetDefinitions"] += len(seq(spec.get("patchSets")))
                xr_api = at(spec, "compositeTypeRef", "apiVersion")
                xr_kind = at(spec, "compositeTypeRef", "kind")
                xr_status, xr_schema, _ = xrd_index.match(xr_api, xr_kind)
                resources = list(seq(spec.get("resources")))
                for step in seq(spec.get("pipeline")):
                    resources.extend(seq(at(step, "input", "resources")))
                for resource in resources:
                    resource = obj(resource)
                    base = obj(resource.get("base"))
                    bases += 1
                    subject_counts["bases"] += 1
                    if bases > MAX_BASES:
                        raise Refusal("base cap exceeded")
                    status, schema, crd_file = index.match(base.get("apiVersion"), base.get("kind"))
                    if not at(base, "metadata", "name"):
                        exclusions["baseWithoutName"] += 1
                    if status != "found":
                        exclusions["basesWithoutSchema" if status == "missing" else "basesAmbiguousSchema"] += 1
                    else:
                        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", dir=str(scratch), delete=False) as base_file:
                            json.dump(base, base_file)
                            base_path = Path(base_file.name)
                        try:
                            base_counts = native(crd_file, base_path, [identity(base, allow_unnamed=True)])
                            add_counts(subject_counts["native"], base_counts)
                            add_counts(subject_counts["nativeProviderBases"], base_counts)
                        finally:
                            base_path.unlink()
                        subject_counts["nativeCalls"] += 1
                    for patch in seq(resource.get("patches")):
                        patches += 1
                        subject_counts["patches"] += 1
                        if patches > MAX_PATCHES:
                            raise Refusal("patch cap exceeded")
                        patch = obj(patch)
                        typ = patch.get("type", "FromCompositeFieldPath")
                        if typ in ("FromCompositeFieldPath", "ToCompositeFieldPath"):
                            source_schema = xr_schema if typ == "FromCompositeFieldPath" and xr_status == "found" else schema if typ == "ToCompositeFieldPath" and status == "found" else None
                            verdict = field_verdict(source_schema, patch.get("fromFieldPath"))
                            subject_counts["fieldPaths"][verdict] += 1
                        else:
                            exclusions["unsupportedPatches"] += 1
        subject_rows.append({"id": subject["id"], "sha": subject["sha"],
                             "contentSha256": file_hash.hexdigest(), "counts": subject_counts})
        for key in ("yamlFiles", "crossplaneFiles", "compositions", "xrds", "bases", "patches", "nativeCalls"):
            totals[key] = totals.get(key, 0) + subject_counts[key]
        for key in ("native", "nativeProviderBases", "fieldPaths"):
            add_counts(totals.setdefault(key, {}), subject_counts[key])
    for provider in provider_evidence:
        scanned_crd_hash.update(provider["id"].encode() + b"\0")
        scanned_crd_hash.update(bytes.fromhex(provider["crdSha256"]))
    for root, count, before in provider_snapshots:
        after_files = safe_files(root, limit=MAX_PROVIDER)
        if len(after_files) != count or tree_digest(root, after_files) != before:
            raise Refusal("provider source changed during probe")
    after_core = safe_files(core, limit=MAX_PROVIDER)
    if len(after_core) != len(core_files) or tree_digest(core, after_core) != core_digest:
        raise Refusal("core CRD source changed during probe")
    for subject, count, before in subject_snapshots:
        git_identity(subject["root"], subject["sha"], scratch)
        after_files = safe_files(subject["root"], subject["scope"])
        if len(after_files) != count or tree_digest(Path(subject["root"]), after_files) != before:
            raise Refusal("subject source changed during probe")
    for file, before in control_snapshots:
        if digest(file) != before:
            raise Refusal("control source changed during probe")
    receipt = {"format": "crossplane-schema-probe-v1", "tier": "C", "calibrationOnly": True,
               "toolSha256": {"crossplane": digest(crossplane), "yq": digest(yq)},
               "providerPins": provider_evidence,
               "providerCrdFiles": provider_count,
               "scannedProviderCrdsSha256": scanned_crd_hash.hexdigest(),
               "usedProviderCrdSha256": sorted(used_crd_hashes),
               "rawTruthSha256": raw_truth_hash.hexdigest(),
               "controlSourceSha256": control_sources,
               "coreCrds": {"files": len(core_files), "sha256": core_digest},
               "subjects": subject_rows,
               "aggregate": totals, "controls": control_counts, "exclusions": exclusions,
               "limits": {"subjectFiles": MAX_FILES, "fileBytes": MAX_FILE, "subjectBytes": MAX_TOTAL,
                          "providerFiles": MAX_PROVIDER, "bases": MAX_BASES, "patches": MAX_PATCHES,
                          "nativeCalls": MAX_CALLS, "toolOutputBytes": MAX_OUTPUT}}
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=str(out_path.parent), prefix=".probe-", delete=False) as stream:
        temp = Path(stream.name)
        json.dump(receipt, stream, indent=2, sort_keys=True)
        stream.write("\n")
    os.replace(temp, out_path)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("manifest", "out", "scratch", "crossplane", "yq"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    try:
        run(args.manifest, args.out, args.scratch, args.crossplane, args.yq)
    except (Refusal, OSError, ValueError, KeyError) as exc:
        parser.exit(1, "crossplane probe refused: " + str(exc) + "\n")


if __name__ == "__main__":
    main()
