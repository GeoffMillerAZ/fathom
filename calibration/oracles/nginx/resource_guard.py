"""Keep new subjects and source builds above the controller's disk floor."""
import shutil
from pathlib import Path

MIN_FREE_BYTES = 20 * 1024**3


def require_free_space(operation):
    free = shutil.disk_usage(Path.home()).free
    if free < MIN_FREE_BYTES:
        raise ValueError('{} requires at least 20 GiB free; currently {:.2f} GiB'.format(
            operation, free / 1024**3))
