go_binary(
    name = "remote_server",
    srcs = ["main.go"],
    visibility = ["PUBLIC"],
    deps = [
        "//src/cli",
        "//src/test/proto:remote",
        "//third_party/go:logging",
        "//tools/remote_server/master",
        "//tools/remote_server/worker",
    ],
)
