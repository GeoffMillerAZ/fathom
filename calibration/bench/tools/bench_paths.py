"""Keep instrument-created files inside an explicit workspace directory."""
import os
from pathlib import Path


def workspace_path(value):
    path = Path(value).resolve()
    if 'workspace' not in path.parts:
        raise ValueError('instrument outputs must be under workspace/')
    return path


def job_dir():
    return workspace_path(os.environ['BENCH_JOB'])
