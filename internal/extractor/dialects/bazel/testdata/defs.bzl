"""Rules and macros."""

load("//tools:helpers.bzl", "join")
load(":local.bzl", inner = "local_rule")

def _impl(ctx):
    return [DefaultInfo()]

my_rule = rule(
    implementation = _impl,
    attrs = {"src": attr.label()},
)

tool_ext = module_extension(implementation = _impl)

def my_macro(
        name,
        deps = [],
        **kwargs):
    """A macro."""
    inner(name = name + "_inner")
    join(name)
    native.cc_library(
        name = name,
        deps = deps,
    )

def helper(x):
    return x
