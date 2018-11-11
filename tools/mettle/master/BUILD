go_library(
    name = "master",
    srcs = ["master.go"],
    visibility = ["//tools/remote_server:all"],
    deps = [
        "//src/grpcutil",
        "//src/remote/proto:remote",
        "//third_party/go:grpc",
        "//third_party/go:logging",
        "//tools/mettle/proto:worker",
    ],
)

go_test(
    name = "master_test",
    srcs = ["master_test.go"],
    labels = ["manual"],  # Race conditions abound.
    deps = [
        ":master",
        "//src/grpcutil",
        "//src/remote/proto:remote",
        "//third_party/go:testify",
    ],
)
