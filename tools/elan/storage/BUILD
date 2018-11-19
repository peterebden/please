go_library(
    name = "storage",
    srcs = ["storage.go"],
    visibility = ["//tools/elan/..."],
    deps = [
        "//third_party/go:logging",
        "//third_party/go:protobuf",
        "//tools/elan/proto:cluster",
    ],
)

go_test(
    name = "storage_test",
    srcs = ["storage_test.go"],
    deps = [
        ":storage",
        "//third_party/go:testify",
    ],
)
