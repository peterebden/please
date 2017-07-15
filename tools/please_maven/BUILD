go_library(
    name = 'maven',
    srcs = [
        'fetch.go',
        'pom.go',
        'print.go',
    ],
    deps = ['//third_party/go:logging'],
)

go_binary(
    name = 'please_maven',
    srcs = ['main.go'],
    deps = [
        ':maven',
        '//src/cli',
    ],
    visibility = ['PUBLIC'],
)
