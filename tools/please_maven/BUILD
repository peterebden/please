go_library(
    name = 'maven',
    srcs = [
        'fetch.go',
        'pom.go',
        'print.go',
        'resolver.go',
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

go_test(
    name = 'maven_test',
    srcs = ['maven_test.go'],
    data = ['test_data'],
    deps = [
        ':maven',
        '//third_party/go:testify',
    ],
)
