"""sam_probe.py - how much of cfn_oracle.py's tier-C cross_references (-xref-out) the AWS SAM CLI's own
resolvers report (IF-C6, docs/bench/readings/iac-facts/c6-cloudformation-l3.md).

Run it with an interpreter that has aws-sam-cli installed (pip install aws-sam-cli==1.171.0), from the
subject's root, offline:

    python sam_probe.py -xref <cfn-xref.json>

SamLocalStackProvider.get_stacks(template) is the CLI's resolution of a nested stack (a relative local
TemplateURL or Location); SamFunctionProvider reports each function's CodeUri and Handler string after the
SAM transform has applied Globals. Neither resolves a handler to a file and symbol (the runtime does that
when it invokes), so this probe grades the part of the truth the CLI can report: the nested-template edges,
and, per handler edge, that the function exists and that its CodeUri directory holds the file the truth
names. A function whose CodeUri is the template's own directory holds every file under it, so those are counted
apart as sam_codeuri_root:handler and are not a directory match. It prints one JSON object of counts.
"""
import argparse
import collections
import json
import logging
import os

logging.disable(logging.CRITICAL)

from samcli.lib.providers.sam_function_provider import SamFunctionProvider  # noqa: E402
from samcli.lib.providers.sam_stack_provider import SamLocalStackProvider  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-xref", required=True)
    a = ap.parse_args()
    cr = json.load(open(a.xref))["cross_references"]
    by = collections.defaultdict(list)
    for c in cr:
        by[c["path"]].append(c)
    n = collections.Counter()
    for path, cs in sorted(by.items()):
        try:
            stacks, _ = SamLocalStackProvider.get_stacks(path)
            funcs = {f.name: f for f in SamFunctionProvider(stacks, use_raw_codeuri=True).get_all()}
        except Exception as e:  # pylint: disable=broad-except
            n["template_error"] += len(cs)
            n["error:" + type(e).__name__] += len(cs)
            continue
        kids = {os.path.normpath(s.location) for s in stacks[1:]}
        for c in cs:
            edge = c["edge"]
            n["truth:" + edge] += 1
            if edge in ("nested_stack", "nested_application"):
                if os.path.normpath(c["to_path"]) in kids:
                    n["sam_found:" + edge] += 1
            elif edge == "handler":
                f = funcs.get(c["from"])
                if f is None:
                    continue
                n["sam_function:handler"] += 1
                base = os.path.dirname(path)
                code = os.path.normpath(os.path.join(base, f.codeuri or "."))
                if code == ".":
                    # A CodeUri that is the template's own directory holds every file under it, so the check
                    # says nothing. Count it apart, and never as a directory match.
                    n["sam_codeuri_root:handler"] += 1
                elif os.path.normpath(c["to_path"]).startswith(code + os.sep):
                    n["sam_codeuri_holds_file:handler"] += 1
    print(json.dumps(dict(sorted(n.items())), indent=1))


if __name__ == "__main__":
    main()
