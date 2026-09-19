# pkg/wrapper.py exercises a WHOLE-MODULE dotted import (import pkg.util):
# wrap -> pkg.util.other, resolved via member access on the "pkg.util"
# binding.
import pkg.util


def wrap():
    return pkg.util.other()
