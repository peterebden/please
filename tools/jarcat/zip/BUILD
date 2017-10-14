go_library(
    name = 'zip',
    srcs = ['writer.go'],
    deps = [
        '//third_party/go:logging',
        '//third_party/go:testify',
        '//third_party/go/zip',
    ],
    visibility = ['//tools/please_pex:all'],
)

go_test(
    name = 'writer_test',
    srcs = ['writer_test.go'],
    data = ['test_data/test.zip'],
    deps = [
        ':zip',
        '//third_party/go/zip',
    ],
)
