"""Build the test-only probe against unmodified pinned Conftest source."""
import argparse,subprocess
from pathlib import Path
PIN='744c5637a97d60d9cbc06ff41881749c2dc766b9'
def git(root,*args):return subprocess.check_output(['git','-C',str(root),*args],text=True).strip()
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('-subject',required=True);p.add_argument('-out',required=True);a=p.parse_args();root=Path(a.subject)
 if git(root,'rev-parse','HEAD')!=PIN or git(root,'status','--porcelain'):raise SystemExit('native subject must be clean at '+PIN)
 probe=root/'internal/commands/fathom_config_oracle_test.go'
 if probe.exists():raise SystemExit('probe path already exists')
 probe.write_bytes(Path(__file__).with_name('native_config_test.go.txt').read_bytes())
 try:subprocess.run(['go','-C',str(root),'test','-c','-o',str(Path(a.out).resolve()),'./internal/commands'],check=True,timeout=1200)
 finally:probe.unlink()
 if git(root,'status','--porcelain'):raise SystemExit('native build left source changes')
