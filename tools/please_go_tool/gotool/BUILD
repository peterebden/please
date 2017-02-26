package(go_tool = '//tools/please_go_tool:bootstrap_gotool')

filegroup(
    name = 'gotool_srcs',
    srcs = glob(['*.go'], excludes = ['*_test.go']),
    visibility = ['//tools/please_go_tool:all'],
)

go_library(
    name = 'gotool',
    srcs = [':gotool_srcs'],
    deps = ['//third_party/go:logging'],
    visibility = ['//tools/please_go_tool/...'],
)

go_test(
    name = 'find_cover_vars_test',
    srcs = ['find_cover_vars_test.go'],
    data = [
        'test_data/binary/core.a',
        'test_data/binary/lock.go',
        'test_data/core.a',
        'test_data/lock.go',
        ':test_excluded_archive',
    ],
    deps = [
        ':gotool',
        '//third_party/go:testify',
    ],
)

# Little tweak so we don't have to commit this file twice.
genrule(
    name = 'test_excluded_archive',
    srcs = ['test_data/core.a'],
    outs = ['test_data/x/core.a'],
    cmd = 'cp $SRC $OUT',
)

go_test(
    name = 'write_test_main_test',
    srcs = ['write_test_main_test.go'],
    data = glob(['test_data/*.go']),
    deps = [
        ':gotool',
        '//third_party/go:testify',
    ],
)
