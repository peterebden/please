subinclude("//build_defs:docker")
subinclude("//build_defs:k8s")

go_binary(
    name = "elan",
    srcs = ["main.go"],
    visibility = ["PUBLIC"],
    static = True,
    deps = [
        "//src/cli",
        "//src/remote/proto:remote",
        "//third_party/go:logging",
        "//tools/elan/grpc",
        "//tools/elan/http",
        "//tools/elan/storage",
    ],
)

docker_image(
    name = "image",
    srcs = [":elan"],
    image = "thoughtmachine/elan",
)

k8s_config(
    name = "k8s_statefulset",
    srcs = ["elan-statefulset.yaml", "elan-svc.yaml", "storageclass.yaml"],
    containers = [":image"],
)
