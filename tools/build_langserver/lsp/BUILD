go_library(
    name = "lsp",
    srcs = glob(
        ["*.go"],
        exclude = ["*_test.go"],
    ),
    deps = [
        "//src/parse/asp",
        "//src/core",
        "//third_party/go:lsp",
        "//third_party/go:logging",
        "//third_party/go:jsonrpc2",
    ],
)

go_test(
    name = "lsp_test",
    srcs = ["lsp_test.go"],
    deps = [
        ":lsp",
        "//third_party/go:lsp",
        "//third_party/go:jsonrpc2",
        "//third_party/go:testify",
    ],
    data = ["test_data"],
)
