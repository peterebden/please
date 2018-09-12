go_library(
    name = "master",
    srcs = ["master.go"],
    visibility = ["//tools/remote_server:all"],
    deps = [
        "//src/grpcutil",
        "//src/test/proto:remote",
        "//third_party/go:grpc",
        "//third_party/go:logging",
    ],
)

go_test(
    name = "master_test",
    srcs = ["master_test.go"],
    labels = ["manual"],  # Race conditions abound.
    deps = [
        ":master",
        "//src/grpcutil",
        "//src/test/proto:remote",
        "//third_party/go:grpc-middleware",
        "//third_party/go:testify",
    ],
)
