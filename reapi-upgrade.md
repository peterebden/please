# Upgrading Please's REAPI version

Context: [thought-machine/please#3600](https://github.com/thought-machine/please/issues/3600). Please reports REAPI 2.1 and rejects servers whose range doesn't include it (e.g. Buildbarn and Buildfarm, which now start at 2.3).

## Summary

- **This is mostly a negotiation problem, not a dependency problem.** Our Go bindings are already current: `remote-apis` is pinned at `becdd8f`, which is after v2.12.0, and the SDK is from 2026-06. What fails is the check at `src/remote/remote.go:243`, which accepts a server only if its low..high range contains our one hardcoded `apiVersion = 2.1`.
- **Nothing between 2.2 and 2.12 adds a new obligation for a SHA-256 client that doesn't use the new optional features.** Every MUST/SHOULD added between v2.3.0 and v2.12.0 only applies if you opt into chunking, a new digest function or the routing header. We already do what 2.1 and 2.2 require, and we already rely on 2.3's lookup rule for the first argument.
- **There is one real bug to fix first.** Since v2.1, a server that receives `output_paths` returns symlink outputs in `ActionResult.output_symlinks`, not in the two older, deprecated symlink fields. Our code only reads the deprecated ones, and so does the upstream SDK. Buildbarn only fills `output_symlinks` (`pkg/builder/output_hierarchy.go`), so once the version check stops blocking Buildbarn, symlinked outputs would be lost there.

## Version history and what each one means for us

| Version (tag date) | Main changes | Effect on Please |
|---|---|---|
| **2.1** (2019-11) | `Command.output_paths` replaces `output_files`/`output_directories`; `ActionResult.output_symlinks` replaces `output_file_symlinks`/`output_directory_symlinks` | We send `output_paths` ✅. We **don't read `output_symlinks`** ❌ |
| **2.2** (2020-10) | Platform moves to `Action.platform` (clients should set both). Remote Asset API, LogStream, `Action.salt`, node properties | We set both ✅ |
| **2.3** (2022-04) | Compressed blobs over ByteStream (zstd/deflate), `auxiliary_metadata`, `virtual_execution_duration`, extra `RequestMetadata` fields (`action_mnemonic`, `target_id`, `configuration_id`). The first command argument may now be looked up via `PATH` | We **already depend on the PATH lookup** (default `remote.Shell = "bash"`). Compression and the new metadata fields are optional |
| **2.4** (2022-10) | `OutputDirectory.is_topologically_sorted` hint; Tree ordering notes | Optional |
| **2.5** (2023-03, includes 2.6/2.7) | SHA256TREE, brotli, `partial_execution_metadata`, **multiple digest functions**: `digest_function` on every request, `ExecutionCapabilities.digest_functions` | `digest_function` may be left unset for SHA-1/SHA-256 ✅. We already read `partial_execution_metadata.worker`. `chooseDigest` only looks at the legacy single field, which is allowed but should prefer the list |
| **2.8** (2023-12) | BLAKE3; `Command.output_directory_format` and `OutputDirectory.root_directory_digest` | Optional. The default, TREE_ONLY, is what we do today |
| **2.9** (2024-04) | `ExecuteRequest.inline_stdout`, `inline_stderr`, `inline_output_files` | Optional. Useful for post-build functions |
| **2.10** (2024-04) | `digest_function` in the Remote Asset API | Optional for SHA-256. Our `checksum.sri` encoding matches the clarified wording (#330) ✅ |
| **2.11** (2024-09) | `ExecuteOperationMetadata.digest_function` | Server side |
| **2.12** (2026-02, latest release) | `max_cas_blob_size_bytes`, unary `SplitBlob`/`SpliceBlob` + FastCDC/RepMaxCDC chunking, GITSHA1, optional ByteStream routing header | All optional or server-advertised. We should respect `max_cas_blob_size_bytes` |
| **2.13** (unreleased, on `main`) | Unary Split/Splice **removed**, replaced by streaming `GetChunkMapping`/`RegisterChunkMapping` (#377, 2026-09-08) | Don't build on the 2.12 unary Split/Splice calls |

Minor-version tags are messy (2.4–2.11 were only cut as final releases in 2024–2026). The proto's own "New in vX" annotations are the reliable source.

## Gaps in our code (`src/remote`)

1. **`output_symlinks` is ignored.**
   - `outputTree` (`utils.go:145`, `:172`) builds the stored output tree only from the deprecated fields, so symlink outputs would be missing from the inputs of dependent remote actions.
   - `outputHash` (`utils.go:633`) has the same problem.
   - `DownloadActionOutputs` goes through the SDK's `FlattenActionOutputs` (`cas_download.go:487-498`), which also only reads the deprecated fields (still true on SDK master).
   - `outputsForActionResult` (`action.go:440`) reads both, so verification passes and the failure only shows up later.
2. **The version check tests for one version instead of overlapping ranges.** It should find the overlap between our range and the server's, and accept `deprecated_api_version` (currently ignored) as the server's real lower bound.
3. **Test actions send inconsistent platforms.** `buildTestCommand` puts only `OSFamily` in `Command.platform`, while the Action gets the target's full platform properties. Servers on 2.2+ prefer the Action's.
4. **Side finding: `hash_function = sha1` with remote execution looks broken.** Nothing sets the SDK's `digest.HashFn`, so `digestMessage`, `protoEntry` and `DownloadInputs` always produce SHA-256 digests, while `chooseDigest` would accept a SHA-1 server.
5. `docs/remote_builds.html:54` says "Please requires version 2.1".

## remote-apis-sdks

- We're on `7ffd493e` (2026-06-10). Master has two later fixes worth picking up:
  - #665: downloads could follow symlinks already present at the destination (security-relevant).
  - #667: cleaning the root output directory in `DownloadActionOutputs`.
- The SDK is effectively in maintenance mode for reclient: no support for `output_symlinks` on download, per-request `digest_function`, `output_directory_format`, `max_cas_blob_size_bytes` or split/splice. It does support zstd (off by default; we never set `CompressedBytestreamThreshold`).
- The symlink fix is small enough to upstream. Until then, copy `OutputSymlinks` into the deprecated fields before calling the SDK.

## please-servers

- **Advertised versions:** flair and elan 2.0–2.3; mettle 2.0–2.1, marked "optimistic" (`mettle/api/api.go:294`). Anything we pick has to keep working against 2.1-only servers.
- **Symlink outputs:** the mettle worker only writes the deprecated fields (`mettle/worker/worker.go:1197-1198`), which is why we haven't noticed the client bug. It should fill `OutputSymlinks` as well; filling both is safe for old and new clients.
- **SDK fork:** a replace to `peterebden/remote-apis-sdks@2a9420921957` (2023), 2 commits ahead ("packs" compression, more permissive symlinks) and 149 behind upstream.
- **Already done:** `Action.platform`, `virtual_execution_duration`, `auxiliary_metadata`, `partial_execution_metadata`, zstd, `inline_stdout` on `GetActionResult`. SplitBlob/SpliceBlob are stubbed as Unimplemented and not advertised.

## Server compatibility with the proposed range (2.1–2.12)

| Server | Advertised | Today (needs 2.1) | Proposed |
|---|---|---|---|
| please-servers flair/elan | 2.0–2.3 | ✅ | ✅ |
| please-servers mettle | 2.0–2.1 | ✅ | ✅ |
| bazel-remote | 2.0–2.3 | ✅ | ✅ |
| Buildbarn | 2.3–2.12 (per #3600) | ❌ | ✅ |
| BuildBuddy | 2.0–2.11 | ✅ | ✅ |
| Buildfarm | 2.3–2.11 | ❌ | ✅ |
| BuildGrid | 2.0–2.2 | ✅ | ✅ |
| EngFlow | 2.0–2.11 | ✅ | ✅ |
| Justbuild | 2.0–2.1 | ✅ | ✅ |
| Kajiya | 2.0 | ❌ | ❌ |
| NativeLink | 2.0–2.3 | ✅ | ✅ |

Ranges from the remote-apis README server table.

## Suggested plan

**Phase 1 — fixes #3600:**
1. Handle `output_symlinks` in `outputTree` and `outputHash`; work around or upstream the SDK `FlattenActionOutputs` gap.
2. Replace the single-version check with overlapping ranges: we support **2.1–2.12**; the server's lower bound is `min(deprecated_api_version, low_api_version)`, warning if the only overlap is deprecated. A conservative alternative is capping at 2.3.
3. Bump the SDK to master.
4. Update the docs.
5. Test symlinked outputs against a real Buildbarn.

`SuppressVersionCheck` (this branch) becomes an escape hatch rather than the fix.

**Phase 2 — please-servers:** fill `OutputSymlinks` in mettle, make mettle's advertised range consistent with flair's, plan how to get off the SDK fork.

**Phase 3 — optional improvements, roughly by value:**
- zstd compression (SDK supports it, elan advertises it).
- Respect `max_cas_blob_size_bytes`.
- Fill `target_id` and `action_mnemonic` in `RequestMetadata`.
- Prefer `ExecutionCapabilities.digest_functions` over the legacy field.
- `ExecuteRequest.inline_stdout` for post-build functions.
- `output_directory_format = DIRECTORY_ONLY`, to avoid fetching whole Trees.
- Chunking: wait for 2.13 to be released.
