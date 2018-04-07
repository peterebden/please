go_library(
    name = 'vfs',
    srcs = ['vfs.go'],
    deps = [
        '//third_party/go:fuse',
        '//third_party/go:logging',
    ],
)

go_test(
    name = 'vfs_test',
    srcs = ['vfs_test.go'],
    data = ['test_data'],
    deps = [
        ':vfs',
        '//third_party/go:fuse',
        '//third_party/go:testify',
    ],
)
