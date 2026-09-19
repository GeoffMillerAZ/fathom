# pkg/util.py — the SU-11 fixture's dependency module: two module-scope
# functions, consumed both by a "from pkg.util import helper" named import
# and by a whole-module "import pkg.util" dotted member access.
def helper():
    return 1


def other():
    return 2
