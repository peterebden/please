go_library(
    name = "fsclient",
    srcs = [
        "client.go",
        "interface.go",
    ],
    visibility = ["PUBLIC"],
    deps = [
        "//src/fs",
        "//src/grpcutil",
        "//src/remote/proto:fs",
        "//third_party/go:errgroup",
        "//third_party/go:go-multierror",
        "//third_party/go:logging",
        "//third_party/go:xxhash",
    ],
)

go_test(
    name = "fsclient_test",
    srcs = ["fsclient_test.go"],
    visibility = ["PUBLIC"],
    deps = [
        ":fsclient",
        "//src/grpcutil",
        "//third_party/go:grpc",
        "//third_party/go:testify",
    ],
)
