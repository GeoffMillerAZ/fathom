# main.py exercises a NAMED cross-file import (run -> pkg.util.helper) and
# a same-file module-scope call (run -> local).
from pkg.util import helper


def run():
    return helper() + local()


def local():
    return 3
