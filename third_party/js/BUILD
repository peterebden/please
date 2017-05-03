filegroup(
    name = 'asciinema',
    srcs = [
        'asciinema-player.css',
        'asciinema-player.js',
    ],
    visibility = ['//docs/...'],
)

remote_file(
    name = 'yarn',
    binary = True,
    hashes = ['d4920d5c273ad6c3ad9f0c5faa90b4f56215f70e'],
    url = 'https://github.com/yarnpkg/yarn/releases/download/v0.23.4/yarn-0.23.4.js',
)

yarn_library(
    name = 'babel',
    version = '6.23.0',
    visibility = ['PUBLIC'],
)
