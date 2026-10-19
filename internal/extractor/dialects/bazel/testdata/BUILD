load("//tools:defs.bzl", "my_macro", renamed = "other_macro")
load(":local.bzl", "local_rule")
load("@rules_cc//cc:defs.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

cc_library(
    name = "core",
    srcs = ["core.cc"],
    deps = [
        ":util",
        "//third:dep",
    ],
)

cc_library(
    name = "util",
    srcs = ["util.cc"],
    hdrs = ["util.h"],
)

my_macro(
    name = "gen",
    deps = [":core"] + [":util"],
)

renamed(name = "aliased", data = select({
    "//conditions:default": [":gen"],
}))

local_rule(name = "plain", src = ":core")

package_group(name = "friends", packages = ["//..."])

selects.config_setting_group(
    name = "both",
    match_all = [":plain"],
)
