go_binary(
    name = "please_go_build",
    srcs = ["main.go"],
)

go_binary(
    name = "bootstrap",
    srcs = ["bootstrap_main.go"],
    deps = ["//tools/please_go_build/get"],
)
