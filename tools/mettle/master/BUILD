go_library(
    name = "master",
    srcs = ["master.go"],
    visibility = ["//tools/mettle:all"],
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
    deps = [
        ":master",
        "//src/grpcutil",
        "//src/remote/proto:remote",
        "//third_party/go:testify",
        "//tools/mettle/proto:worker",
    ],
)
