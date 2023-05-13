subinclude("///go//build_defs:go")

go_library(
    name = "plugin",
    srcs = ["plugin.go"],
    resources = [":plugins"],
    visibility = [
        "//src/plz/...",
        "//src/update/...",
    ],
    deps = [
        "//src/cli/logging",
        "//src/fs",
    ],
)

subrepo(
    name = "plugins",
    path = ".",
    config = "src/plugin/plzconfig.plugins",
)

filegroup(
    name = "plugins",
    srcs = [
        "///src/plugin/plugins//src/plugin/prometheus",
        "///src/plugin/plugins//src/plugin/remote",
        "///src/plugin/plugins//src/plugin/verify",
    ],
)
