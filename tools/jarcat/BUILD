go_binary(
    name = 'jarcat',
    srcs = ['jarcat.go'],
    deps = [
        ':zip_writer',
        '//src/cli',
        '//third_party/go:logging',
        '//third_party/go/zip',
        '//tools/jarcat/tar',
    ],
    visibility = ['PUBLIC'],
)
