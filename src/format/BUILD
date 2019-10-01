go_library(
    name = "format",
    srcs = ["format.go"],
    deps = [
        "//src/parse/asp",
        "//src/core",
        "//src/cli",
        "//src/fs",
        "//third_party/go:logging",
        "//third_party/go:shlex",
    ],
    visibility = ["PUBLIC"],
)

go_test(
    name = "format_test",
    srcs = ["format_test.go"],
    deps = [
        ":format",
        "//src/core",
        "//third_party/go:testify",
    ],
    data = [
        "test_data",
        "//src/format/test_bin",
    ],
)
