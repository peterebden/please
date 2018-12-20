go_library(
    name = "cluster",
    srcs = ["cluster.go", "ring.go"],
    visibility = ["//tools/elan/..."],
    deps = [
        "//third_party/go:logging",
        "//third_party/go:memberlist",
        "//third_party/go:protobuf",
        "//src/remote/proto:fs",
        "//tools/elan/proto:cluster",
        "//third_party/go:go-multierror",
        "//src/grpcutil",
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
