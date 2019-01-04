go_library(
    name = "cluster",
    srcs = [
        "cluster.go",
        "ring.go",
    ],
    visibility = ["//tools/elan/..."],
    deps = [
        "//src/grpcutil",
        "//src/remote/proto:fs",
        "//third_party/go:go-multierror",
        "//third_party/go:logging",
        "//third_party/go:memberlist",
        "//third_party/go:protobuf",
        "//tools/elan/proto:cluster",
    ],
)

go_test(
    name = "ring_test",
    srcs = ["ring_test.go"],
    deps = [
        ":cluster",
        "//src/remote/proto:fs",
        "//third_party/go:testify",
        "//tools/elan/proto:cluster",
    ],
)
