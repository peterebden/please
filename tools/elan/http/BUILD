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
        "//third_party/go:logging",
        "//tools/elan/proto:cluster",
    ],
)
