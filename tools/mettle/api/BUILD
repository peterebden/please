go_library(
    name = "api",
    srcs = ["api.go"],
    deps = [
        "//third_party/go/gocloud",
        "//third_party/go:bytestream",
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:longrunning",
        "//third_party/go:logging",
        "//third_party/go:protobuf",
        "//third_party/go:remote-apis",
        "//third_party/go:rpcstatus",
        "//third_party/go/prometheus",
        "//third_party/go/prometheus:grpc-prometheus",
        "//tools/mettle/common",
    ],
    visibility = ["//tools/mettle/..."],
)
