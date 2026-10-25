"""xcheck_misshit.py: an ungraded cross-check of fathom's Simulink block reading against
miss-hit-core's own .slx parser (miss_hit_core.s_parser, GPL-3, run from a venv under
workspace/tools, never vendored). Counts only: no model content is written.

    <venv>/bin/python -I xcheck_misshit.py -subjects <calibration dir> -fathom <blocks.jsonl> -out <json>

blocks.jsonl is one JSON object per model file, {"File": <path under the calibration dir>,
"Err": <reader error or "">, "Blocks": [{"SID": ..., "Path": ...}]}, written from
extractor.ReadSimulinkModel. miss-hit reads .slx only; a block is compared by its SID, its
path local to the model (fathom's doubled `/` undoubled, a line break read as a space).
Classes per subject: agree, trailing_break (a name ending in a line break, which fathom
reads as a trailing space and miss-hit drops), path_differs, missing_in_fathom (a miss-hit SID fathom has no
block for), fathom_only (fathom's blocks miss-hit does not report, the chart-internal
blocks it folds among them); models parsed, refused (an exception), external harnesses
skipped.
"""

import argparse
import json
import os
import sys

from miss_hit_core.config import Config
from miss_hit_core.errors import Message_Handler
from miss_hit_core.s_parser import Simulink_SLX_Parser


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subjects", required=True)
    ap.add_argument("-fathom", required=True)
    ap.add_argument("-out", required=True)
    a = ap.parse_args()
    fathom = {}
    with open(a.fathom) as f:
        for line in f:
            m = json.loads(line)
            fathom[m["File"]] = m
    out = {}
    for rel in sorted(fathom):
        if not rel.endswith(".slx"):
            continue
        subj = rel.split("/", 1)[0]
        c = out.setdefault(subj, {k: 0 for k in ("models", "parsed", "refused", "external_harness",
                                                 "agree", "trailing_break", "path_differs", "missing_in_fathom",
                                                 "fathom_only")})
        c["models"] += 1
        fm = fathom[rel]
        by_sid = {}
        for b in fm["Blocks"] or []:
            local = b["Path"].split("/", 1)[1] if "/" in b["Path"] else ""
            by_sid.setdefault(b["SID"], local.replace("//", "/"))
        mh = Message_Handler("debug")
        mh.register_file(os.path.join(a.subjects, rel))
        try:
            slp = Simulink_SLX_Parser(mh, os.path.join(a.subjects, rel), Config())
            if slp.is_external_harness:
                c["external_harness"] += 1
                continue
            container = slp.parse_file()
            blocks = list(container.iter_all_blocks())
        except Exception as e:  # noqa: BLE001: any failure is one refused model, counted by class
            c["refused"] += 1
            k = "refused_" + type(e).__name__
            c[k] = c.get(k, 0) + 1
            continue
        c["parsed"] += 1
        seen = set()
        for b in blocks:
            seen.add(b.sid)
            local = " ".join(b.local_name().splitlines())
            if b.sid not in by_sid:
                c["missing_in_fathom"] += 1
            elif by_sid[b.sid] == local:
                c["agree"] += 1
            elif "/".join(x.rstrip() for x in by_sid[b.sid].split("/")) == "/".join(x.rstrip() for x in local.split("/")):
                c["trailing_break"] += 1
            else:
                c["path_differs"] += 1
        c["fathom_only"] += sum(1 for s in by_sid if s not in seen)
    tot = {}
    for c in out.values():
        for k, v in c.items():
            tot[k] = tot.get(k, 0) + v
    json.dump({"tool": "miss_hit_core.s_parser", "subjects": out, "total": tot}, open(a.out, "w"), indent=1, sort_keys=True)
    print(json.dumps(tot, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
