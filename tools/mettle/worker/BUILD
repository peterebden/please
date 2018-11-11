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
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:logging",
    ],
)
