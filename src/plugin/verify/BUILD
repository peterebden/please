subinclude("//build_defs:plugin")

plugin(
    name = "verify",
    srcs = ["main.go"],
    deps = [
        "//src/cli/logging",
        "///third_party/go/github.com_sigstore_sigstore//pkg/cryptoutils",
        "///third_party/go/github.com_sigstore_sigstore//pkg/signature",
    ],
    visibility = ["//src/plugin/..."],
)
