go_binary(
    name = "remote_worker",
    srcs = ["main.go"],
    visibility = ["PUBLIC"],
    deps = [
        "//src/cli",
        "//src/test/proto:remote",
        "//third_party/go:logging",
        "//tools/remote_worker/master",
        "//tools/remote_worker/worker",
    ],
)
