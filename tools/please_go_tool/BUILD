go_library(
    name = 'gotool',
    srcs = ['compile.go'],
    deps = ['//third_party/go:logging'],
)

go_binary(
    name = 'please_go_tool',
    srcs = ['please_go_tool.go'],
    deps = [
        ':gotool',
        '//src/cli',
        '//third_party/go:logging',
    ],
    visibility = ['PUBLIC'],
)
