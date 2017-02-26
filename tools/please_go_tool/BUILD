# We have a big circular dependency problem here: If we need this tool to compile
# any Go code, and it itself is written in Go, how do we compile it?
# Answer: bootstrap it manually with go.
package(go_tool = ':bootstrap_gotool')

genrule(
    name = 'bootstrap_gotool',
    srcs = {
        'main': ['please_go_tool.go'],
        'lib': ['//tools/please_go_tool/gotool:gotool_srcs'],
    },
    outs = ['bootstrap_gotool'],
    cmd = ' && '.join([
        'export GOPATH=$TMP_DIR:$TMP_DIR/third_party/go',
        'ln -s . src',
        'GOBIN=$TMP_DIR go install $SRCS_MAIN',
        'mv please_go_tool bootstrap_gotool',
    ]),
    deps = [
        '//third_party/go:go-flags',
        '//third_party/go:logging',
    ],
    visibility = ['PUBLIC'],
)

go_binary(
    name = 'please_go_tool',
    srcs = ['please_go_tool.go'],
    deps = [
        '//third_party/go:go-flags',
        '//third_party/go:logging',
        '//tools/please_go_tool/gotool',
    ],
    visibility = ['PUBLIC'],
)
