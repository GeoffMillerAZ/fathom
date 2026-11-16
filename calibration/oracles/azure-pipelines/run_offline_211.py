"""Run only the authored SDK API probe in a network-denied macOS process."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
from offline_probe_211 import (ReceiptRefusal, decode_receipt, invalidate_summary,
                               refuse_aliases, validate, write_summary)


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--dotnet',type=Path,required=True)
    parser.add_argument('--assembly',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    try:
        refuse_aliases([args.dotnet, args.assembly],
                       [args.out/'receipt.json', args.out/'raw.json', args.out/'stderr.log'])
        args.out.mkdir(exist_ok=True,parents=True)
        invalidate_summary(args.out/'receipt.json')
        local=args.out.resolve()/'runtime'
        local.mkdir(exist_ok=True)
        env=dict(os.environ,DOTNET_ROOT=str(args.dotnet.resolve().parent),DOTNET_CLI_HOME=str(local/'cli-home'),
                 NUGET_PACKAGES=str(local/'nuget/packages'),NUGET_HTTP_CACHE_PATH=str(local/'nuget/http'),
                 DOTNET_GENERATE_ASPNET_CERTIFICATE='false',DOTNET_ADD_GLOBAL_TOOLS_TO_PATH='false',
                 DOTNET_CLI_TELEMETRY_OPTOUT='1',DOTNET_NOLOGO='1',DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1',
                 DOTNET_CLI_WORKLOAD_UPDATE_NOTIFY_DISABLE='1',
                 DOTNET_CLI_USE_MSBUILD_SERVER='0',MSBUILDDISABLENODEREUSE='1',
                 DOTNET_GCHeapHardLimit='0x30000000')
        cmd=['/usr/bin/sandbox-exec','-p','(version 1)(allow default)(deny network*)',str(args.dotnet.resolve()),str(args.assembly.resolve())]
        r=subprocess.run(cmd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
        (args.out/'raw.json').write_bytes(r.stdout);(args.out/'stderr.log').write_bytes(r.stderr)
        if r.returncode:
            raise ReceiptRefusal('offline producer process failed')
        summary=validate(decode_receipt(r.stdout))
        summary.update(raw_sha256=hashlib.sha256(r.stdout).hexdigest(),assembly_sha256=hashlib.sha256(args.assembly.read_bytes()).hexdigest(),
                       dotnet_sha256=hashlib.sha256(args.dotnet.read_bytes()).hexdigest(),network='sandbox-exec deny network*',
                       process_timeout_seconds=30,heap_limit_bytes=768*1024**2,services_started=False,jobs_executed=False)
        write_summary(args.out/'receipt.json',summary)
    except (ReceiptRefusal, OSError, subprocess.TimeoutExpired) as exc:
        reason=str(exc) if isinstance(exc,ReceiptRefusal) else 'producer or receipt I/O failure'
        print('receipt_refused: '+reason,file=sys.stderr)
        return 1
    print(json.dumps(summary))
    return 0


if __name__=='__main__':
    sys.exit(main())
