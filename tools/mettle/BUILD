go_binary(
    name = "mettle",
    srcs = ["main.go"],
    visibility = ["PUBLIC"],
    deps = [
        "//src/cli",
        "//src/remote/proto:remote",
        "//third_party/go:logging",
        "//tools/mettle/master",
        "//tools/mettle/worker",
    ],
)
