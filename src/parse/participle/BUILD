go_library(
    name = 'participle',
    srcs = [
        'grammar.go',
        'lexer.go',
        'parser.go',
    ],
    deps = [
        '//src/core',
        '//third_party/go:logging',
        '//third_party/go:participle',
    ],
)

go_test(
    name = 'parser_test',
    srcs = ['parser_test.go'],
    deps = [
        ':participle',
        '//third_party/go:testify',
    ],
)
