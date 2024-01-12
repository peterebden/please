go_library(
    name = "ordmap",
    srcs = ["ordmap.go"],
    visibility = ["//src/..."],
)

go_test(
    name = "ordmap_test",
    srcs = ["ordmap_test.go"],
    external = True,
    deps = [
        ":ordmap",
        "///third_party/go/github.com_stretchr_testify//assert",
    ],
)
