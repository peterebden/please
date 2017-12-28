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
    visibility = ['//src/parse/...'],
)

go_test(
    name = 'parser_test',
    srcs = ['parser_test.go'],
    data = ['test_data'],
    deps = [
        ':participle',
        '//third_party/go:testify',
    ],
)

go_test(
    name = 'lexer_test',
    srcs = ['lexer_test.go'],
    deps = [
        ':participle',
        '//third_party/go:participle',
        '//third_party/go:testify',
    ],
)
