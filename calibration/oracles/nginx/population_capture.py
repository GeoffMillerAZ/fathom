"""Capture complete successful native-parser profiles before scratch deletion.

No scanner references are accepted here. The measurement adapter must still
classify the captured native population and run all protocol controls.
"""
from pathlib import Path, PurePosixPath
from population import decode_events


class NativeCapture:
    def __init__(self, target):
        self.target = target
        self.failed_contexts = 0
        self._selected = None

    def __call__(self, record):
        if record['target'] != self.target:
            raise ValueError('native callback target mismatch')
        if record['returncode'] != 0:
            self.failed_contexts += 1
            return
        if self._selected is not None:
            raise ValueError('expected exactly one successful native profile')
        directory = Path(record['directory']).resolve()
        prefix = PurePosixPath(record['prefix'])
        if not directory.is_dir() or prefix.is_absolute() or '..' in prefix.parts:
            raise ValueError('invalid or already removed native profile')
        events = decode_events(record['stderr'])
        for event in events:
            path = Path(event['path'])
            resolved = (path if path.is_absolute() else directory / path).resolve()
            try:
                relative = resolved.relative_to(directory)
            except ValueError as error:
                raise ValueError('native source leaves isolated profile') from error
            if relative.as_posix() == '_wrapper.conf':
                event['path'] = '@context-wrapper'
            else:
                event['path'] = (prefix / relative.as_posix()).as_posix()
        self._selected = dict(events=events, target=self.target, mode=record['mode'])

    def selected(self):
        if self._selected is None:
            raise ValueError('expected exactly one successful native profile')
        return self._selected
