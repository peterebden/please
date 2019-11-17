go_library(
    name = "worker",
    srcs = ["worker.go"],
    deps = [
        "//third_party/go/gocloud",
        "//third_party/go:bytestream",
        "//third_party/go:grpc",
        "//third_party/go:grpc-middleware",
        "//third_party/go:longrunning",
        "//third_party/go:logging",
        "//third_party/go:protobuf",
        "//third_party/go:remote-apis",
        "//third_party/go:remote-apis-sdks",
        "//third_party/go:rpcstatus",
        "//tools/mettle/common",
    ],
    visibility = ["//tools/mettle/..."],
)
