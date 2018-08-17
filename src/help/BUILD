go_bindata(
    name = "data",
    srcs = [
        "misc.json",
        ":config_json",
    ],
    compress = True,
)

go_library(
    name = "help",
    srcs = [
        "completion.go",
        "help.go",
        ":data",
    ],
    visibility = ["PUBLIC"],
    deps = [
        "//src/cli",
        "//src/parse",
        "//src/utils",
        "//third_party/go:go-flags",
        "//third_party/go:logging",
    ],
)

go_test(
    name = "help_test",
    srcs = ["help_test.go"],
    deps = [
        ":help",
        "//third_party/go:testify",
    ],
)

genrule(
    name = "config_json",
    outs = ["config.json"],
    cmd = "$TOOL > $OUTS",
    tools = ["//src/help/config:config_help"],
)
