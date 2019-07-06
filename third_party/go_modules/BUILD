package(default_visibility = ["PUBLIC"])

# This is needed to break a circular dependency.
package(jarcat_tool = "//tools/jarcat:jarcat_unzip")

go_module(
    name = "logging",
    path = "gopkg.in/op/go-logging.v1",
    repo = "github.com/op/go-logging",
    version = "b2cb9fa56473e98db8caba80237377e83fe44db5",
)

go_module(
    name = "terminal",
    path = "golang.org/x/crypto/ssh/terminal",
    version = "7b85b097bf7527677d54d3220065e966a0e3b613",
)

go_module(
    name = "warnings",
    path = "gopkg.in/warnings.v0",
    version = "v0.1.2",
)

go_module(
    name = "gcfg",
    path = "github.com/peterebden/gcfg",
    version = "v1.3.0",
    deps = [
        ":warnings",
    ],
)

go_module(
    name = "xattr",
    path = "github.com/pkg/xattr",
    version = "d15dbc2bb0b5da267362b5e066e2c44c1fcff6c7",
    deps = [":unix"],
)

go_module(
    name = "go-bindata",
    binary = True,
    path = "github.com/kevinburke/go-bindata",
    version = "46eb4c183bfc1ebb527d9d19bcded39476302eb8",
)

go_module(
    name = "go-flags",
    path = "github.com/jessevdk/go-flags",
    version = "v1.4.0",
)

go_module(
    name = "humanize",
    path = "github.com/dustin/go-humanize",
    version = "8929fe90cee4b2cb9deb468b51fb34eba64d1bf0",
)

go_module(
    name = "mux",
    path = "github.com/gorilla/mux",
    version = "9c068cf16d982f8bd444b8c352acbeec34c4fe5b",
    deps = [
        ":gorilla_context",
    ],
)

go_module(
    name = "gorilla_context",
    path = "github.com/gorilla/context",
    version = "1c83b3eabd45b6d76072b66b746c20815fb2872d",
)

go_module(
    name = "net",
    path = "golang.org/x/net",
    version = "136a25c244d3019482a795d728110278d6ba09a4",
    deps = [
        ":terminal",
        ":text",
    ],
)

go_module(
    name = "text",
    path = "golang.org/x/text",
    version = "4e4a3210bb54bb31f6ab2cdca2edcc0b50c420c1",
    deps = [
        ":tools",
    ],
)

go_module(
    name = "tools",
    path = "golang.org/x/tools",
    version = "2ae76fd1560b622911f444c1e66b70a857e1de67",
)

go_module(
    name = "grpc",
    exported_deps = [":net"],
    path = "google.golang.org/grpc",
    version = "v1.10.0",
    deps = [
        ":protobuf",
        ":rpcstatus",
    ],
)

#TODO: build from the actual proto.
go_module(
    name = "rpcstatus",
    path = "google.golang.org/genproto/googleapis/rpc/status",
    version = "2b5a72b8730b0b16380010cfe5286c42108d88e7",
    deps = [":protobuf"],
)

go_module(
    name = "protobuf",
    path = "github.com/golang/protobuf",
    version = "v1.1.0",
)

go_module_binary(
    name = "protoc-gen-go",
    src = ":protobuf",
    path = "github.com/golang/protobuf/protoc-gen-go",
)

go_module(
    name = "spew",
    path = "github.com/davecgh/go-spew/spew",
    version = "ecdeabc65495df2dec95d7c4a4c3e021903035e5",
)

go_module(
    name = "testify",
    path = "github.com/stretchr/testify",
    version = "f390dcf405f7b83c997eac1b06768bb9f44dec18",
    test_only = True,
    deps = [
        ":difflib",
        ":objx",
        ":spew",
    ],
)

go_module(
    name = "objx",
    path = "github.com/stretchr/objx",
    version = "v0.2.0",
    test_only = True,
)

go_module(
    name = "difflib",
    path = "github.com/pmezard/go-difflib",
    version = "5d4384ee4fb2527b0a1256a821ebfc92f91efefc",
    test_only = True,
)

go_module(
    name = "go-isatty",
    path = "github.com/mattn/go-isatty",
    version = "6ca4dbf54d38eea1a992b3c722a76a5d1c4cb25c",
)

go_module(
    name = "prompter",
    path = "github.com/Songmu/prompter",
    version = "f49666b0047d12850875d771340e1d862d9e7a0c",
    deps = [
        ":go-isatty",
        ":terminal",
    ],
)

go_module(
    name = "levenshtein",
    path = "github.com/texttheater/golang-levenshtein/levenshtein",
    version = "14026fface0cb806188c85e792a93d625dc37d0f",
)

go_module(
    name = "queue",
    path = "github.com/Workiva/go-datastructures/queue",
    version = "064f3ea06ab2337e6eaf42052392d14f779e91cb",
)

go_module(
    name = "fsnotify",
    path = "github.com/fsnotify/fsnotify",
    version = "a8a77c9133d2d6fd8334f3260d06f60e8d80a5fb",
    deps = [":unix"],
)

go_module(
    name = "concurrent-map",
    path = "github.com/streamrail/concurrent-map",
    version = "5fc745307dc80a1883243b978f7e7c0fd5ce7206",
)

go_module(
    name = "quantile",
    path = "github.com/beorn7/perks/quantile",
    version = "4c0e84591b9aa9e6dcfdf3e020114cd81f89d5f9",
)

go_module(
    name = "pbutil",
    path = "github.com/matttproud/golang_protobuf_extensions/pbutil",
    version = "c12348ce28de40eed0136aa2b644d0ee0650e56c",
    deps = [":protobuf"],
)

go_module(
    name = "prometheus",
    path = "github.com/prometheus/client_golang/prometheus",
    version = "v0.8.0",
    deps = [
        ":grpc",
        ":pbutil",
        ":procfs",
        ":prometheus_client_model",
        ":prometheus_common",
        ":protobuf",
        ":quantile",
    ],
)

go_module(
    name = "prometheus_common",
    path = "github.com/prometheus/common",
    version = "c7de2306084e37d54b8be01f3541a8464345e9a5",
    deps = [
        ":pbutil",
        ":prometheus_client_model",
        ":protobuf",
    ],
)

go_module(
    name = "prometheus_client_model",
    path = "github.com/prometheus/client_model/go",
    version = "5c3871d89910bfb32f5fcab2aa4b9ec68e65a99f",
    deps = [
        ":protobuf",
    ],
)

go_module(
    name = "procfs",
    path = "github.com/prometheus/procfs",
    version = "05ee40e3a273f7245e8777337fc7b46e533a9a92",
)

go_module(
    name = "grpc-prometheus",
    path = "github.com/grpc-ecosystem/go-grpc-prometheus",
    version = "34abd90a014618f61222a1b0a7b7eb834a2d0dc3",
    deps = [
        ":grpc",
        ":procfs",
        ":prometheus",
        ":protobuf",
    ],
)

go_module(
    name = "grpc-middleware",
    path = "github.com/grpc-ecosystem/go-grpc-middleware",
    version = "fa8fef87dcecac0bda02d36abb3c790ab9e0030b",
    deps = [
        ":grpc",
        ":net",
        ":protobuf",
    ],
)

go_module(
    name = "shlex",
    path = "github.com/google/shlex",
    version = "6f45313302b9c56850fc17f99e40caebce98c716",
)

go_module(
    name = "semver",
    path = "github.com/coreos/go-semver/semver",
    version = "8ab6407b697782a06568d4b7f1db25550ec2e4c6",
)

go_module(
    name = "memberlist",
    path = "github.com/hashicorp/memberlist",
    version = "2072f3a3ff4b7b3d830be77678d5d4b978362bc4",
    deps = [
        ":dns",
        ":go-metrics",
        ":go-msgpack",
        ":go-multierror",
        ":go-sockaddr",
        ":seed",
    ],
)

go_module(
    name = "seed",
    path = "github.com/sean-/seed",
    version = "e2103e2c35297fb7e17febb81e49b312087a2372",
)

go_module(
    name = "go-msgpack",
    path = "github.com/hashicorp/go-msgpack",
    version = "fa3f63826f7c23912c15263591e65d54d080b458",
)

go_module(
    name = "go-multierror",
    path = "github.com/hashicorp/go-multierror",
    version = "b7773ae218740a7be65057fc60b366a49b538a44",
    deps = [":errwrap"],
)

go_module(
    name = "errwrap",
    path = "github.com/hashicorp/errwrap",
    version = "v1.0.0",
)

go_module(
    name = "go-metrics",
    path = "github.com/armon/go-metrics",
    version = "7aa49fde808223f8dadfdbfd3a20ff6c19e5f9ec",
    deps = [
        ":go-immutable-radix",
        ":verify",
    ],
)

go_module(
    name = "go-sockaddr",
    path = "github.com/hashicorp/go-sockaddr",
    version = "6d291a969b86c4b633730bfc6b8b9d64c3aafed9",
    deps = [
        ":radix",
        ":errwrap",
        ":wordwrap",
    ],
)

go_module(
    name = "radix",
    path = "github.com/armon/go-radix",
    version = "v1.0.0",
)

go_module(
    name = "wordwrap",
    path = "github.com/mitchellh/go-wordwrap",
    version = "v1.0.0",
)

go_module(
    name = "go-immutable-radix",
    path = "github.com/hashicorp/go-immutable-radix",
    version = "7f3cd4390caab3250a57f30efdb2a65dd7649ecf",
    deps = [
        ":go-uuid",
        ":simplelru",
    ],
)

go_module(
    name = "simplelru",
    path = "github.com/hashicorp/golang-lru/simplelru",
    version = "0fb14efe8c47ae851c0034ed7a448854d3d34cf3",
)

go_module(
    name = "go-uuid",
    path = "github.com/hashicorp/go-uuid",
    version = "64130c7a86d732268a38cb04cfbaf0cc987fda98",
)

go_module(
    name = "verify",
    path = "github.com/pascaldekloe/goe/verify",
    version = "c580d16984352a427218a8e43232313d1c1c6f84",
)

go_module(
    name = "dns",
    path = "github.com/miekg/dns",
    version = "be5ae6ca7ac994584d2e30167737687f1c1ded8e",
)

go_module(
    name = "atime",
    path = "github.com/djherbis/atime",
    version = "89517e96e10b93292169a79fd4523807bdc5d5fa",
)

go_module(
    name = "errgroup",
    path = "golang.org/x/sync/errgroup",
    version = "457c5828408160d6a47e17645169cf8fa20218c4",
    deps = [":net"],
)

go_module(
    name = "psutil",
    path = "github.com/shirou/gopsutil",
    version = "v2.17.09",
    deps = [":unix"],
)

go_module(
    name = "unix",
    path = "golang.org/x/sys/unix",
    version = "1b2967e3c290b7c545b3db0deeda16e9be4f98a2",
)

go_module(
    name = "openpgp",
    path = "golang.org/x/crypto/openpgp",
    version = "077efaa604f994162e3307fafe5954640763fc08",
    deps = [":cast5"],
)

go_module(
    name = "cast5",
    path = "golang.org/x/crypto/cast5",
    version = "077efaa604f994162e3307fafe5954640763fc08",
)

go_module_binary(
    name = "go-fuzz-build",
    src = ":go-fuzz-dep",
    path = "github.com/dvyukov/go-fuzz/go-fuzz-build",
)

go_module_binary(
    name = "go-fuzz",
    src = ":go-fuzz-dep",
    path = "github.com/dvyukov/go-fuzz/go-fuzz",
)

go_module(
    name = "go-fuzz-dep",
    path = "github.com/dvyukov/go-fuzz",
    version = "445b00a1141b27425541ee8d7dc2f524faf202a9",
)

go_module(
    name = "godirwalk",
    path = "github.com/karrick/godirwalk",
    version = "v1.7.0",
    deps = [":errors"],
)

go_module(
    name = "errors",
    path = "github.com/pkg/errors",
    version = "30136e27e2ac8d167177e8a583aa4c3fea5be833",
)

go_module(
    name = "docker",
    path = "docker.io/go-docker",
    repo = "github.com/docker/go-docker",
    version = "v1.0.0",
    deps = [
        ":docker-distribution",
        ":errors",
        ":go-connections",
        ":go-digest",
        ":go-units",
        ":gogo-protobuf",
        ":image-spec",
        ":net",
    ],
)

go_module(
    name = "docker-distribution",
    path = "github.com/docker/distribution",
    version = "edc3ab29cdff8694dd6feb85cfeb4b5f1b38ed9c",
    deps = [
        ":go-digest",
        ":net",
    ],
)

go_module(
    name = "go-connections",
    path = "github.com/docker/go-connections",
    version = "v0.3.0",
    deps = [
        ":errors",
        ":net",
    ],
)

go_module(
    name = "go-units",
    path = "github.com/docker/go-units",
    version = "v0.3.2",
)

go_module(
    name = "gogo-protobuf",
    path = "github.com/gogo/protobuf",
    version = "v1.2.1",
)

go_module(
    name = "go-digest",
    path = "github.com/opencontainers/go-digest",
    version = "v1.0.0-rc1",
)

go_module(
    name = "image-spec",
    path = "github.com/opencontainers/image-spec",
    version = "v1.0.0",
    deps = [
        ":errors",
        ":go-digest",
    ],
)

go_module(
    name = "skylark",
    path = "github.com/google/skylark/syntax",
    version = "a5f7082aabed29c0e429c722292c66ec8ecf9591",
)

go_module(
    name = "buildifier",
    path = "github.com/bazelbuild/buildtools",
    binary = True,
    version = "0.19.2",
    deps = [
        ":protobuf",
        ":skylark",
    ],
)

go_module(
    name = "jsonrpc2",
    path = "github.com/sourcegraph/jsonrpc2",
    version = "549eb959f029d014d623104d40ab966d159a92de",
    deps = [":websocket"],
)

go_module(
    name = "websocket",
    path = "github.com/gorilla/websocket",
    version = "v1.4.0",
)

go_module(
    name = "xz",
    path = "github.com/ulikunitz/xz",
    version = "v0.5.6",
)

go_module(
    name = "ar",
    path = "github.com/peterebden/ar",
    version = "a0ae3a11a518fb9559c95ed151ec07865b31980b",
)

go_module(
    name = "cli-init",
    path = "github.com/peterebden/go-cli-init",
    version = "v1.0.0",
    deps = [
        ":go-flags",
        ":logging",
        ":terminal",
    ],
)
