go_library(
    name = "worker",
    srcs = ["worker.go"],
    visibility = ["//tools/mettle/..."],
    deps = [
        "//src/cli",
        "//src/core",
        "//src/fs",
        "//src/grpcutil",
        "//src/remote/proto:remote",
        "//src/remote/fsclient",
        "//third_party/go:errgroup",
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:logging",
        "//tools/mettle/proto:worker",
    ],
)

go_test(
    name = "worker_test",
    srcs = ["worker_test.go"],
    deps = [
        ":worker",
        "//src/fs",
        "//src/remote/proto:remote",
        "//third_party/go:grpc",
        "//third_party/go:testify",
        "//tools/mettle/proto:worker",
    ],
)
