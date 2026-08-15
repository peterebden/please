plugin_repo(
    name = "go",
    plugin = "go-rules",
    revision = "v1.31.1",
)

plugin_repo(
    name = "cc",
    plugin = "cc-rules",
    revision = "v0.7.1",
)

plugin_repo(
    name = "shell",
    plugin = "shell-rules",
    revision = "v0.2.1",
)

plugin_repo(
    name = "python",
    plugin = "python-rules",
    revision = "v1.14.0",
)

# Exposed so //test/build_defs:plugin_versions_match_repo_test can check that the e2e test repos
# pin the same plugin versions this repo builds with.
filegroup(
    name = "build_file",
    srcs = ["BUILD"],
    visibility = ["//test/build_defs:all"],
)
