go_library(
    name = "prometheus",
    srcs = ["prometheus.go"],
    visibility = ["//src/plugin/prometheus/..."],
    deps = [
        "///third_party/go/github.com_prometheus_client_golang//prometheus",
        "///third_party/go/github.com_prometheus_client_golang//prometheus/push",
        "///third_party/go/github.com_prometheus_common//expfmt",
        "//src/cli/logging",
        "//src/core",
        "//src/metrics",
    ],
)
