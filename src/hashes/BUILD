go_bindata(
    name = 'hash_rewriter',
    srcs = ['hash_rewriter.py'],
)

go_library(
    name = 'hashes',
    srcs = [
        'rewrite_hashes.go',
        ':hash_rewriter',
    ],
    deps = [
        '//src/build',
        '//src/core',
        '//src/parse/asp',
        '//third_party/go:logging',
    ],
    visibility = ['PUBLIC'],
)

go_test(
    name = 'hash_rewriter_test',
    srcs = ['hash_rewriter_test.go'],
    data = [
        'test_data',
    ],
    deps = [
        ':hashes',
        '//src/core',
        '//third_party/go:testify',
    ],
)
