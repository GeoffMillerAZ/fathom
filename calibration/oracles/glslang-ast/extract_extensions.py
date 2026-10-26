#!/usr/bin/env python3
"""Audit pinned Khronos GLSL extension specs before generating a catalog.

This emits an ignored-workspace candidate ledger. A candidate is not a verified
built-in declaration; unresolved candidates prohibit a complete catalog.
"""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
import re
import tarfile
import xml.etree.ElementTree as ET
from pathlib import Path

GLSL_ARCHIVE_SHA256 = "4ffb1bb817c970670644ea2d19168dc1127a1d5502eb6bbfeb6600859090a23d"
GLSL_REVISION = "0099bf83b0288469917fc2e2972cb0a2b74d6711"
REGISTRY_ARCHIVE_SHA256 = "fa80b86e5c2d91c3088b67a128d4070f2520dac6bb823f0538f4870ec9e50942"
REGISTRY_REVISION = "6af574a14089ccfee87efe230ebcdd8742859813"
PROTOTYPE = re.compile(r"^[ \t]*(?:\|[ \t]*)?(?:(?:highp|mediump|lowp|precise|const)[ \t]+)*([A-Za-z_]\w*)[ \t]+([A-Za-z_]\w*)[ \t]*\(")
PROTOTYPE_POINTER = re.compile(r"^[ \t]*([A-Za-z_]\w*)[ \t]+\*+[ \t]*([A-Za-z_]\w*)[ \t]*\(")
PROSE_DECL = re.compile(r"\b(?:add|introduce|define|declare)\b[^\n.]*?\bbuilt-in\s+functions?\s+([A-Za-z_]\w*)\s*\(", re.I)
MODIFICATION_HEADING = re.compile(r"^\s*(?:(?:Modifications|Additions|Changes)\s+to|Modifications\s+of\s+the\s+additions\s+in)\b", re.I)
SHADER_HEADING = re.compile(r"\b(?:Shading Language|GLSL)\b", re.I)
EXAMPLE_HEADING = re.compile(r"^\s*(?:(?:Usage\s+)?Examples?(?:\s*:\s*[^;]*)?|Pseudo[ -]code\s*:?|Issues|Revision History)\s*$", re.I)
NON_SHADER_HEADING = re.compile(r"^\s*(?:Interactions with|New Procedures and Functions|New Tokens|New Implementation Dependent State)\b", re.I)
BUILTIN_HEADING = re.compile(r"^\s*(?:New|Add)\s+Built[ -]in\s+functions?\s*$", re.I)
SECTION_HEADING = re.compile(r"^\s*(?:Modify|Add|Change|Modifications?)(?:\s+to)?\b.*?\b(?:Section|Chapter)\s+([0-9]+)", re.I)
NUMERIC_SECTION_HEADING = re.compile(r"^\s*(?:Modify|Add|Changes?|Modifications?)\b[^\n]*?\b(?:section|subsection)?\s*([0-9]+)\.[0-9]+", re.I)
FUNCTION_INTRO = re.compile(r"\b(?:adds?|introduces?)\s+(?:additional|new)\s+(?:built-in\s+)?(?:texture\s+(?:lookup|gather)\s+)?functions\b", re.I)
LOCAL_FUNCTION_LIST = re.compile(
    r"(?:The existing isotropic vertex texture functions\s*:|"
    r"add the following atomic memory functions to\s+section 8\.|"
    r"Add to the list of texture gather functions\b|"
    r"These objects support reading .*? following\s+functions\s*:|"
    r"Add the following (?:new|additional) function variants\s*:)", re.I)
NON_NORMATIVE_MAPPING = re.compile(r"^\s*Mapping to SPIR-V\s*$", re.I)
CHALLENGE_PROTOTYPE = re.compile(r"(?m)^[ \t]*(?:\|[ \t]*)?(?:(?:highp|mediump|lowp|precise|const)[ \t]+)*([A-Za-z_]\w*)(?:[ \t]+|\n[ \t]*)([A-Za-z_]\w*)[ \t]*\([^{};]{0,400}\)\s*;?")
CHALLENGE_ANGLE = re.compile(r"(?m)^[ \t]*(?:\|[ \t]*)?[A-Za-z_]\w*<[^\n]+>[ \t]+([A-Za-z_]\w*)[ \t]*\(")
CHALLENGE_POINTER = re.compile(r"(?m)^[ \t]*[A-Za-z_]\w*[ \t]+\*+[ \t]*([A-Za-z_]\w*)[ \t]*\(")
CHALLENGE_PROSE = re.compile(r"\bbuilt[ -]in\s+functions?\s+([A-Za-z_]\w*)\b", re.I)
TABLE_CELL = re.compile(r"^\s*\|\s*([^|]+?)\s*\|")
CHALLENGE_TABLE = re.compile(r"(?m)^\s*\|\s*([A-Za-z_]\w*)\s*\|[^\n]*\n\s*\|\s*([A-Za-z_]\w*)\s*\(")
SYNTAX_LABEL = re.compile(r"^\s*Syntax\s*:\s*$", re.I)
SYNTAX_NAME = re.compile(r"\b([A-Za-z_]\w*)\s*\(")
CORE_TYPES = set(json.loads(Path(__file__).with_name("glsl460-functions.json").read_text())["types"])
CORE_FUNCTIONS = set(json.loads(Path(__file__).with_name("glsl460-functions.json").read_text())["functions"])
PROSE_LEAD_WORDS = frozenset({"a", "an", "the", "and", "input", "local", "call", "return", "for",
                              "to", "calling", "between", "be", "of", "counter",
                              "function", "shader", "internal", "different", "fragment"})
# Individually reviewed raw challenger spans in the pinned Registry revision.
# These are source-section dispositions, never callable-name exclusions.
REVIEWED_NON_SHADER_SPANS = {
    ("AMD/AMD_shader_explicit_vertex_parameter.txt", 100, "interpolateAtVertex"):
        "prose reference; Chapter 8 table at line 158 declares interpolateAtVertexAMD",
    ("ARB/ARB_sparse_texture_clamp.txt", 469, "TextureSample"):
        "NV_gpu_program4 texture-access utility, not a GLSL function",
    ("ARB/ARB_uniform_buffer_object.txt", 2957, "MAX_COMBINED_UNIFORM_BLOCKS"):
        "OpenGL token with parenthetical explanation",
    ("EXT/EXT_framebuffer_object.txt", 2781, "EXT_superbuffers"):
        "extension-name list with parenthetical aside",
    ("EXT/EXT_sparse_texture2.txt", 1137, "TextureSample"):
        "NV_gpu_program4 texture-access utility, not a GLSL function",
    ("EXT/EXT_texture_compression_astc_decode_mode.txt", 100, "GL_RGB9_E5"):
        "OpenGL token with parenthetical explanation",
    ("EXT/EXT_texture_compression_latc.txt", 8, "GL_NV_texture_compression_latc"):
        "legacy extension name string",
    ("KHR/KHR_blend_equation_advanced.txt", 840, "f"):
        "blend equation notation, not a GLSL declaration",
    ("NV/NV_blend_equation_advanced.txt", 1630, "f"):
        "blend equation notation, not a GLSL declaration",
    ("NV/NV_gpu_program4.txt", 2358, "TextureSample"):
        "program-instruction description utility, not GLSL",
    ("NV/NV_register_combiners.txt", 394, "FOG"):
        "OpenGL token with parenthetical explanation",
    ("NV/NV_texture_shader.txt", 1037, "TEXTURE_RECTANGLE_NV"):
        "OpenGL token with parenthetical explanation",
    ("NV/NV_vertex_program.txt", 1184, "COLOR"):
        "program input token with parenthetical explanation",
    ("NV/NV_vertex_program.txt", 2067, "Inputs"):
        "program instruction prose, not GLSL",
    ("NV/NV_vertex_program1_1.txt", 138, "Inputs"):
        "program instruction prose, not GLSL",
    ("QCOM/QCOM_shader_framebuffer_fetch_rate.txt", 61, "interpolateAtSample"):
        "listed as disallowed pre-existing built-in, not declared by this extension",
    ("SUN/SUN_triangle_list.txt", 277, "draw"):
        "host-side pseudocode invocation, not GLSL",
}
# Independently inspected GLSL-spec contact lines and parenthetical prose.
# The digest binds every path, complete source SHA, line, parsed tokens, and
# original status, so a new unknown return shape cannot inherit this decision.
GLSL_REVIEWED_PROSE_SHA256 = "3d330b9b011cb0643bbc3e1d52031bbaa924fd3cb7f5eb57497b0c0b3f25b53a"
GLSL_REVIEWED_PROSE_COUNT = 78
GLSL_REVIEWED_PROSE_STATUSES = frozenset({"unclassified_no_modifications_section",
                                         "unclassified_outside_function_section"})
REGISTRY_REVIEWED_STATUSES = frozenset({"unclassified_no_modifications_section",
                                        "unclassified_outside_function_section"})
REGISTRY_REVIEW_DISPOSITIONS = frozenset({"prose_or_metadata_non_signature",
                                          "illustrative_code", "conversion_table",
                                          "host_api", "builtin_declaration"})
OUTSIDE_SHADER_REVIEW_DISPOSITIONS = frozenset({"host_api_unregistered",
                                                "prose_or_metadata_non_signature",
                                                "illustrative_code", "conversion_table",
                                                "shader_builtin_declaration"})
REGISTRY_REVIEW_CANDIDATE_SCHEMA = "fathom.glsl.registry-review-candidate.v1"


def known_return_type(name, extra_types=frozenset()):
    return (name in {"void", "bvec", "dmat"} or name in CORE_TYPES or name in extra_types or
            bool(re.fullmatch(r"[ibud]?gen\w*Type", name)) or
            bool(re.fullmatch(r"[A-Za-z_]*\d+(?:vec\d+|_t)", name)) or
            bool(re.fullmatch(r"(?:gvec\d+|[fiu]?coopmat(?:NV)?)", name)))


def parameterized_signature(line):
    """Read a balanced parameterized return before the callable identifier."""
    text = line.lstrip().removeprefix("|").lstrip()
    head = re.match(r"([A-Za-z_]\w*)<", text)
    if not head:
        return None
    depth = 0
    close = None
    for index in range(head.end() - 1, len(text)):
        if text[index] == "<":
            depth += 1
        elif text[index] == ">":
            depth -= 1
            if depth == 0:
                close = index
                break
            if depth < 0:
                return None
    if close is None:
        return None
    tail = re.match(r"\s+([A-Za-z_]\w*)\s*\(", text[close + 1:])
    if not tail:
        return None
    return head.group(1), tail.group(1), text[:close + 1]


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def extension_name_strings(source):
    """Read the registry's declared GLSL name strings, excluding dependencies."""
    lines = source.splitlines()
    try:
        start = next(index for index, line in enumerate(lines)
                     if line.strip() in {"Name Strings", "Name String"}) + 1
    except StopIteration:
        # A few newer GLSL specs use GL_... directly in the Name block.
        try:
            start = next(index for index, line in enumerate(lines) if line.strip() == "Name") + 1
        except StopIteration:
            return []
    names = []
    for line in lines[start:]:
        stripped = line.strip()
        if stripped in {"Contact", "Contributors", "Status", "Version", "Number", "Dependencies", "Overview", "Name String", "Name Strings"}:
            break
        if re.fullmatch(r"GL_[A-Za-z0-9_]+", stripped):
            names.append(stripped)
        elif stripped.startswith("none (implied by "):
            names.extend(re.findall(r"\bGL_[A-Za-z0-9_]+\b", stripped))
    return list(dict.fromkeys(names))


def starts_definition(lines, index):
    """Recognize a shader function body following a signature, not a prototype."""
    for line in lines[index:index + 8]:
        if ")" not in line:
            continue
        suffix = line.split(")", 1)[1].strip()
        if "{" in suffix.split(";", 1)[0]:
            return True
        if suffix.startswith(";") or suffix.startswith("|"):
            return False
        following = next((part.strip() for part in lines[index + 1:index + 4]
                          if part.strip()), "")
        return following.startswith("{")
    return False


def read_archive(path, expected_sha):
    raw = path.read_bytes()
    if sha(raw) != expected_sha:
        raise ValueError("archive digest mismatch")
    files = {}
    with tarfile.open(path, "r:gz") as archive:
        for member in archive.getmembers():
            if not member.isfile() or "/extensions/" not in member.name or not member.name.endswith(".txt"):
                continue
            relative = member.name.split("/extensions/", 1)[1]
            if relative in files:
                raise ValueError("duplicate extension source path: " + relative)
            source = archive.extractfile(member)
            if source is None:
                raise ValueError("missing extension source: " + relative)
            files[relative] = source.read()
    if not files:
        raise ValueError("no extension specifications in archive")
    return files


def adjudicate_return_notation(relative, rows):
    # Pinned Registry text line 236 writes getType where the generic GLSL
    # notation is genType. The Chapter-8 fwidth signature is still explicit.
    if relative != "ARB/ARB_derivative_control.txt":
        return
    target = [row for row in rows if row.get("return_type") == "getType"
              and row["status"] == "unclassified_return_type"]
    if len(target) != 1 or (target[0]["line_in_normative_section"], target[0]["name"]) != (236, "fwidth"):
        raise ValueError("pinned source return notation no longer matches")
    target[0]["status"] = "candidate_prototype"
    target[0]["reviewed_return_notation"] = "source generic-type typo"


def adjudicate_reviewed_glsl_prose(ledger, expected_digest=GLSL_REVIEWED_PROSE_SHA256,
                                   expected_count=GLSL_REVIEWED_PROSE_COUNT):
    """Apply a closed, pinned line-level review to GLSL non-declarations."""
    reviewed = [(item["path"], item["source_sha256"], row["line_in_normative_section"],
                 row["name"], row["return_type"], row["status"])
                for item in ledger for row in item["candidates"]
                if row["status"] in GLSL_REVIEWED_PROSE_STATUSES]
    actual = sha(json.dumps(sorted(reviewed), separators=(",", ":")).encode())
    if len(reviewed) != expected_count or actual != expected_digest:
        raise ValueError("pinned GLSL non-declaration line review mismatch")
    for item in ledger:
        for row in item["candidates"]:
            if row["status"] in GLSL_REVIEWED_PROSE_STATUSES:
                row["status"] = "excluded_source_reviewed_prose"
                row["review_basis"] = "pinned contact, metadata, or parenthetical prose line"


def _review_row_key(path, source_sha256, row, review=False):
    """Canonical identity for a line-level source adjudication."""
    return (path, source_sha256,
            row["line"] if review else row["line_in_normative_section"],
            row["name"], row.get("return_type", ""))


def registry_review_candidate_sha256(source_sha256, line, name, return_type):
    """Return the canonical private identity for one Registry candidate row."""
    identity = {
        "line": line,
        "name": name,
        "return_type": return_type,
        "schema": REGISTRY_REVIEW_CANDIDATE_SCHEMA,
        "source_sha256": source_sha256,
    }
    return sha(json.dumps(identity, sort_keys=True, separators=(",", ":")).encode())


def adjudicate_registry_review(report, review):
    """Apply an independent, exhaustive review of ambiguous Registry rows."""
    part = report["registry"]
    if (review.get("source_revision") != part.get("revision") or
            review.get("source_archive_sha256") != part.get("archive_sha256")):
        raise ValueError("registry review source pin mismatch")
    if review.get("candidate_identity_schema") != REGISTRY_REVIEW_CANDIDATE_SCHEMA:
        raise ValueError("registry review candidate identity schema mismatch")
    candidates = {}
    candidate_identities = {}
    ambiguous = set()
    for item in part["sources"]:
        for row in item["candidates"]:
            key = _review_row_key(item["path"], item["source_sha256"], row)
            if key in candidates:
                raise ValueError("duplicate registry source row identity")
            candidates[key] = row
            identity = registry_review_candidate_sha256(
                item["source_sha256"], row["line_in_normative_section"],
                row["name"], row.get("return_type", ""))
            if identity in candidate_identities:
                raise ValueError("duplicate registry source candidate digest")
            candidate_identities[identity] = key
            if row["status"] in REGISTRY_REVIEWED_STATUSES:
                ambiguous.add(key)
    reviewed = {}
    for row in review.get("rows", []):
        disposition = row.get("disposition")
        if disposition not in REGISTRY_REVIEW_DISPOSITIONS:
            raise ValueError("unknown registry review disposition")
        if row.get("original_status") not in REGISTRY_REVIEWED_STATUSES:
            raise ValueError("registry review original status is outside review scope")
        if disposition == "builtin_declaration":
            if "candidate_sha256" in row:
                raise ValueError("registry review builtin row must retain factual identifiers")
            try:
                key = _review_row_key(row["source"], row["source_sha256"], row, review=True)
            except KeyError as exc:
                raise ValueError("registry review builtin row is missing factual identifiers") from exc
        else:
            if "name" in row or "return_type" in row:
                raise ValueError("registry review rejected row exposes raw candidate tokens")
            identity = row.get("candidate_sha256")
            if not isinstance(identity, str) or not re.fullmatch(r"[0-9a-f]{64}", identity):
                raise ValueError("registry review rejected row has invalid candidate digest")
            key = candidate_identities.get(identity)
            if key is None or key[:3] != (row.get("source"), row.get("source_sha256"), row.get("line")):
                raise ValueError("registry review rejected row candidate digest mismatch")
        if key in reviewed:
            raise ValueError("duplicate registry review row")
        reviewed[key] = row
    if (review.get("row_count") != len(reviewed) or
            not set(reviewed) <= set(candidates) or not ambiguous <= set(reviewed)):
        raise ValueError("registry review rows do not exactly match ambiguous source rows")
    for key, decision in reviewed.items():
        target = candidates[key]
        disposition = decision["disposition"]
        if disposition == "builtin_declaration":
            target["status"] = "candidate_reviewed_prototype"
        else:
            target["status"] = "excluded_source_reviewed_" + disposition
        target["review_basis"] = "independent pinned source-line adjudication"


def registry_command_names(xml_files):
    """Return exact API command spellings and the Registry's prefixless notation."""
    commands = set()
    for filename, raw in xml_files.items():
        stem = Path(filename).stem
        if stem not in {"gl", "glx", "wgl"}:
            raise ValueError("unexpected Registry command XML")
        prefix = {"gl": "gl", "glx": "glX", "wgl": "wgl"}[stem]
        root = ET.fromstring(raw)
        for command in root.findall("./commands/command"):
            name = command.findtext("proto/name") or command.get("name")
            if name:
                commands.add(name)
                commands.add(name.removeprefix(prefix))
    return commands


def read_registry_commands(path, expected_sha):
    """Read command truth from XML inside the already pinned Registry archive."""
    raw = path.read_bytes()
    if sha(raw) != expected_sha:
        raise ValueError("archive digest mismatch")
    xml_files = {}
    with tarfile.open(path, "r:gz") as archive:
        for member in archive.getmembers():
            filename = Path(member.name).name
            if filename not in {"gl.xml", "glx.xml", "wgl.xml"} or "/xml/" not in member.name:
                continue
            source = archive.extractfile(member)
            if source is None or filename in xml_files:
                raise ValueError("invalid Registry command XML member")
            xml_files[filename] = source.read()
    if set(xml_files) != {"gl.xml", "glx.xml", "wgl.xml"}:
        raise ValueError("pinned Registry command XML set mismatch")
    return registry_command_names(xml_files), {
        filename: sha(raw) for filename, raw in sorted(xml_files.items())}


def adjudicate_host_api_sections(report, command_names):
    """Exclude only non-GLSL rows independently named by pinned Registry XML."""
    for item in report["registry"]["sources"]:
        for row in item["candidates"]:
            if (row["status"] == "unclassified_outside_shader_section" and
                    row["name"] in command_names):
                row["status"] = "excluded_verified_host_api"
                row["review_basis"] = (
                    "non-GLSL section and exact command in pinned Registry XML")


def adjudicate_outside_shader_review(report, review):
    """Apply exhaustive source-context review to rows not proved by Registry XML."""
    part = report["registry"]
    if (review.get("source_revision") != part.get("revision") or
            review.get("source_archive_sha256") != part.get("archive_sha256")):
        raise ValueError("outside-shader review source pin mismatch")
    targets = {}
    for item in part["sources"]:
        for row in item["candidates"]:
            if row["status"] != "unclassified_outside_shader_section":
                continue
            key = _review_row_key(item["path"], item["source_sha256"], row)
            if key in targets:
                raise ValueError("duplicate outside-shader source row identity")
            targets[key] = row
    reviewed = {}
    for row in review.get("rows", []):
        if row.get("original_status") != "unclassified_outside_shader_section":
            raise ValueError("outside-shader review original status mismatch")
        if row.get("disposition") not in OUTSIDE_SHADER_REVIEW_DISPOSITIONS:
            raise ValueError("unknown outside-shader review disposition")
        key = _review_row_key(row["source"], row["source_sha256"], row, review=True)
        if key in reviewed:
            raise ValueError("duplicate outside-shader review row")
        reviewed[key] = row
    if review.get("row_count") != len(reviewed) or set(reviewed) != set(targets):
        raise ValueError("outside-shader review rows do not exactly match unresolved rows")
    for key, decision in reviewed.items():
        target = targets[key]
        disposition = decision["disposition"]
        if disposition == "shader_builtin_declaration":
            target["status"] = "candidate_reviewed_prototype"
        else:
            target["status"] = "excluded_source_reviewed_" + disposition
        target["review_basis"] = "independent pinned source-context adjudication"


def audit_text(source, extra_types=frozenset()):
    rows = []
    section = "unknown"
    function_section = False
    table_return = None
    lines = source.splitlines()
    for number, line in enumerate(lines, 1):
        if EXAMPLE_HEADING.match(line):
            section, function_section = "excluded_example", False
        elif NON_NORMATIVE_MAPPING.match(line):
            section, function_section = "excluded_mapping", False
        elif NON_SHADER_HEADING.match(line):
            section, function_section = "outside_shader", False
        elif section == "normative" and line.strip().lower().rstrip(".") == "none":
            section, function_section = "outside_shader", False
        elif BUILTIN_HEADING.match(line):
            section, function_section = "normative", True
        elif LOCAL_FUNCTION_LIST.search(" ".join(lines[number - 1:number + 2])):
            section, function_section = "normative", True
        elif section == "normative" and line.strip() in {"Functions:", "Built-in Functions:"}:
            function_section = True
        elif MODIFICATION_HEADING.match(line):
            # Amendment headings are often wrapped across two lines.
            parts = [line]
            for continuation in lines[number:number + 2]:
                if not continuation.strip() or MODIFICATION_HEADING.match(continuation):
                    break
                parts.append(continuation)
            heading = " ".join(parts)
            if SHADER_HEADING.search(heading):
                section = "normative"
            elif re.search(r"\bOpenGL\b|\bNV_gpu_program\b", heading, re.I):
                section = "outside_shader"
            # Section-only amendments inherit the last named specification.
            numbered = re.search(r"\b(?:Section|Chapter)\s+([0-9]+)", heading, re.I)
            if numbered:
                function_section = numbered.group(1) == "8"
        elif section == "normative":
            heading = (SECTION_HEADING.match(line) or NUMERIC_SECTION_HEADING.match(line) or
                       re.match(r"^\s*Section\s+([0-9]+)(?:\.[A-Za-z0-9.]+)?(?:,|\s*$|\s+(?![\"']))", line))
            if heading:
                function_section = heading.group(1) == "8"
            elif re.match(r"^\s*At the end of 8\.[X0-9]+\b.*\badd\b.*\bfunctions\b", line, re.I):
                function_section = True
            elif re.search(r"\b(?:created|set|initialized|updated)\s+by\s+calling\s*:?\s*$", line, re.I):
                function_section = True
            elif FUNCTION_INTRO.search(" ".join(lines[number - 1:number + 1])):
                function_section = True
        cell = TABLE_CELL.match(line)
        if cell:
            first = cell.group(1).strip()
            if table_return and (table_call := re.match(r"([A-Za-z_]\w*)\s*\(", first)):
                rows.append({"line_in_normative_section": number, "name": table_call.group(1),
                             "return_type": table_return, "section": section,
                             "status": "candidate_table_prototype" if section == "normative" and function_section
                             else "unclassified_table_prototype"})
            table_return = first if known_return_type(first, extra_types) else None
        else:
            table_return = None
        match = PROTOTYPE.match(line) or PROTOTYPE_POINTER.match(line)
        parameterized = None if match else parameterized_signature(line)
        if match or parameterized:
            if match:
                return_type, name = match.groups()
                return_base = return_type
            else:
                return_base, name, return_type = parameterized
            status = ("excluded_prose_sentence" if return_type.lower() in PROSE_LEAD_WORDS else
                      "excluded_shader_definition" if starts_definition(lines, number - 1) else
                      "excluded_example" if section == "excluded_example" else
                      "excluded_non_normative_mapping" if section == "excluded_mapping" else
                      "unclassified_no_modifications_section" if section == "unknown" else
                      "unclassified_outside_shader_section" if section == "outside_shader" else
                      "unclassified_outside_function_section" if not function_section else
                      "candidate_prototype" if known_return_type(return_base, extra_types) else "unclassified_return_type")
            rows.append({"line_in_normative_section": number, "name": name,
                         "return_type": return_type, "section": section,
                         "status": status})
        elif number > 1 and (split := re.match(r"^\s*([A-Za-z_]\w*)\s*\(", line)):
            previous = lines[number - 2].strip()
            if known_return_type(previous, extra_types):
                split_status = ("excluded_example" if section == "excluded_example" else
                                "unclassified_outside_shader_section" if section == "outside_shader" else
                                "unclassified_split_prototype")
                rows.append({"line_in_normative_section": number, "name": split.group(1),
                             "return_type": previous, "section": section,
                             "status": split_status})
        prose = PROSE_DECL.search(line)
        if prose and not any(row["name"] == prose.group(1) for row in rows):
            prose_status = ("excluded_hypothetical_program_function"
                            if section == "outside_shader" and "hypothetical" in line.lower()
                            else "unclassified_prose_declaration")
            rows.append({"line_in_normative_section": number, "name": prose.group(1),
                         "section": section, "status": prose_status})
    return rows


def challenger_text(source):
    """Independent raw-text span scan; deliberately ignores section state."""
    rows = []
    for match in CHALLENGE_PROTOTYPE.finditer(source):
        rows.append({"name": match.group(2), "line": source.count("\n", 0, match.start(2)) + 1,
                     "kind": "prototype_span"})
    for match in CHALLENGE_ANGLE.finditer(source):
        rows.append({"name": match.group(1), "line": source.count("\n", 0, match.start(1)) + 1,
                     "kind": "parameterized_return_span"})
    for match in CHALLENGE_POINTER.finditer(source):
        rows.append({"name": match.group(1), "line": source.count("\n", 0, match.start(1)) + 1,
                     "kind": "pointer_return_span"})
    for match in CHALLENGE_PROSE.finditer(source):
        rows.append({"name": match.group(1), "line": source.count("\n", 0, match.start(1)) + 1,
                     "kind": "prose_span"})
    for match in CHALLENGE_TABLE.finditer(source):
        rows.append({"name": match.group(2), "line": source.count("\n", 0, match.start(2)) + 1,
                     "kind": "split_table_span"})
    lines = source.splitlines()
    for index, line in enumerate(lines):
        if not SYNTAX_LABEL.match(line):
            continue
        seen, empty = False, 0
        for next_index in range(index + 1, min(index + 101, len(lines))):
            next_line = lines[next_index]
            if re.match(r"^\s*(?:Description|Parameters|Modifications|Additions|Changes)\b", next_line, re.I):
                break
            if not next_line.strip():
                empty += 1
                if seen:
                    break
                continue
            empty = 0
            for match in SYNTAX_NAME.finditer(next_line):
                seen = True
                rows.append({"name": match.group(1), "line": next_index + 1,
                             "kind": "syntax_block_name"})
    return rows


def classify_challenger(source, row, accounted_names):
    """Disposition raw matches without sharing the main heading parser."""
    name = row["name"]
    if name in accounted_names:
        return "accounted_name"
    line = source.splitlines()[row["line"] - 1]
    if row["kind"] in {"prototype_span", "parameterized_return_span", "pointer_return_span"}:
        if row["line"] < 20 and ("Copyright" in line or "Khronos" in line):
            return "copyright_preamble"
        return "unresolved_prototype_span"
    if row["kind"] == "split_table_span":
        return "unresolved_split_table_span"
    if row["kind"] == "syntax_block_name":
        return "unresolved_syntax_block_name"
    if name in CORE_FUNCTIONS:
        return "core_reference_without_local_declaration"
    if re.search(r"\b" + re.escape(name) + r"\s*\(", line):
        return "unresolved_named_call"
    if "_" in name or re.search(r"[A-Z\d]", name[1:]):
        return "unresolved_named_reference"
    # A plain word after “built-in functions” is sentence grammar unless a
    # signature exists. All source lines remain in challenger_dispositions.
    if name.islower() or name[:1].isupper():
        return "ordinary_prose"
    if re.search(r"\b(?:add|adds|added|introduce|introduces|define|defines|declare|declares|new|named|called)\b", line, re.I):
        return "unresolved_declaration_prose"
    return "ordinary_prose"


def reconcile_cross_references(report):
    """Resolve a prose reference to a separately sourced declaration."""
    declarations = {}
    for source_name, part in report.items():
        for item in part["sources"]:
            for row in item["candidates"]:
                if row["status"] in ("candidate_prototype", "candidate_table_prototype"):
                    declarations.setdefault(row["name"], []).append(
                        {"repository": source_name, "source_sha256": item["source_sha256"],
                         "line": row["line_in_normative_section"]})
    for part in report.values():
        for item in part["sources"]:
            for row in item["challenger_dispositions"]:
                origins = [origin for origin in declarations.get(row["name"], [])
                           if origin["source_sha256"] != item["source_sha256"]]
                if (row["kind"] == "prose_span" and row["disposition"].startswith("unresolved_")
                        and not row.get("declaration_cue") and origins):
                    row["disposition"] = "cross_source_prose_reference"
                    row["declared_at"] = origins
            item["challenger_gaps"] = [row for row in item["challenger_dispositions"]
                                       if row["disposition"].startswith("unresolved_")]
        part["challenger_gap_count"] = sum(len(item["challenger_gaps"]) for item in part["sources"])


def adjudicate_reviewed_non_shader_spans(report):
    """Apply exact source-line adjudications after validating pinned archives."""
    matched = set()
    for item in report["registry"]["sources"]:
        for row in item["challenger_dispositions"]:
            key = (item["path"], row["line"], row["name"])
            if key in REVIEWED_NON_SHADER_SPANS and row["disposition"].startswith("unresolved_"):
                row["disposition"] = "reviewed_non_shader_span"
                row["basis"] = REVIEWED_NON_SHADER_SPANS[key]
                matched.add(key)
        item["challenger_gaps"] = [row for row in item["challenger_dispositions"]
                                   if row["disposition"].startswith("unresolved_")]
    missing = set(REVIEWED_NON_SHADER_SPANS) - matched
    if missing:
        raise ValueError("reviewed source spans no longer match pinned audit: " + repr(sorted(missing)))
    report["registry"]["challenger_gap_count"] = sum(len(item["challenger_gaps"])
                                                      for item in report["registry"]["sources"])


def candidate_digest(report):
    """Bind a reviewed accepted-row set to source bytes, lines, and parser output."""
    rows = []
    for repository in sorted(report):
        for item in report[repository]["sources"]:
            for row in item["candidates"]:
                if row["status"].startswith("candidate_"):
                    rows.append((repository, item["path"], item["source_sha256"],
                                 row["line_in_normative_section"], row["name"],
                                 row.get("return_type", ""), row["status"]))
    return sha(json.dumps(sorted(rows), separators=(",", ":")).encode())


def validate_function_review(report, review):
    """Require a separately authored positive receipt for the complete accepted set."""
    if review.get("verdict") != "accepted":
        raise ValueError("function review verdict is not accepted")
    if not isinstance(review.get("review_method"), str) or not review["review_method"].strip():
        raise ValueError("function review method is missing")
    pins = {name: {"revision": part.get("revision"),
                   "archive_sha256": part.get("archive_sha256"),
                   "source_count": part["source_count"]}
            for name, part in sorted(report.items())}
    if review.get("source_pins") != pins:
        raise ValueError("function review source pins mismatch")
    digest = candidate_digest(report)
    if review.get("accepted_candidate_sha256") != digest:
        raise ValueError("accepted function candidate review digest mismatch")
    rows = [row for part in report.values() for item in part["sources"]
            for row in item["candidates"] if row["status"].startswith("candidate_")]
    if (review.get("accepted_candidate_count") != len(rows) or
            review.get("accepted_function_count") != len({row["name"] for row in rows})):
        raise ValueError("function review accepted counts mismatch")
    return {"accepted_candidate_sha256": digest,
            "accepted_candidate_count": len(rows),
            "accepted_function_count": len({row["name"] for row in rows}),
            "review_method": review["review_method"]}


def build_function_catalog(report, function_review, expected_counts=None):
    """Emit only after the source candidate set has independent positive review."""
    expected_counts = expected_counts or {"glsl": 96, "registry": 1025}
    if set(report) != set(expected_counts):
        raise ValueError("source repository set mismatch")
    review_summary = validate_function_review(report, function_review)
    by_extension = {}
    provenance = []
    for repository, expected_count in expected_counts.items():
        part = report[repository]
        if part["source_count"] != expected_count or len(part["sources"]) != expected_count:
            raise ValueError("pinned extension source count mismatch")
        if part["challenger_gap_count"] or any(item["challenger_gaps"] for item in part["sources"]):
            raise ValueError("unresolved independent challenger gap")
        for item in part["sources"]:
            uncertain = [row for row in item["candidates"] if row["status"].startswith("unclassified_")]
            if uncertain:
                raise ValueError("unclassified function declaration candidate")
            accepted = [row for row in item["candidates"] if row["status"].startswith("candidate_")]
            names = {row["name"] for row in accepted}
            if names and not item["name_strings"]:
                raise ValueError("accepted function source lacks extension name strings")
            for extension in item["name_strings"]:
                by_extension.setdefault(extension, set()).update(names)
                provenance.extend({"extension": extension, "repository": repository,
                                   "path": item["path"],
                                   "source_sha256": item["source_sha256"],
                                   "line": row["line_in_normative_section"],
                                   "name": row["name"],
                                   "return_type": row.get("return_type", ""),
                                   "status": row["status"]}
                                  for row in accepted)
    by_extension = {extension: sorted(names) for extension, names in sorted(by_extension.items())}
    return {"complete": True, "functions": sorted({name for names in by_extension.values() for name in names}),
            "functions_by_extension": by_extension,
            "function_provenance": sorted(provenance, key=lambda row: (
                row["extension"], row["name"], row["repository"], row["path"], row["line"],
                row["return_type"], row["status"])),
            "accepted_candidate_sha256": review_summary["accepted_candidate_sha256"],
            "function_review": review_summary,
            "source_review_receipts": report["registry"].get("source_review_receipts", {}),
            "host_api_xml_sha256": report["registry"].get("host_api_xml_sha256", {}),
            "source_pins": {repository: {"revision": report[repository].get("revision"),
                                         "archive_sha256": report[repository].get("archive_sha256"),
                                         "source_count": report[repository]["source_count"]}
                            for repository in sorted(report)}}


def compose_catalog(report, type_payload, function_review, expected_counts=None):
    """Join independently reviewed function and type facts, then fail closed."""
    type_audit = type_payload.get("type_audit", {})
    if (type_audit.get("complete") is not True or
            type_audit.get("unresolved_candidates") != 0 or
            type_audit.get("challenger_gaps") != 0):
        raise ValueError("extension type audit remains incomplete")
    functions = build_function_catalog(report, function_review, expected_counts)
    sources = [{"repository": name, **functions["source_pins"][key]}
               for key, name in (("glsl", "KhronosGroup/GLSL"),
                                 ("registry", "KhronosGroup/OpenGL-Registry"))]
    if type_payload.get("type_sources") != sources:
        raise ValueError("type and function source pins differ")
    types = type_payload.get("types")
    by_extension = type_payload.get("types_by_extension")
    if (not isinstance(types, list) or not isinstance(by_extension, dict) or
            types != sorted(set(types)) or
            set(types) != {name for names in by_extension.values() for name in names} or
            not set(type_payload.get("constructors", [])) <= set(types)):
        raise ValueError("extension type catalog is inconsistent")
    return {"complete": True, "functions": functions["functions"],
            "functions_by_extension": functions["functions_by_extension"],
            "function_provenance": functions["function_provenance"],
            "types": types, "types_by_extension": by_extension,
            "constructors": type_payload["constructors"],
            "constructors_by_extension": type_payload.get("constructors_by_extension", {}),
            "type_provenance": type_payload.get("type_provenance", {}),
            "sources": sources,
            "accepted_function_candidate_sha256": functions["accepted_candidate_sha256"],
            "function_review": functions["function_review"],
            "source_review_receipts": functions["source_review_receipts"],
            "host_api_xml_sha256": functions["host_api_xml_sha256"],
            "audit": {"unresolved_candidates": 0, "challenger_gaps": 0}}


def audit_archive(path, expected_sha, revision, extra_types=frozenset()):
    ledger = []
    for relative, raw in sorted(read_archive(path, expected_sha).items()):
        text = raw.decode("utf-8", errors="replace")
        rows = audit_text(text, extra_types)
        adjudicate_return_notation(relative, rows)
        accounted = {row["name"] for row in rows}
        challenges = [{**row, "declaration_cue": bool(re.search(
                           r"\b(?:add|introduce|define|declare|new)\b",
                           text.splitlines()[row["line"] - 1].split("built-in function")[0], re.I)),
                       "disposition": classify_challenger(text, row, accounted)}
                      for row in challenger_text(text)]
        gaps = [row for row in challenges if row["disposition"].startswith("unresolved_")]
        ledger.append({"path": relative, "source_sha256": sha(raw), "candidates": rows,
                       "name_strings": extension_name_strings(text),
                       "challenger_dispositions": challenges,
                       "challenger_gaps": gaps,
                       "mentions_glsl": "Shading Language" in text or "GLSL" in text})
    if revision == GLSL_REVISION:
        adjudicate_reviewed_glsl_prose(ledger)
    return {"revision": revision, "archive_sha256": expected_sha, "source_count": len(ledger),
            "candidate_count": sum(len(item["candidates"]) for item in ledger),
            "challenger_gap_count": sum(len(item["challenger_gaps"]) for item in ledger),
            "files_with_candidates": sum(bool(item["candidates"]) for item in ledger),
            "families": dict(sorted(Counter(item["path"].split("/", 1)[0]
                                            for item in ledger if item["candidates"]).items())),
            "sources": ledger}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("glsl_archive", type=Path)
    parser.add_argument("registry_archive", type=Path)
    parser.add_argument("out", type=Path, help="ignored-workspace audit receipt")
    parser.add_argument("--with-pinned-types", action="store_true",
                        help="use independently extracted types to recognize extension return types")
    parser.add_argument("--registry-review", type=Path,
                        help="independent exhaustive adjudication of ambiguous Registry rows")
    parser.add_argument("--outside-shader-review", type=Path,
                        help="independent source-context adjudication after XML host proof")
    parser.add_argument("--function-review", type=Path,
                        help="independent positive receipt for the complete accepted function set")
    parser.add_argument("--catalog-out", type=Path,
                        help="emit a complete function/type catalog after every audit closes")
    args = parser.parse_args()
    if "workspace" not in args.out.resolve().parts:
        raise SystemExit("audit ledger must stay in ignored workspace")
    extra_types = frozenset()
    type_payload = None
    if args.with_pinned_types or args.catalog_out:
        type_path = Path(__file__).with_name("extract_extension_types.py")
        type_spec = importlib.util.spec_from_file_location("extract_extension_types", type_path)
        type_module = importlib.util.module_from_spec(type_spec)
        type_spec.loader.exec_module(type_module)
        type_payload = type_module.extract(args.glsl_archive, args.registry_archive)
        if args.catalog_out and not type_payload["type_audit"]["complete"]:
            raise ValueError("independent extension type audit incomplete")
        extra_types = frozenset(type_payload["types"])
    report = {"glsl": audit_archive(args.glsl_archive, GLSL_ARCHIVE_SHA256, GLSL_REVISION, extra_types),
              "registry": audit_archive(args.registry_archive, REGISTRY_ARCHIVE_SHA256, REGISTRY_REVISION, extra_types)}
    reconcile_cross_references(report)
    adjudicate_reviewed_non_shader_spans(report)
    review_receipts = {}
    if args.registry_review:
        review_raw = args.registry_review.read_bytes()
        adjudicate_registry_review(report, json.loads(review_raw))
        review_receipts["ambiguous_registry_rows_sha256"] = sha(review_raw)
    command_names, command_hashes = read_registry_commands(
        args.registry_archive, REGISTRY_ARCHIVE_SHA256)
    adjudicate_host_api_sections(report, command_names)
    report["registry"]["host_api_xml_sha256"] = command_hashes
    if args.outside_shader_review:
        outside_raw = args.outside_shader_review.read_bytes()
        adjudicate_outside_shader_review(report, json.loads(outside_raw))
        review_receipts["outside_shader_rows_sha256"] = sha(outside_raw)
    report["registry"]["source_review_receipts"] = dict(sorted(review_receipts.items()))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({key: {field: value for field, value in part.items() if field != "sources"}
                      for key, part in report.items()}, indent=2))
    digest = candidate_digest(report)
    print("accepted candidate digest (awaiting independent positive review):", digest)
    if not args.catalog_out:
        raise SystemExit("candidate audit only; complete function catalog not established")
    if not args.registry_review or not args.function_review:
        raise ValueError("catalog emission requires source adjudications and function review")
    if "workspace" not in args.catalog_out.resolve().parts:
        raise ValueError("catalog must stay in ignored workspace")
    catalog = compose_catalog(report, type_payload, json.loads(args.function_review.read_text()))
    args.catalog_out.parent.mkdir(parents=True, exist_ok=True)
    args.catalog_out.write_text(json.dumps(catalog, indent=2) + "\n")
    print(json.dumps({"complete": True, "functions": len(catalog["functions"]),
                      "types": len(catalog["types"]), "catalog": str(args.catalog_out)}, indent=2))


if __name__ == "__main__":
    main()
