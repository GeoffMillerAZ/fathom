"""Bound independent parser output before constructing original-site scope."""
import json
from pathlib import Path
import resource
import subprocess
import tempfile


def parse(shfmt, source, max_bytes=32 << 20, max_depth=256):
    if not (1 <= max_bytes <= 32 << 20 and 1 <= max_depth <= 256):
        raise ValueError('whole-unit refusal: invalid parser caps')
    parent = Path(__file__).resolve().parents[3] / 'workspace/tmp'
    parent.mkdir(parents=True, exist_ok=True)
    def limit():
        # One byte above the accepted limit proves overflow before JSON loading.
        resource.setrlimit(resource.RLIMIT_FSIZE, (max_bytes + 1, max_bytes + 1))
    with tempfile.TemporaryFile(dir=parent) as output, tempfile.TemporaryFile(dir=parent) as error:
        try:
            result = subprocess.run([shfmt, '-ln', 'bash', '--to-json'], input=source,
                                    stdout=output, stderr=error, preexec_fn=limit, timeout=60)
        except subprocess.TimeoutExpired as exc:
            raise ValueError('whole-unit refusal: parser timeout') from exc
        if output.tell() > max_bytes or error.tell() > max_bytes:
            raise ValueError('whole-unit refusal: parser output byte cap')
        if result.returncode:
            raise ValueError('whole-unit refusal: original parser input failed')
        output.seek(0)
        try:
            tree = json.load(output)
        except (ValueError, RecursionError) as exc:
            raise ValueError('whole-unit refusal: malformed or overnested parser output') from exc
    pending = [(tree, 1)]
    while pending:
        node, depth = pending.pop()
        if depth > max_depth:
            raise ValueError('whole-unit refusal: parser nesting cap')
        children = node.values() if isinstance(node, dict) else node if isinstance(node, list) else ()
        pending.extend((child, depth + 1) for child in children)
    return tree


def source_sites(tree, root, relative, syntax, max_sites=1_000_000):
    if not 0 <= max_sites <= 1_000_000:
        raise ValueError('whole-unit refusal: invalid source-site cap')
    sites, dynamic, external = [], 0, 0
    for line, word in syntax.source_statements(tree):
        value, kind = syntax.word_literal(word) if word else ('', 'no_argument')
        if kind != 'literal':
            dynamic += 1
            continue
        if len(sites) >= max_sites:
            raise ValueError('whole-unit refusal: source site cap')
        if value.startswith('/') and not syntax.inside(str(root), value):
            external += 1
        sites.append(dict(path=relative, line=line, value=value))
    return sites, dynamic, external
