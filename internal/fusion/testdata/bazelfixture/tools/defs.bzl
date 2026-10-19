load(":helpers.bzl", "join")

def my_macro(name, deps = []):
    join(name)
    native.cc_library(name = name, deps = deps)

def other_macro(name):
    join(name)
