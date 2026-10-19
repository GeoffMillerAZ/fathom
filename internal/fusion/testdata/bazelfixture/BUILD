load("//tools:defs.bzl", "my_macro", renamed = "other_macro")
load(":local.bzl", "local_rule")
load("@rules_cc//cc:defs.bzl", "cc_library")

cc_library(
    name = "core",
    deps = [":plain"],
)

my_macro(
    name = "gen",
    deps = [":core"],
)

renamed(name = "aliased")

local_rule(name = "plain")
