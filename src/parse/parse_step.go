// Package responsible for parsing build files and constructing build targets & the graph.
//
// The actual work here is done by an embedded PyPy instance. Various rules are built in to
// the binary itself using go-bindata to embed the .py files; these are always available to
// all programs which is rather nice, but it does mean that must be run before 'go run' etc
// will work as expected.
package parse

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"core"
)

// Parses the package corresponding to a single build label. The label can be :all to add all targets in a package.
// It is not an error if the package has already been parsed.
//
// By default, after the package is parsed, any targets that are now needed for the build and ready
// to be built are queued, and any new packages are queued for parsing. When a specific label is requested
// this is straightforward, but when parsing for pseudo-targets like :all and ..., various flags affect it:
// If 'noDeps' is true, then no new packages will be added and no new targets queued.
// 'include' and 'exclude' refer to the labels of targets to be added. If 'include' is non-empty then only
// targets with at least one matching label are added. Any targets with a label in 'exclude' are not added.
// 'forSubinclude' is set when the parse is required for a subinclude target so should proceed
// even when we're not otherwise building targets.
func Parse(tid int, state *core.BuildState, label, dependor core.BuildLabel, noDeps bool, include, exclude []string, forSubinclude bool) {
	defer func() {
		if r := recover(); r != nil {
			state.LogBuildError(tid, label, core.ParseFailed, fmt.Errorf("%s", r), "Failed to parse package")
		}
	}()
	pkg, added := state.Graph.GetOrAddPackage(label.PackageName)
	if added {
		// We are the first to reach this package, so it falls to us to parse it.
		state.LogBuildResult(tid, label, core.PackageParsing, "Parsing...")
		if parsePackage(state, pkg, label, dependor) {
			state.LogBuildResult(tid, label, core.PackageParsed, "Deferred")
		} else {
			state.LogBuildResult(tid, label, core.PackageParsed, "Parsed")
		}
	}
	state.AddPendingTarget()
	go pkg.WhenReady(func() {
		activateTarget(state, pkg, label, dependor, noDeps, forSubinclude, include, exclude)
		state.TaskDone()
	})
}

// activateTarget marks a target as active (ie. to be built) and adds its dependencies as pending parses.
func activateTarget(state *core.BuildState, pkg *core.Package, label, dependor core.BuildLabel, noDeps, forSubinclude bool, include, exclude []string) {
	if !label.IsAllTargets() && state.Graph.Target(label) == nil {
		msg := fmt.Sprintf("Parsed build file %s/BUILD but it doesn't contain target %s", label.PackageName, label.Name)
		if dependor != core.OriginalTarget {
			msg += fmt.Sprintf(" (depended on by %s)", dependor)
		}
		panic(msg + suggestTargets(pkg, label, dependor))
	}
	if noDeps && !dependor.IsAllTargets() { // IsAllTargets indicates requirement for parse
		return // Some kinds of query don't need a full recursive parse.
	} else if label.IsAllTargets() {
		for _, target := range pkg.Targets {
			if target.ShouldInclude(include, exclude) {
				// Must always do this for coverage because we need to calculate sources of
				// non-test targets later on.
				if !state.NeedTests || target.IsTest || state.NeedCoverage {
					l := target.Label
					l.Arch = label.Arch
					addDep(state, l, dependor, false, dependor.IsAllTargets())
				}
			}
		}
	} else {
		for _, l := range state.Graph.DependentTargets(dependor, label) {
			// We use :all to indicate a dependency needed for parse.
			addDep(state, l, dependor, false, forSubinclude || dependor.IsAllTargets())
		}
	}
}

// Used to arbitrate single access to this map
var pendingTargetMutex sync.Mutex

// Map of build label -> any packages that have subincluded it.
var deferredParses = map[core.BuildLabel][]*core.Package{}

// deferParse defers the parsing of a package until the given label has been built.
// Returns true if it was deferred, or false if it's already built.
func deferParse(label core.BuildLabel, pkg *core.Package) bool {
	pendingTargetMutex.Lock()
	defer pendingTargetMutex.Unlock()
	if target := core.State.Graph.Target(label); target != nil && target.State() >= core.Built {
		return false
	}
	log.Debug("Deferring parse of %s pending %s", pkg.Name, label)
	deferredParses[label] = append(deferredParses[label], pkg)
	pkg.SubincludeUnready()
	core.State.AddPendingParse(label, core.BuildLabel{PackageName: pkg.Name, Name: "all"}, true)
	return true
}

// UndeferAnyParses un-defers the parsing of a package if it depended on some subinclude target being built.
func UndeferAnyParses(state *core.BuildState, target *core.BuildTarget) {
	pendingTargetMutex.Lock()
	defer pendingTargetMutex.Unlock()
	if pkgs, present := deferredParses[target.Label]; present {
		for _, pkg := range pkgs {
			pkg.SubincludeReady()
		}
		delete(deferredParses, target.Label) // Don't need this any more.
	}
}

// parsePackage performs the initial parse of a package.
// It returns true if the parse was deferred due to pending subinclude() calls or false if it's ready immediately.
func parsePackage(state *core.BuildState, pkg *core.Package, label, dependor core.BuildLabel) bool {
	packageName := label.PackageName
	if strings.HasPrefix(packageName, systemPackage) {
		// System packages don't have associated BUILD files. It is annoying if we can't handle those targets though.
		parseSystemPackage(state, packageName)
		return false // system packages cannot subinclude so are never deferred
	}
	if pkg.Filename = buildFileName(state, packageName); pkg.Filename == "" {
		exists := core.PathExists(packageName)
		// Handle quite a few cases to provide more obvious error messages.
		if dependor != core.OriginalTarget && exists {
			panic(fmt.Sprintf("%s depends on %s, but there's no BUILD file in %s/", dependor, label, packageName))
		} else if dependor != core.OriginalTarget {
			panic(fmt.Sprintf("%s depends on %s, but the directory %s doesn't exist", dependor, label, packageName))
		} else if exists {
			panic(fmt.Sprintf("Can't build %s; there's no BUILD file in %s/", label, packageName))
		}
		panic(fmt.Sprintf("Can't build %s; the directory %s doesn't exist", label, packageName))
	}
	return parsePackageDefer(state, pkg)
}

// parsePackageDefer does the actual parse of a package, handling subinclude deferral.
// It returns true if the parse was deferred due to pending subinclude() calls or false if it's ready immediately.
func parsePackageDefer(state *core.BuildState, pkg *core.Package) bool {
	deferred := parsePackageFile(state, pkg.Filename, pkg)
	if deferred {
		go pkg.WhenSubincludeReady(func() {
			parsePackageDefer(state, pkg)
		})
	} else {
		addTargetsToGraph(state, pkg)
	}
	return deferred
}

// addTargetsToGraph adds all the targets in a newly parsed package to the build graph.
func addTargetsToGraph(state *core.BuildState, pkg *core.Package) {
	for _, target := range pkg.Targets {
		state.Graph.AddTarget(target)
		for _, out := range target.DeclaredOutputs() {
			pkg.MustRegisterOutput(out, target)
		}
		for _, out := range target.TestOutputs {
			if !core.IsGlob(out) {
				pkg.MustRegisterOutput(out, target)
			}
		}
	}
	// Do this in a separate loop so we get intra-package dependencies right now.
	for _, target := range pkg.Targets {
		for _, dep := range target.DeclaredArchDependencies() {
			state.Graph.AddDependency(target.Label, dep)
		}
	}
	// Package is now ready to go.
	pkg.Ready()
}

func buildFileName(state *core.BuildState, pkgName string) string {
	// Bazel defines targets in its "external" package from its WORKSPACE file.
	// We will fake this by treating that as an actual package file...
	if state.Config.Bazel.Compatibility && pkgName == "external" {
		return "WORKSPACE"
	}
	for _, buildFileName := range state.Config.Please.BuildFileName {
		if filename := path.Join(pkgName, buildFileName); core.FileExists(filename) {
			return filename
		}
	}
	return ""
}

// Adds a single target to the build queue.
func addDep(state *core.BuildState, label, dependor core.BuildLabel, rescan, forceBuild bool) {
	// Stop at any package that's not loaded yet
	if state.Graph.Package(label.PackageName) == nil {
		if forceBuild {
			log.Debug("Adding forced pending parse of %s", label)
		}
		state.AddPendingParse(label, dependor, forceBuild)
		return
	}
	target := state.Graph.Target(label)
	if target == nil {
		log.Fatalf("Target %s (referenced by %s) doesn't exist\n", label, dependor)
	}
	if forceBuild {
		log.Debug("Forcing build of %s", label)
	}
	if target.State() >= core.Active && !rescan && !forceBuild {
		return // Target is already tagged to be built and likely on the queue.
	}
	// Only do this bit if we actually need to build the target
	if !target.SyncUpdateState(core.Inactive, core.Semiactive) && !rescan && !forceBuild {
		return
	}
	if state.NeedBuild || forceBuild {
		if target.SyncUpdateState(core.Semiactive, core.Active) {
			state.AddActiveTarget()
			if target.IsTest && state.NeedTests {
				state.AddActiveTarget() // Tests count twice if we're gonna run them.
			}
		}
	}
	// If this target has no deps, add it to the queue now, otherwise handle its deps.
	// Only add if we need to build targets (not if we're just parsing) but we might need it to parse...
	if target.State() == core.Active && state.Graph.AllDepsBuilt(target) {
		if target.SyncUpdateState(core.Active, core.Pending) {
			state.AddPendingBuild(label, dependor.IsAllTargets())
		}
		if !rescan {
			return
		}
	}
	for _, dep := range target.DeclaredArchDependencies() {
		// Check the require/provide stuff; we may need to add a different target.
		if len(target.Requires) > 0 {
			if depTarget := state.Graph.Target(dep); depTarget != nil && len(depTarget.Provides) > 0 {
				for _, provided := range depTarget.ProvideFor(target) {
					addDep(state, provided, label, false, forceBuild)
				}
				continue
			}
		}
		if forceBuild {
			log.Debug("Forcing build of dep %s -> %s", label, dep)
		}
		addDep(state, dep, label, false, forceBuild)
	}
}

// RunPreBuildFunction runs a pre-build callback function registered on a build target via pre_build = <...>.
//
// This is called before the target is built. It doesn't receive any output like the post-build one does but can
// be useful for other things; for example if you want to investigate a target's transitive labels to adjust
// its build command, you have to do that here (because in general the transitive dependencies aren't known
// when the rule is evaluated).
func RunPreBuildFunction(tid int, state *core.BuildState, target *core.BuildTarget) error {
	state.LogBuildResult(tid, target.Label, core.PackageParsing,
		fmt.Sprintf("Running pre-build function for %s", target.Label))
	pkg := state.Graph.Package(target.Label.PackageName)
	pkg.BuildCallbackMutex.Lock()
	defer pkg.BuildCallbackMutex.Unlock()
	pkg.CurrentArch = target.Label.Arch
	if err := runPreBuildFunction(pkg, target); err != nil {
		state.LogBuildError(tid, target.Label, core.ParseFailed, err, "Failed pre-build function for %s", target.Label)
		return err
	}
	rescanDeps(state, pkg)
	state.LogBuildResult(tid, target.Label, core.TargetBuilding,
		fmt.Sprintf("Finished pre-build function for %s", target.Label))
	return nil
}

// RunPostBuildFunction runs a post-build callback function registered on a build target via post_build = <...>.
//
// This is called after the target has been built and it is given the combined stdout/stderr of
// the build process. This output is passed to the post-build Python function which can then
// generate new targets or add dependencies to existing unbuilt targets.
func RunPostBuildFunction(tid int, state *core.BuildState, target *core.BuildTarget, out string) error {
	state.LogBuildResult(tid, target.Label, core.PackageParsing,
		fmt.Sprintf("Running post-build function for %s", target.Label))
	pkg := state.Graph.Package(target.Label.PackageName)
	pkg.BuildCallbackMutex.Lock()
	defer pkg.BuildCallbackMutex.Unlock()
	pkg.CurrentArch = target.Label.Arch
	log.Debug("Running post-build function for %s. Build output:\n%s", target.Label, out)
	if err := runPostBuildFunction(pkg, target, out); err != nil {
		state.LogBuildError(tid, target.Label, core.ParseFailed, err, "Failed post-build function for %s", target.Label)
		return err
	}
	rescanDeps(state, pkg)
	state.LogBuildResult(tid, target.Label, core.TargetBuilding,
		fmt.Sprintf("Finished post-build function for %s", target.Label))
	return nil
}

func rescanDeps(state *core.BuildState, pkg *core.Package) {
	// Run over all the targets in this package and ensure that any newly added dependencies enter the build queue.
	for _, target := range pkg.Targets {
		// TODO(pebers): this is pretty brutal; we're forcing a recheck of all dependencies
		//               in case we have any new targets. It'd be better to do it only for
		//               targets that need it but it's not easy to tell we're in a post build
		//               function at the point we'd need to do that.
		if !state.Graph.AllDependenciesResolved(target) {
			for _, dep := range target.DeclaredArchDependencies() {
				state.Graph.AddDependency(target.Label, dep)
			}
		}
		s := target.State()
		if s < core.Built && s > core.Inactive {
			addDep(state, target.Label, core.OriginalTarget, true, false)
		}
	}
}
