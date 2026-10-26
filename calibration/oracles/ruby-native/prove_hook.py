#!/usr/bin/env python3
"""Prove the released runtime hook survives a subject's File.expand_path stub."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

from bootstrap import environment


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    args=ap.parse_args()
    base=args.tools.resolve()
    state=json.loads((base/'bootstrap.json').read_text())
    ruby=base/state['ruby']['binary']
    repo=Path(__file__).resolve().parents[3]
    scratch=repo/'workspace/artifacts'
    scratch.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(dir=scratch) as temp:
        root=Path(temp)
        subject=root/'subject'
        subject.mkdir()
        trace=root/'trace'
        trace.mkdir()
        script=subject/'native.rb'
        script.write_text('''raise 'premature JSON activation' if defined?(JSON)
raise 'premature Set activation' unless Object.autoload?(:Set)
class HookFixture
  def target; 'x'.length; end
  def driver
    original = File.method(:expand_path)
    File.define_singleton_method(:expand_path) { |*| nil }
    target
  ensure
    File.define_singleton_method(:expand_path, original)
  end
end
HookFixture.new.driver
''')
        env=dict(environment(base,ruby),RUBY_ORACLE_SUBJECT=str(subject),
                 RUBY_ORACLE_TRACE_DIR=str(trace),RUBYOPT='-r'+str(Path(__file__).with_name('trace_hook.rb')))
        result=subprocess.run([str(ruby),str(script)],env=env,capture_output=True,text=True)
        assert result.returncode==0,result.stdout+result.stderr
        raw=list(trace.glob('native-*.json'))
        assert len(raw)==1
        data=json.loads(raw[0].read_text())
        assert not data['refused']
        assert any(row[2:] == ['driver','target','target'] for row in data['calls'])
        assert any(row[2:] == ['target','length','length'] for row in data['calls'])
        print('PASS: native endpoints survive subject File.expand_path stub; no pre-Bundler gem activation')


if __name__=='__main__':main()
