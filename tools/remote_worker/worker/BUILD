go_library(
    name = "worker",
    srcs = ["worker.go"],
    visibility = ["PUBLIC"],
    deps = [
        "//src/build",
        "//src/cli",
        "//src/core",
        "//src/fs",
        "//src/grpcutil",
        "//src/test/proto:remote",
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:logging",
    ],
)
