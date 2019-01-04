go_library(
    name = "grpc",
    srcs = [
        "fan.go",
        "grpc.go",
    ],
    visibility = ["//tools/elan/..."],
    deps = [
        "//src/grpcutil",
        "//src/remote/proto:fs",
        "//third_party/go:go-multierror",
        "//third_party/go:grpc",
        "//third_party/go:logging",
        "//tools/elan/cluster",
        "//tools/elan/proto:cluster",
        "//tools/elan/storage",
    ],
)

go_test(
    name = "grpc_test",
    srcs = ["grpc_test.go"],
    flaky = True,  # Hard to be 100% confident with everything going on in there.
    labels = ["manual"],
    deps = [
        ":grpc",
        "//src/remote/proto:fs",
        "//third_party/go:testify",
        "//tools/elan/proto:cluster",
        "//tools/elan/storage",
    ],
)

go_test(
    name = "fan_test",
    srcs = ["fan_test.go"],
    deps = [
        ":grpc",
        "//src/remote/proto:fs",
        "//third_party/go:testify",
    ],
)
