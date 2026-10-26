"""Recheck authored syntax controls against Jinja only, never the product parser."""
import argparse
from importlib.metadata import version
import json
from pathlib import Path
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-pythonpath", required=True)
    parser.add_argument("-controls", default="internal/extractor/dialects/jinja2/testdata/syntax-controls.json")
    parser.add_argument("-write", action="store_true", help="regenerate Jinja verdicts; review any diff")
    args = parser.parse_args()
    sys.path.insert(0, str(Path(args.pythonpath).resolve()))
    from jinja2 import Environment, TemplateSyntaxError
    if version("jinja2") != "3.1.6":
        parser.error("requires Jinja2 3.1.6")
    path = Path(args.controls)
    controls = json.loads(path.read_text())
    env = Environment()
    mismatches = []
    for case in controls["cases"]:
        try:
            env.parse(case["source"])
            valid = True
        except TemplateSyntaxError:
            valid = False
        if case["jinja_valid"] != valid:
            mismatches.append(case["name"])
        case["jinja_valid"] = valid
    if args.write:
        controls["oracle"] = "Jinja2 3.1.6"
        path.write_text(json.dumps(controls, indent=2) + "\n")
    elif mismatches:
        raise SystemExit("Jinja verdict changed: " + ", ".join(mismatches))
    print(f"Jinja2 3.1.6: {len(controls['cases'])} syntax verdicts checked")


if __name__ == "__main__":
    main()
