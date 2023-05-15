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
    config = "src/plugin/plzconfig.plugins",
    path = ".",
)

filegroup(
    name = "plugins",
    srcs = [
        "///src/plugin/plugins//src/plugin/prometheus",
        "///src/plugin/plugins//src/plugin/remote",
        "///src/plugin/plugins//src/plugin/verify",
    ],
)

go_test(
    name = "plugin_test",
    srcs = ["plugin_test.go"],
    external = True,
    deps = [
        ":plugin",
        "///third_party/go/github.com_stretchr_testify//assert",
    ],
)
