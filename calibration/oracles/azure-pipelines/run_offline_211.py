"""Run only the authored SDK API probe in a network-denied macOS process."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
from offline_probe_211 import validate

parser=argparse.ArgumentParser()
parser.add_argument('--dotnet',type=Path,required=True)
parser.add_argument('--assembly',type=Path,required=True)
parser.add_argument('--out',type=Path,required=True)
args=parser.parse_args()
args.out.mkdir(exist_ok=False,parents=True)
root=args.dotnet.resolve().parent.parent
local=root/'probe-runtime';local.mkdir(exist_ok=True)
env=dict(os.environ,DOTNET_ROOT=str(args.dotnet.resolve().parent),DOTNET_CLI_HOME=str(local),
         DOTNET_GENERATE_ASPNET_CERTIFICATE='false',DOTNET_ADD_GLOBAL_TOOLS_TO_PATH='false',
         DOTNET_CLI_TELEMETRY_OPTOUT='1',DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1',
         DOTNET_CLI_USE_MSBUILD_SERVER='0',MSBUILDDISABLENODEREUSE='1',
         DOTNET_GCHeapHardLimit='0x30000000')
cmd=['/usr/bin/sandbox-exec','-p','(version 1)(allow default)(deny network*)',str(args.dotnet.resolve()),str(args.assembly.resolve())]
r=subprocess.run(cmd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
(args.out/'raw.json').write_bytes(r.stdout);(args.out/'stderr.log').write_bytes(r.stderr)
if r.returncode:
    raise RuntimeError('offline producer process failed: exit '+str(r.returncode))
if len(r.stdout)>1024**2:
    raise ValueError('post-capture parse cap')
summary=validate(json.loads(r.stdout))
summary.update(raw_sha256=hashlib.sha256(r.stdout).hexdigest(),assembly_sha256=hashlib.sha256(args.assembly.read_bytes()).hexdigest(),
               dotnet_sha256=hashlib.sha256(args.dotnet.read_bytes()).hexdigest(),network='sandbox-exec deny network*',
               process_timeout_seconds=30,heap_limit_bytes=768*1024**2,services_started=False,jobs_executed=False)
(args.out/'receipt.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary))
