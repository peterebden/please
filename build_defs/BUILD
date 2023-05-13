filegroup(
    name = "benchmark",
    srcs = ["benchmark.build_defs"],
    visibility = ["PUBLIC"],
)

filegroup(
    name = "plugin",
    srcs = ["plugin.build_defs"],
    visibility = ["//src/plugin/..."],
)

genrule(
    name = "version",
    srcs = ["//:version"],
    outs = ["version.build_defs"],
    cmd = "echo \"VERSION = '`cat $SRCS`'\" > \"$OUT\"",
    visibility = ["PUBLIC"],
)
