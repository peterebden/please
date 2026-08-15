package(default_visibility = ["//test/..."])

subinclude("///shell//build_defs:shell", "///go//build_defs:go")

sh_binary(
    name = "content_checker",
    main = "test_file_content.sh",
    visibility = ["//test/..."],
)

filegroup(
    name = "build_defs",
    srcs = ["test.build_defs"],
    visibility = [
        "//src/test/...",
        "//test/...",
    ],
)

filegroup(
    name = "base_config",
    srcs = [
        ".plzconfig.e2e",
        ".plzconfig_freebsd_amd64",
    ],
)

go_binary(
    name = "plugin_version_checker",
    srcs = ["check_plugin_versions.go"],
    visibility = ["//test/..."],
)

filegroup(
    name = "plugin_versions",
    srcs = ["plugin_versions.json"],
    visibility = ["//test/..."],
)

# One command to bring everything back into line after bumping a plugin in //plugins:BUILD or the Go
# toolchain in //third_party/go:BUILD. The failing checks point at this by name.
# Labelled codegen so `plz autofix` picks it up along with the other mechanical fixups.
sh_cmd(
    name = "fix_plugin_versions",
    cmd = "$(out_exe :plugin_version_checker) -versions test/build_defs/plugin_versions.json -fix",
    data = [":plugin_version_checker"],
    labels = ["codegen"],
    visibility = ["PUBLIC"],
)

# The e2e tests check their own repos against plugin_versions.json; this checks that file against
# the versions the please repo itself builds with, so it can't quietly drift into being a separate
# source of truth.
gentest(
    name = "plugin_versions_match_repo_test",
    data = {
        "PLUGINS": ["//plugins:build_file"],
        "TOOLCHAIN": ["//third_party/go:build_file"],
        "VERSIONS": [":plugin_versions"],
    },
    no_test_output = True,
    test_cmd = "$TOOLS_CHECKER -versions $DATA_VERSIONS -plugins $DATA_PLUGINS -toolchain $DATA_TOOLCHAIN",
    test_tools = {"CHECKER": [":plugin_version_checker"]},
)

# Used for some of the parse tests that want a subinclude that isn't visible to them.
filegroup(
    name = "not_visible",
    visibility = [],
)
