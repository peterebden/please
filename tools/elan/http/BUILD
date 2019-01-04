go_bindata(
    name = "static_files",
    srcs = [
        "index.html",
        "styles.css",
    ],
)

go_library(
    name = "http",
    srcs = [
        "http.go",
        ":static_files",
    ],
    visibility = ["//tools/elan/..."],
    deps = [
        "//src/grpcutil",
        "//src/remote/proto:fs",
        "//third_party/go:logging",
        "//tools/elan/proto:cluster",
    ],
)

go_test(
    name = "http_test",
    srcs = ["http_test.go"],
    deps = [
        ":http",
        "//third_party/go:testify",
    ],
)
