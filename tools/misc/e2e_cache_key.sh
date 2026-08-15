#!/usr/bin/env bash
#
# Prints a fingerprint of everything that determines what ends up in the e2e tests' shared dir cache
# (see SHARED_CACHE_DIR in //test/build_defs:test.build_defs), for CI to checksum as a cache key.
#
# In practice that's the plugin revisions, Go toolchain and third-party Go modules the test repos
# pin: those account for almost all of the cache, and everything else in it is cheap to rebuild.
# Cache entries are content-addressed, so a key that's too coarse only leaves dead weight behind
# rather than serving anything stale.
#
# Files are selected by what they contain rather than where they live - test repos spell their build
# files BUILD, BUILD_FILE and BUILD.test, and pins turn up in all three - so a newly added test repo
# is picked up automatically.

set -euo pipefail

cd "$(dirname "$0")/../.."

{
    # The canonical list every test repo is held to by //test/build_defs:plugin_versions_match_repo_test.
    cksum test/build_defs/plugin_versions.json
    grep -rl --include='BUILD' --include='BUILD_FILE' --include='BUILD.*' \
        -E 'plugin_repo\(|go_toolchain\(|go_repo\(' test \
        | LC_ALL=C sort \
        | xargs cksum
}
