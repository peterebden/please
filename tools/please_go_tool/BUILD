# We have a big circular dependency problem here: If we need this tool to compile
# any Go code, and it itself is written in Go, how do we compile it?
# Answer: bootstrap it manually with go.
package(go_tool = ':bootstrap_gotool')

genrule(
    name = 'bootstrap_gotool',
    srcs = glob(['*.go'], excludes = ['*_test.go']),
    outs = ['bootstrap_gotool'],
    cmd = ' && '.join([
        'export GOPATH=$TMP_DIR:$TMP_DIR/third_party/go',
        'mv tools/please_go_tool/please_go_tool.go src',
        'mv tools src',
        'ln -s $TMP_DIR/src/tools/please_go_tool $TMP_DIR/src/tools/please_go_tool/gotool',
        'GOBIN=$TMP_DIR go install src/please_go_tool.go',
        'mv please_go_tool bootstrap_gotool',
    ]),
    requires = ['go_src'],
    deps = [
        '//third_party/go:logging',
    ],
)

go_library(
    name = 'gotool',
    srcs = [
        'compile.go',
        'find_cover_vars.go',
        'write_test_main.go',
    ],
    deps = ['//third_party/go:logging'],
)

go_binary(
    name = 'please_go_tool',
    srcs = ['please_go_tool.go'],
    deps = [
        ':gotool',
        '//third_party/go:logging',
    ],
    visibility = ['PUBLIC'],
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
