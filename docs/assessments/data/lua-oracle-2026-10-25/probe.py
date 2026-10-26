#!/usr/bin/env python3
"""Replay LuaLS documentation and EmmyLua diagnostic contract probes offline."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--luals', required=True)
ap.add_argument('--emmy', required=True)
ap.add_argument('--out', required=True)
a = ap.parse_args()
out = Path(a.out).resolve()
out.mkdir(parents=True, exist_ok=False)
subject = out / 'subject'
subject.mkdir()
(out / 'doc').mkdir()
shutil.copyfile(Path(__file__).with_name('probe.lua'), subject / 'probe.lua')
p = subprocess.run([str(Path(a.luals).resolve()), '--doc=' + str(subject),
                    '--doc_out_path=' + str(out / 'doc'), '--logpath=' + str(out / 'logs'),
                    '--metapath=' + str(out / 'meta')], capture_output=True, text=True, timeout=120)
assert p.returncode == 0, 'LuaLS doc failed; inspect local logs'
doc = json.loads((out / 'doc/doc.json').read_text())
for r in doc:
    if 'DOC' in r: r['DOC'] = '<probe>'
assert len(doc) == 1 and doc[0]['defines'] == [] and doc[0]['fields'] == []
(out / 'doc.json').write_text(json.dumps(doc, indent=2, sort_keys=True) + '\n')
version = subprocess.check_output([str(Path(a.emmy).resolve()), '--version'], text=True).strip()
p = subprocess.run([str(Path(a.emmy).resolve()), str(subject), '--output-format', 'json'],
                   capture_output=True, text=True, timeout=120)
assert p.returncode in (0, 1), 'EmmyLua check failed'
# Keep the native diagnostic schema, replacing only absolute probe paths.
text = p.stdout.replace(str(subject), '<probe>')
(out / 'emmy.json').write_text(text)
summary = {'luals_version': '3.19.2', 'doc_records': len(doc), 'defines': 0, 'fields': 0,
           'emmy_version': version, 'emmy_exit': p.returncode,
           'emmy_output_keys': sorted(json.loads(text)) if isinstance(json.loads(text), dict) else 'list',
           'scope': 'instrument contract only; no subject grade or resolver coverage'}
(out / 'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
print(json.dumps(summary, sort_keys=True))
