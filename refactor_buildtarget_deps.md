# Refactoring Plan: Removing Active Dependency Resolution from BuildTarget

## Objective
Simplify `BuildTarget` by removing its active role in resolving and caching dependency graph structures. Specifically, we will remove the `depInfo` struct and the `resolveDependencies` family of functions. `BuildTarget` will transition to a passive data structure that strictly stores declared `BuildLabel`s and metadata (e.g., exported, source, runtime), delegating resolution and `requires/provides` mapping to the `BuildGraph` and its consumers.

## Key Files & Context
- `src/core/build_target.go`: Contains `BuildTarget`, `depInfo`, and all dependency iterators.
- `src/core/build_target_test.go`: Tests for the dependency iterators.
- `src/core/utils.go`: Centralizes build input iteration and relies heavily on `recursivelyProvideFor`.
- `src/plz/plz.go`: Initiates the build phase and currently relies on `target.ResolveDependencies()`.
- Widespread Consumers: `src/query/*`, `src/gc/gc.go`, `src/export/export.go`, `src/watch/watch.go`, etc.

## Proposed Solution & Implementation Steps

### Phase 1: Structural Simplification of BuildTarget
1. **Remove `depInfo`**: Delete the `depInfo` struct in `src/core/build_target.go`.
2. **Introduce `DeclaredDependency`**: Create a new, simpler struct to hold a `BuildLabel` alongside its metadata flags, but without any pointers to `*BuildTarget` or graph state.
   ```go
   type DeclaredDependency struct {
       Label    BuildLabel
       Exported bool
       Source   bool
       Internal bool
       Runtime  bool // Could potentially be merged with the existing runtimeDependencies slice
   }
   ```
3. **Update `BuildTarget` Fields**: Replace `dependencies []depInfo` with `dependencies []DeclaredDependency`.
4. **Update `AddMaybeExportedDependency`**: Modify this function to append to the new `DeclaredDependency` slice instead of `depInfo`. Remove deduplication logic that relies on `dependencyInfo`, replacing it with a simple map/loop check against `DeclaredDependency.Label`.

### Phase 2: Updating Iterators and Extracting Resolution
1. **Refactor Iterators to yield `BuildLabel`**:
   - Change `DeclaredDependencies()`, `ExportedDependencies()`, etc., to return `iter.Seq[BuildLabel]` or `[]BuildLabel`.
   - **Crucial Changes**:
     - Remove `Dependencies() []*BuildTarget`.
     - Remove `BuildDependencies() []*BuildTarget`.
     - Remove `ExternalDependencies() []*BuildTarget`.
2. **Remove Resolution Logic**: Delete `resolveDependencies`, `resolveOneDependency`, `ResolveDependencies`, and `resolveDependency` from `BuildTarget`.
3. **Migrate `requires/provides` handling**: The logic inside `resolveOneDependency` that handled `provideFor` must be formally extracted. `src/core/utils.go` already has `recursivelyProvideFor`; this should be promoted to a primary mechanism on `BuildGraph` (e.g., `graph.ResolveEdges(target, depLabel)`) for use by consumers who need the *actual* targets a label maps to.

### Phase 3: Updating Consumers
This is the most extensive phase, requiring updates across the codebase. Since iterators will return `BuildLabel` instead of `*BuildTarget`, consumers must fetch targets from the graph and manually resolve `requires/provides` where necessary.

**1. Core Graph Traversal (DFS/BFS)**
These files iterate dependencies to visit the entire graph. They need to fetch `graph.TargetOrDie(label)` and manage resolution.
- `src/watch/watch.go`: `startWatching()` uses `Dependencies()`.
- `src/gc/gc.go`: `addTarget()` uses `Dependencies()`.
- `src/export/export.go`: `export()` uses `Dependencies()`.
- `src/core/cycle_detector.go`: `visit()` uses `Dependencies()`.

**2. Test Coverage & Stamps**
- `src/test/coverage.go`: `collectAllFiles` uses `ExternalDependencies()`. Will need to filter out targets with the same parent label manually.
- `src/core/stamp.go`: `populateStampInfo()` uses `Dependencies()`.

**3. Build Inputs & Sandboxing**
These are critical for determining what actually goes into the build sandbox. They already heavily use `recursivelyProvideFor`, but the initial entry point needs to change.
- `src/core/utils.go`:
  - `IterInputs()` uses `BuildDependencies()`.
  - `IterRuntimeFiles()` uses `Dependencies()`.

**4. Query Commands**
These build JSON graphs or print information and need to map labels to targets.
- `src/query/graph.go`: `addJSONTarget` and `makeJSONTarget` use `Dependencies()`.
- `src/parse/asp/builtins.go`: `getLabels()` (the `get_labels` builtin) uses `Dependencies()`.

**5. Command Replacements**
- `src/core/command_replacements.go`: `ReplaceSequences` uses `DependenciesFor(label)`. This will need to be replaced with a `BuildGraph` method that fully resolves the dependencies for a given label considering `requires/provides`.

**6. Build Orchestration**
- `src/plz/plz.go`: `reallyBuild()` calls `target.ResolveDependencies(state.Graph)`. This is a blocking call that ensures all dependencies are in the graph before building. The centralized orchestrator or worker pool must replace this by tracking when all `DeclaredDependencies` are satisfied in the graph.

**7. Test Updates**
Many tests directly manipulate or assert on the dependency graph and will need updating:
- `src/core/build_target_test.go`: Tests asserting `Dependencies()`, `BuildDependencies()`, `DependenciesFor()`, `ExternalDependencies()`.
- `src/core/cycle_detector_test.go`: Asserts `len(target.DeclaredDependencies()) != len(target.Dependencies())`.
- `src/query/graph_test.go`, `src/query/changes_test.go`, `src/query/reverse_deps_test.go`, `src/parse/asp/builtins_test.go`, `src/core/command_replacements_test.go`: These all manually call `target.ResolveDependencies(graph)` to set up test state. They will either need to rely on the new orchestrator logic or use a test helper to force resolution.

## Alternatives Considered
- **Keep `depInfo` but strip the `*BuildTarget` cache**: This is essentially what Phase 1 does, but renaming it to `DeclaredDependency` clarifies intent.
- **Move Resolution to `BuildState` instead of `BuildGraph`**: `BuildGraph` is the more appropriate owner for edge traversal logic, keeping state mutation separate from graph querying.

## Verification & Testing
1. Ensure `please test //src/core/...` passes after Phase 1 and 2.
2. Run the full integration test suite (`please test //...`) to verify that the distributed consumer updates in Phase 3 correctly maintain `requires/provides` semantics and do not introduce regressions in sandboxing or graph traversal.
3. Pay special attention to cycle detection tests (`//src/core:cycle_detector_test`), as they heavily rely on traversing the dependency graph.