go_library(
    name = 'worker',
    srcs = ['worker.go'],
    deps = [
        '//src/core',
        '//src/test',
        '//src/test/proto:worker',
        '//third_party/go:context',
        '//third_party/go:grpc',
        '//third_party/go:logging',
    ],
)

go_binary(
    name = 'please_test_worker',
    srcs = ['main.go'],
    deps = [
        ':worker',
        '//src/cli',
    ],
)

go_test(
    name = 'worker_test',
    srcs = ['worker_test.go'],
    deps = [
        ':worker',
        '//src/test/proto:worker',
        '//third_party/go:context',
        '//third_party/go:grpc',
        '//third_party/go:testify',
    ],
)
