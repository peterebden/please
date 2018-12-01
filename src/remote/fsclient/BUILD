go_library(
    name = "fsclient",
    srcs = ["client.go", "interface.go"],
    visibility = ["PUBLIC"],
    deps = [
        "//src/grpcutil",
        "//third_party/go:errgroup",
        "//third_party/go:logging",
        "//third_party/go:go-multierror",
        "//third_party/go:xxhash",
        "//src/remote/proto:fs",
    ],
)

go_test(
    name = "fsclient_test",
    srcs = ["fsclient_test.go"],
    visibility = ["PUBLIC"],
    deps = [
        ":fsclient",
        "//third_party/go:testify",
        "//third_party/go:grpc",
        "//src/grpcutil",
    ],
)
