go_library(
    name = 'unzip'
    srcs = ['unzip.go'],
    visibility = ['//tools/jarcat:all'],
)

go_test(
    name = 'unzip_test',
    srcs = ['unzip_test.go'],
    deps = [':unzip', '//third_party/go:testify'],
    data = ['test_data'],
)
