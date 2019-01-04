go_library(
    name = "storage",
    srcs = ["storage.go"],
    visibility = ["//tools/elan/..."],
    deps = [
        "//src/fs",
        "//src/remote/proto:fs",
        "//third_party/go:logging",
        "//third_party/go:protobuf",
        "//third_party/go:xattr",
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
