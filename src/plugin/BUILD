go_library(
    name = "plugin",
    srcs = ["plugin.go"],
    resources = [":plugins"],
    visibility = ["//src/plz/..."],
    deps = [
        "//src/cli/logging",
        "//src/core",
        "//src/fs",
    ],
)

filegroup(
    name = "plugins",
    srcs = [
        "///plugins//src/plugin/prometheus",
        "///plugins//src/plugin/remote",
    ],
)
