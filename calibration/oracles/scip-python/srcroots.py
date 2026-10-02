"""srcroots.py <subject-dir>: print the subject's Python source roots, ':'-joined.

A source root is a directory, below the subject, that holds a package (`<root>/<pkg>/__init__.py`)
and is not the subject itself: `src` by convention, plus the roots pyproject.toml names
(setuptools `package-dir`, poetry `packages` with `from`). The subject root itself is
scip-python's own cwd and needs no help. Output is sorted and deduplicated (deterministic).
"""
import sys
import tomllib
from pathlib import Path


def has_package(d: Path) -> bool:
    return d.is_dir() and any(c.is_dir() and (c / "__init__.py").is_file() for c in d.iterdir())


def candidates(subj: Path):
    yield "src"
    pp = subj / "pyproject.toml"
    if not pp.is_file():
        return
    try:
        data = tomllib.loads(pp.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError):
        return
    tool = data.get("tool", {})
    for v in (tool.get("setuptools", {}).get("package-dir") or {}).values():
        yield v
    for pkg in tool.get("poetry", {}).get("packages") or []:
        if isinstance(pkg, dict) and pkg.get("from"):
            yield pkg["from"]


def roots(subj: Path):
    out = set()
    for c in candidates(subj):
        d = (subj / c).resolve()
        if d != subj.resolve() and subj.resolve() in d.parents and has_package(d):
            out.add(str(d))
    return sorted(out)


if __name__ == "__main__":
    print(":".join(roots(Path(sys.argv[1]))))
