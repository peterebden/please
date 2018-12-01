go_library(
    name = "grpc",
    srcs = [
        "grpc.go",
        "ring.go",
    ],
    visibility = ["//tools/elan/..."],
    deps = [
        "//src/grpcutil",
        "//src/remote/proto:fs",
        "//third_party/go:go-multierror",
        "//third_party/go:grpc",
        "//third_party/go:logging",
        "//tools/elan/proto:cluster",
        "//tools/elan/storage",
    ],
)

go_test(
    name = "ring_test",
    srcs = ["ring_test.go"],
    deps = [
        ":grpc",
        "//src/remote/proto:fs",
        "//third_party/go:testify",
        "//tools/elan/proto:cluster",
    ],
)

go_test(
    name = "grpc_test",
    srcs = ["grpc_test.go"],
    deps = [
        ":grpc",
        "//src/remote/proto:fs",
        "//third_party/go:testify",
        "//tools/elan/storage",
        "//tools/elan/proto:cluster",
    ],
    flaky = True,  # Hard to be 100% confident with everything going on in there.
)
