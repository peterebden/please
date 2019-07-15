go_library(
    name = "remote",
    srcs = glob(
        ["*.go"],
        exclude = ["*_test.go"],
    ),
    deps = [
        "//src/core",
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:logging",
        "//third_party/go:protobuf",
        "//third_party/go:remote-apis",
    ],
)

go_test(
    name = "remote_test",
    srcs = ["remote_test.go"],
    deps = [
        ":remote",
        "//src/core",
        "//third_party/go:grpc",
        "//third_party/go:remote-apis",
        "//third_party/go:testify",
    ],
)
