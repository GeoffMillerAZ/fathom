#!/usr/bin/env python3
"""Opt-in real-PTY checks. No terminal emulator, dependency installation or live data."""
import argparse
import copy
import errno
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

MAX_BYTES = 1024 * 1024
WAIT_SECONDS = 6
CASES = ('keyboard-mouse-resize', 'bidi-filter', 'small-viewport', 'ctrl-c-byte', 'sigint', 'sigterm', 'context-deadline')
BIDI_CONTROLS = '\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069'


def configuration(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, help='absolute path to the built appkit-terminal command')
    parser.add_argument('--deadline-binary', required=True, help='absolute path to the built deadline fixture')
    parser.add_argument('--case', choices=CASES, help='run one named diagnostic case instead of the full matrix')
    parser.add_argument('--repeat', type=int, default=1, help='repeat the selected cases in fresh PTYs (1-50; default 1)')
    parser.add_argument('--inject-echo-drift', action='store_true', help='negative canary for --case ctrl-c-byte; deliberately corrupt only the owned test PTY and expect exit 1')
    args = parser.parse_args(argv)
    if not 1 <= args.repeat <= 50:
        parser.error('repeat must be between 1 and 50')
    if args.inject_echo_drift and args.case != 'ctrl-c-byte':
        parser.error('echo-drift canary requires the explicit --case ctrl-c-byte')
    for path in (args.binary, args.deadline_binary):
        if not os.path.isabs(path) or not os.path.isfile(path) or not os.access(path, os.X_OK):
            parser.error('both binaries must be explicit absolute executable regular files; build the documented source first')
    if sys.platform not in ('darwin', 'linux'):
        parser.error('this termios probe requires macOS or Linux; unsupported hosts are not passes')
    return args


def attrs_json(attrs):
    """Preserve every termios field, including PENDIN and control characters."""
    names = ('iflag', 'oflag', 'cflag', 'lflag', 'ispeed', 'ospeed', 'cc')
    values = list(attrs)
    values[6] = [value[0] if isinstance(value, bytes) else value for value in attrs[6]]
    return dict(zip(names, values))


class Probe:
    def __init__(self, binary, args, root, name):
        self.name = name
        self.root = root
        self.master, self.slave = pty.openpty()
        self.resize(80, 24)
        self.before = copy.deepcopy(termios.tcgetattr(self.slave))
        self.raw = bytearray()
        self.cursor = 0
        self.result = {'name': name, 'assertions': [], 'before': attrs_json(self.before)}
        env = dict(os.environ, TERM='xterm-256color', NO_COLOR='1')
        # A fresh session owns only this newly allocated PTY. No existing terminal
        # or process group is targeted by test input, resize or signal delivery.
        def controlling_terminal():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)
            os.tcsetpgrp(0, os.getpgrp())
            # macOS refuses tcgetpgrp from the parent outside this tty's session.
            # Query inside the owning child and retain the exact handshake bytes.
            owner = f'APPKIT_PTY_OWNER {os.getpid()}:{os.getpgrp()}:{os.getsid(0)}:{os.tcgetpgrp(0)}\n'
            os.write(1, owner.encode())
        try:
            self.child = subprocess.Popen([sys.executable, str(Path(__file__).with_name('session.py')), binary, *args], stdin=self.slave, stdout=self.slave,
                                          stderr=self.slave, env=env, preexec_fn=controlling_terminal)
            self.result['child_pid'] = self.child.pid
            self.result['process_group'] = os.getpgid(self.child.pid)
            self.result['session_id'] = os.getsid(self.child.pid)
        except BaseException:
            os.close(self.master)
            os.close(self.slave)
            raise

    def check(self, ok, name, detail=None):
        self.result['assertions'].append({'name': name, 'pass': bool(ok), 'detail': detail})
        if not ok:
            raise AssertionError(f'{self.name}: {name}: {detail}')

    def pump(self, timeout=0.1):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                chunk = os.read(self.master, 65536)
            except OSError as error:
                if error.errno == errno.EIO:
                    return
                raise
            self.raw.extend(chunk)
            if len(self.raw) > MAX_BYTES:
                del self.raw[MAX_BYTES:]
                raise RuntimeError('PTY transcript exceeded the declared 1 MiB bound')

    def drain(self):
        while select.select([self.master], [], [], 0)[0]:
            prior = len(self.raw)
            self.pump(0)
            if len(self.raw) == prior:
                break

    def expect(self, text, label=None):
        needles = text if isinstance(text, tuple) else (text,)
        needles = [item.encode() if isinstance(item, str) else item for item in needles]
        until = time.monotonic() + WAIT_SECONDS
        while True:
            for needle in needles:
                where = self.raw.find(needle, self.cursor)
                if where >= 0:
                    self.cursor = where + len(needle)
                    self.check(True, label or f'emitted {text!r}')
                    return
            if time.monotonic() >= until:
                self.check(False, label or f'emitted {text!r}', 'not observed before bounded deadline')
            self.pump()

    def send(self, data):
        # Start the oracle after already observed output, never match an old frame.
        self.drain()
        self.cursor = len(self.raw)
        os.write(self.master, data)

    def metadata(self, kind):
        pattern = re.compile(rb'APPKIT_PTY_' + kind.encode() + rb' ([^\r\n]+)[\r\n]')
        until = time.monotonic() + WAIT_SECONDS
        while True:
            match = pattern.search(self.raw)
            if match:
                return json.loads(match.group(1))
            if time.monotonic() >= until:
                self.check(False, f'session metadata {kind}', 'not observed before bounded deadline')
            self.pump()

    def resize(self, columns, rows):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack('HHHH', rows, columns, 0, 0))
        if hasattr(self, 'app_pid') and self.child.poll() is None:
            os.kill(self.app_pid, signal.SIGWINCH)

    def begin_shutdown(self):
        self.drain()
        self.shutdown_cursor = len(self.raw)

    def check_cleanup(self):
        # F2 and initialization also emit mode sequences. Only shutdown output
        # can prove shutdown cleanup, never an earlier frame in the transcript.
        end = self.raw.find(b'APPKIT_PTY_EXIT ', self.shutdown_cursor)
        self.check(end >= self.shutdown_cursor, 'application exit marker follows shutdown boundary')
        tail = self.raw[self.shutdown_cursor:end]
        for sequence, label in ((b'\x1b[?1049l', 'alternate screen exit'),
                                (b'\x1b[?25h', 'cursor show'),
                                (b'\x1b[?2004l', 'bracketed paste disabled'),
                                (b'\x1b[?1002l', 'cell mouse disabled')):
            self.check(sequence in tail, label)

    def finish(self, expected_exit=0, inject_echo_drift=False):
        code = self.metadata('EXIT')
        self.result['exit_code'] = code
        self.check(code == expected_exit, 'application exits within bound with expected code', code)
        self.result['immediate'] = self.metadata('IMMEDIATE')
        self.result['shutdown_offset'] = self.shutdown_cursor
        self.check_cleanup()
        # This is an actual canonical line read, not masking a flag or calling
        # tcsetattr to repair a broken program. Preserve immediate state separately.
        self.metadata('READ_READY')
        if inject_echo_drift:
            attrs = termios.tcgetattr(self.slave)
            attrs[3] ^= termios.ECHO
            termios.tcsetattr(self.slave, termios.TCSANOW, attrs)
            self.result['injected_fault'] = 'toggled ECHO on this owned test PTY after app exit'
        os.write(self.master, b'restoration-check\n')
        line = self.metadata('LINE')
        self.check(line == b'restoration-check\n'.hex(), 'canonical line content restored', line)
        self.result['after_read'] = self.metadata('AFTER_READ')
        self.result['parent_after_read'] = attrs_json(termios.tcgetattr(self.slave))
        os.write(self.master, b'keeper-exit\n')
        # Drain echo/final output while the session leader tears down its tty;
        # macOS can wait for pending tty output when the controlling session exits.
        until = time.monotonic() + WAIT_SECONDS
        while self.child.poll() is None and time.monotonic() < until:
            self.pump(0.05)
        self.child.wait(timeout=0)
        self.check(self.child.returncode == 0, 'session keeper exits cleanly', self.child.returncode)
        self.check(self.result['parent_after_read'] == self.result['after_read'], 'independent parent termios observation agrees')
        self.check(self.result['after_read'] == self.result['before'], 'all termios fields restored exactly after canonical read')

    def owned_process_alive(self):
        if self.child.poll() is None:
            return True
        # A failed keeper must not hide a surviving app. Only signal when a
        # recorded process still belongs to this exact owned session and group.
        try:
            return (hasattr(self, 'app_pid') and os.getpgid(self.app_pid) == self.child.pid
                    and os.getsid(self.app_pid) == self.child.pid)
        except ProcessLookupError:
            return False

    def close(self):
        try:
            if self.owned_process_alive():
                self.result['forced_cleanup'] = True
                # An observed keeper or application still anchors group identity;
                # never signal a bare numeric PGID after both are gone.
                os.killpg(self.child.pid, signal.SIGCONT)
                os.killpg(self.child.pid, signal.SIGTERM)
                until = time.monotonic() + 2
                while self.owned_process_alive() and time.monotonic() < until:
                    self.pump(0.05)
                if self.owned_process_alive():
                    os.killpg(self.child.pid, signal.SIGKILL)
                    self.child.wait(timeout=5)
            try:
                os.killpg(self.child.pid, 0)
            except ProcessLookupError:
                pass
            else:
                # The reference starts no descendants. Preserve an unexpected
                # survivor as a failure rather than target an unanchored group.
                self.result['cleanup_error'] = 'process group still present after owned keeper/application cleanup'
        except Exception as error:
            self.result['cleanup_error'] = str(error)
        finally:
            try:
                self.drain()
            except Exception as error:
                self.result['cleanup_error'] = str(error)
            data = bytes(self.raw)
            self.result['transcript_sha256'] = hashlib.sha256(data).hexdigest()
            self.result['transcript_bytes'] = len(data)
            try:
                (self.root / f'{self.name}.pty').write_bytes(data)
            except Exception as error:
                self.result['cleanup_error'] = str(error)
            for fd in (self.master, self.slave):
                try:
                    os.close(fd)
                except OSError as error:
                    self.result['cleanup_error'] = str(error)


def journey(probe):
    probe.expect('Selected: analytics.comparison')
    probe.send(b'\r')
    probe.expect('analytics.comparison | experimental')
    probe.expect('Detail lines 1-16')
    probe.send(b'\x1b')
    probe.expect('Selected: analytics.comparison')
    # First filtered row is y=6; 80-column [Open] occupies columns 75–80.
    probe.send(b'\x1b[<0;77;6M\x1b[<0;77;6m')
    probe.expect('analytics.comparison | experimental', 'mouse opens same exact ID as Enter')
    probe.pump(0)
    probe.cursor = len(probe.raw)
    probe.resize(60, 14)
    probe.expect('Detail lines 1-6', 'resize rewraps current detail at 60x14')
    probe.send(b'\x1b[<65;10;6M')
    # The pinned renderer may emit a cursor-addressed patch rather than repeat
    # the unchanged label. Assert that exact patch, then confirm on a full redraw.
    probe.expect(('Detail lines 4-9', b'\x1b[5;14H4-9 '), 'mouse wheel emits exact three-line scroll update')
    probe.resize(61, 14)
    probe.expect('Detail lines 4-9', 'full resize redraw confirms retained wheel position')
    probe.send(b'\x1b[6~')  # xterm Page Down: six visible detail rows at height 14.
    probe.resize(60, 14)
    probe.expect('Detail lines 10-15', 'Page Down advances exactly one visible detail page')
    probe.send(b'\x1b[5~')  # xterm Page Up must return to the wheel-selected position.
    probe.resize(61, 14)
    probe.expect('Detail lines 4-9', 'Page Up returns exactly one visible detail page')
    probe.send(b'\x1bOQ')  # xterm F2
    probe.expect(b'\x1b[?1002l', 'mouse disable protocol emitted after F2')
    probe.resize(60, 14)
    probe.expect('[F2 Mouse off]', 'full resize redraw confirms F2 capture opt-out')
    probe.send(b'\x1bOQ')
    probe.expect(b'\x1b[?1002h', 'keyboard re-enables mouse capture')
    probe.resize(61, 14)
    probe.expect('[F2 Mouse on]', 'full resize redraw confirms capture on before shutdown')
    probe.send(b'\x1b')
    probe.expect('Selected: analytics.comparison', 'Back preserves exact selection')
    probe.begin_shutdown()
    probe.send(b'q')


def bidi_filter_journey(probe):
    probe.expect('Selected: analytics.comparison')
    probe.send(b'/\x15')  # Edit, then clear with Ctrl+U.
    probe.expect('> Filter: _')
    payload = ('forms' + BIDI_CONTROLS + ' validation').encode()
    probe.send(b'\x1b[200~' + payload + b'\x1b[201~')
    input_start = probe.cursor
    probe.expect('Input sanitized or filter limit reached (256 bytes).')
    # Confirm state on a new full redraw rather than matching an old/delta frame.
    probe.cursor = len(probe.raw)
    probe.resize(81, 24)
    probe.expect('> Filter: forms validation_', 'all direction controls removed from native pasted filter')
    probe.expect('Selected: interaction.forms', 'sanitized native paste resolves the intended capability')
    probe.check(not any(control.encode() in probe.raw[input_start:] for control in BIDI_CONTROLS),
                'no direction-control bytes emitted after native paste')
    probe.send(b'\r')  # Finish editing, then use the ordinary inspection path.
    probe.resize(80, 24)
    probe.expect('Filter: forms validation')
    probe.expect('Selected: interaction.forms')
    probe.send(b'\r')
    probe.expect('interaction.forms |', 'Enter inspects the sanitized search result')
    probe.send(b'\x1b')
    probe.expect('Selected: interaction.forms', 'Back preserves the sanitized selection')
    probe.send(b'/\x15')
    probe.expect('> Filter: _')
    ordinary = 'مرحبا שלום e\u0301 👩\u200d👩\u200d👧\u200d👦 می\u200cروم'
    probe.send(b'\x1b[200~' + ordinary.encode() + b'\x1b[201~')
    probe.expect(ordinary, 'native multilingual paste retains letters, combining marks and join controls')
    probe.cursor = len(probe.raw)
    probe.resize(81, 24)
    probe.expect('> Filter: ' + ordinary + '_', 'full redraw retains exact multilingual query bytes')
    probe.send(b'\r')  # q only quits outside editing.
    probe.resize(80, 24)
    probe.expect('Filter: ' + ordinary)
    probe.begin_shutdown()
    probe.send(b'q')


def small_viewport_journey(probe):
    probe.expect('Selected: analytics.comparison')
    probe.cursor = len(probe.raw)
    probe.resize(39, 9)
    probe.expect(b'\x1b[?1002l', 'undersized viewport releases mouse capture')
    probe.expect('Mouse released here. F2 preference: on', 'undersized fallback discloses capture preference')
    probe.cursor = len(probe.raw)
    probe.resize(80, 24)
    probe.expect(b'\x1b[?1002h', 'usable viewport restores the chosen capture preference')
    probe.expect('Selected: analytics.comparison', 'resize preserves selected capability')
    for key, label, width, mode, protocol in (
            (b'\x1bq', 'Alt+q', 81, 'off', b'\x1b[?1002l'),
            (b'\x11', 'Ctrl+q', 80, 'on', b'\x1b[?1002h')):
        probe.send(key + b'\x1bOQ')  # Following F2 must still reach the running model.
        probe.expect(protocol, label + ' leaves F2 usable')
        probe.cursor = len(probe.raw)
        probe.resize(width, 24)
        probe.expect('[F2 Mouse ' + mode + ']', label + ' retains a live exact-state redraw')
        probe.expect('Selected: analytics.comparison')
    probe.cursor = len(probe.raw)
    probe.resize(80, 9)
    probe.expect(b'\x1b[?1002l')
    probe.send(b'\x1bOQ')  # Change preference while capture is already suspended.
    probe.resize(81, 9)
    probe.expect('Mouse released here. F2 preference: off', 'F2 can disable preference while undersized')
    probe.cursor = len(probe.raw)
    probe.resize(80, 24)
    probe.expect('[F2 Mouse off]', 'usable resize honors the changed preference')
    probe.expect('Selected: analytics.comparison')
    probe.send(b'\x1bOQ')
    probe.expect(b'\x1b[?1002h', 'capture restored before testing shutdown cleanup')
    probe.begin_shutdown()
    probe.send(b'q')


def main(argv=None):
    args = configuration(sys.argv[1:] if argv is None else argv)
    root = Path(tempfile.mkdtemp(prefix='appkit-terminal-pty-'))
    source_root = Path(__file__).absolute().parent
    report = {'platform': platform.platform(), 'machine': platform.machine(), 'python': platform.python_version(), 'repeat': args.repeat,
              'binaries': {path: hashlib.sha256(Path(path).read_bytes()).hexdigest() for path in (args.binary, args.deadline_binary)},
              'probe_sources': {name: hashlib.sha256((source_root / name).read_bytes()).hexdigest()
                                for name in ('run.py', 'session.py', 'deadline/main.go')},
              'cases': [], 'failures': [], 'limits': ['Actual PTY bytes and termios, not a graphical terminal emulator or physical input device.',
                  'NO_COLOR xterm-256color fixture; no terminal background reply or theme-polarity claim.',
                  'SIGTERM current observed command contract remains zero; deadline fixture zero means expected cancellation assertions passed.',
                  'No consumer integration, screen-reader, clipboard or Windows claim.']}
    print(f'PTY evidence: {root}', flush=True)
    selected = [args.case] if args.case else CASES
    cases = [(case, iteration) for iteration in range(1, args.repeat + 1) for case in selected]
    for case, iteration in cases:
        name = case if args.repeat == 1 else f'{case}-{iteration:03d}'
        probe = None
        try:
            binary = args.deadline_binary if case == 'context-deadline' else args.binary
            options = [] if case == 'context-deadline' else ['--mouse', '--query', 'analytics comparison']
            probe = Probe(binary, options, root, name)
            probe.result.update(case=case, iteration=iteration)
            pid = probe.child.pid
            probe.expect(f'APPKIT_PTY_OWNER {pid}:{pid}:{pid}:{pid}', 'fresh child owns foreground session and PTY')
            probe.result['foreground_group'] = pid
            before = probe.metadata('BEFORE')
            probe.check(before == probe.result['before'], 'session setup preserves all initial termios fields')
            probe.app_pid = probe.metadata('PROCESS')
            probe.result['application_pid'] = probe.app_pid
            probe.check(os.getpgid(probe.app_pid) == pid, 'application belongs to the owned foreground process group')
            probe.expect('app-kit / terminal catalog')
            if case == 'keyboard-mouse-resize':
                journey(probe)
            elif case == 'bidi-filter':
                bidi_filter_journey(probe)
            elif case == 'small-viewport':
                small_viewport_journey(probe)
            elif case == 'ctrl-c-byte':
                probe.begin_shutdown()
                probe.send(b'\x03')
            elif case in ('sigint', 'sigterm'):
                probe.begin_shutdown()
                os.kill(probe.app_pid, signal.SIGINT if case == 'sigint' else signal.SIGTERM)
            else:
                probe.begin_shutdown()
                probe.expect('APPKIT_PTY_DEADLINE_PROVEN')
            # OS SIGINT is runner interruption (2); typed Ctrl+C is the model's
            # explicit Quit (0). Never equate the byte with the operating-system signal.
            probe.finish(expected_exit=2 if case == 'sigint' else 0, inject_echo_drift=args.inject_echo_drift)
        except (Exception, KeyboardInterrupt) as error:
            report['failures'].append({'case': name, 'error': str(error)})
            print(f'{name}: {error}', file=sys.stderr, flush=True)
        finally:
            if probe is not None:
                probe.close()
                report['cases'].append(probe.result)
                if probe.result.get('forced_cleanup') or probe.result.get('cleanup_error'):
                    report['failures'].append({'case': name, 'error': 'forced cleanup or child reaping failure; inspect case details'})
    report['ok'] = not report['failures'] and len(report['cases']) == len(cases)
    report['assertions_passed'] = sum(check['pass'] for case in report['cases'] for check in case['assertions'])
    (root / 'receipt.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'ok': report['ok'], 'assertions_passed': report['assertions_passed'], 'failures': report['failures'], 'receipt': str(root / 'receipt.json')}, indent=2))
    return 0 if report['ok'] else 1


if __name__ == '__main__':
    sys.exit(main())
