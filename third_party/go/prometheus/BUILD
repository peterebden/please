go_get(
    name = "prometheus",
    get = "github.com/prometheus/client_golang/prometheus",
    revision = "v1.1.0",
    deps = [
        ":client_model",
        ":procfs",
        ":prometheus_common",
        ":perks",
        "//third_party/go:protobuf",
        "//third_party/go:net",
    ],
    visibility = ["//tools/..."],
)

go_get(
    name = "grpc-prometheus",
    get = "github.com/grpc-ecosystem/go-grpc-prometheus",
    revision = "v1.2.0",
    deps = [
        ":prometheus",
        "//third_party/go:grpc",
        "//third_party/go:net",
    ],
    visibility = ["//tools/..."],
)

go_get(
    name = "procfs",
    get = "github.com/prometheus/procfs/...",
    revision = "v0.0.5",
    deps = [
        "//third_party/go:errgroup",
    ],
)

go_get(
    name = "perks",
    get = "github.com/beorn7/perks/...",
    revision = "v1.0.1",
)

go_get(
    name = "client_model",
    get = "github.com/prometheus/client_model/...",
    revision = "14fe0d1b01d4d5fc031dd4bec1823bd3ebbe8016",
    deps = [
        "//third_party/go:protobuf",
    ],
)

go_get(
    name = "prometheus_common",
    get = "github.com/prometheus/common",
    install = [
        "expfmt",
        "model",
        "internal/...",
    ],
    revision = "v0.7.0",
    deps = [
        ":client_model",
        ":golang_protobuf_extensions",
        "//third_party/go:protobuf",
    ],
)

go_get(
    name = "golang_protobuf_extensions",
    get = "github.com/matttproud/golang_protobuf_extensions/...",
    revision = "v1.0.1",
    deps = [
        "//third_party/go:protobuf",
    ],
)

go_get(
    name = "kingpin",
    get = "gopkg.in/alecthomas/kingpin.v2",
    revision = "v2.2.6",
    deps = [
        ":template",
        ":units",
    ],
)

go_get(
    name = "units",
    get = "github.com/alecthomas/units",
    revision = "2efee857e7cfd4f3d0138cc3cbb1b4966962b93a",
)

go_get(
    name = "template",
    get = "github.com/alecthomas/template",
    revision = "a0175ee3bccc567396460bf5acd36800cb10c49c",
)
