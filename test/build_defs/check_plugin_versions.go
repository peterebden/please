// check_plugin_versions verifies that the plugin revisions and Go toolchain version pinned by the
// e2e test repos match the canonical list in plugin_versions.json.
//
// The test repos are standalone - they get copied somewhere and run as their own repo - so they
// can't subinclude a shared definition and have to spell their versions out. That means the only
// way to stop them drifting is to check them, which is what this does.
//
// Modes:
//
//	-repo DIR        check every build file under DIR against the canonical list
//	-plugins FILE    check the canonical list against the please repo's own //plugins:BUILD
//	-toolchain FILE  check the canonical Go toolchain against //third_party/go:BUILD
//	-fix             rewrite the canonical list and every test repo to match the please repo
//
// -fix is what you want after bumping a plugin in //plugins:BUILD or the Go toolchain in
// //third_party/go:BUILD: it propagates that outwards so you don't have to find every test repo
// pinning the same thing by hand.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type versions struct {
	Plugins     map[string]string `json:"plugins"`
	GoToolchain string            `json:"go_toolchain"`
	Local       []string          `json:"local"`
}

var (
	pluginRepoRe  = regexp.MustCompile(`(?s)plugin_repo\(([^)]*)\)`)
	goToolchainRe = regexp.MustCompile(`(?s)go_toolchain\(([^)]*)\)`)
	nameRe        = regexp.MustCompile(`\bname\s*=\s*"([^"]*)"`)
	pluginRe      = regexp.MustCompile(`\bplugin\s*=\s*"([^"]*)"`)
	revisionRe    = regexp.MustCompile(`\brevision\s*=\s*"([^"]*)"`)
	versionRe     = regexp.MustCompile(`\bversion\s*=\s*"([^"]*)"`)
	hashesRe      = regexp.MustCompile(`(?s)\bhashes\s*=\s*\[[^\]]*\]`)
)

// fixCommand is the one thing to run to bring everything back into line. Printed on any failure so
// nobody has to work out which files to edit.
const fixCommand = "plz run //test/build_defs:fix_plugin_versions"

func main() {
	versionsFile := flag.String("versions", "", "Path to plugin_versions.json (required)")
	repo := flag.String("repo", "", "Root of a test repo to check")
	plugins := flag.String("plugins", "", "Path to the please repo's //plugins:BUILD")
	toolchain := flag.String("toolchain", "", "Path to the please repo's //third_party/go:BUILD")
	fix := flag.Bool("fix", false, "Rewrite the canonical list and every test repo to match the please repo")
	root := flag.String("root", ".", "Root of the please repo, for -fix")
	flag.Parse()

	if *versionsFile == "" {
		fmt.Fprintln(os.Stderr, "-versions is required")
		os.Exit(2)
	}

	if *fix {
		if err := runFix(*root, *versionsFile); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(2)
		}
		return
	}

	v, err := loadVersions(*versionsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(2)
	}

	var problems []string
	if *repo != "" {
		p, err := checkRepo(*repo, v, *versionsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(2)
		}
		problems = append(problems, p...)
	}
	if *plugins != "" {
		p, err := checkPlugins(*plugins, v, *versionsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(2)
		}
		problems = append(problems, p...)
	}
	if *toolchain != "" {
		p, err := checkToolchain(*toolchain, v, *versionsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(2)
		}
		problems = append(problems, p...)
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintf(os.Stderr, "\nTo fix all of these at once, from the repo root:\n\n    %s\n\n"+
			"That takes the versions in //plugins:BUILD and //third_party/go:BUILD as the truth and\n"+
			"rewrites %s and every test repo to match, so bumping a plugin only means editing the\n"+
			"please repo's own pin. `plz autofix` runs it too.\n",
			fixCommand, filepath.Base(*versionsFile))
		os.Exit(1)
	}
}

// runFix propagates the please repo's own versions outwards: first into the canonical list, then
// from there into every test repo. Doing it in that order means one edit to //plugins:BUILD is
// enough, and that the two checks can't disagree about which way the change should have flowed.
func runFix(root, versionsFile string) error {
	pluginsPath := filepath.Join(root, "plugins", "BUILD")
	toolchainPath := filepath.Join(root, "third_party", "go", "BUILD")

	rootPins, err := pluginPins(pluginsPath)
	if err != nil {
		return err
	}
	goVersion, goHashes, err := goToolchain(toolchainPath)
	if err != nil {
		return err
	}

	changed, err := fixVersionsFile(versionsFile, rootPins, goVersion)
	if err != nil {
		return err
	}

	// Re-read so the test repos are aligned to what the canonical list now says, which covers the
	// plugins the please repo doesn't use itself (proto, pleasings and so on).
	v, err := loadVersions(versionsFile)
	if err != nil {
		return err
	}

	testRepos, err := fixTestRepos(filepath.Join(root, "test"), v, goHashes)
	if err != nil {
		return err
	}
	changed = append(changed, testRepos...)

	if len(changed) == 0 {
		fmt.Println("Plugin versions are already in step; nothing to do.")
		return nil
	}
	fmt.Printf("Updated %d file(s):\n", len(changed))
	for _, c := range changed {
		fmt.Printf("  %s\n", c)
	}
	return nil
}

// fixVersionsFile rewrites the canonical list in place. It edits the text rather than re-marshalling
// so the comments and ordering in the file survive.
func fixVersionsFile(path string, rootPins map[string]string, goVersion string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	before := string(b)
	after := before

	v, err := loadVersions(path)
	if err != nil {
		return nil, err
	}
	for plugin, want := range rootPins {
		if _, shared := v.Plugins[plugin]; !shared {
			continue // Only the please repo uses it; nothing in the test repos to keep in step with.
		}
		re := regexp.MustCompile(`("` + regexp.QuoteMeta(plugin) + `"\s*:\s*)"[^"]*"`)
		after = re.ReplaceAllString(after, `${1}"`+want+`"`)
	}
	if goVersion != "" {
		re := regexp.MustCompile(`("go_toolchain"\s*:\s*)"[^"]*"`)
		after = re.ReplaceAllString(after, `${1}"`+goVersion+`"`)
	}

	if after == before {
		return nil, nil
	}
	if err := os.WriteFile(path, []byte(after), 0644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return []string{path}, nil
}

// fixTestRepos rewrites every pin under the test tree to match the canonical list. Unknown plugins
// are left alone - the check reports those, since we've no way to guess what they should be.
func fixTestRepos(testDir string, v *versions, goHashes string) ([]string, error) {
	var changed []string
	err := filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "plz-out" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isBuildFile(d.Name()) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		before := string(b)

		after := pluginRepoRe.ReplaceAllStringFunc(before, func(call string) string {
			plugin := firstMatch(pluginRe, call)
			if plugin == "" {
				plugin = firstMatch(nameRe, call)
			}
			want, known := v.Plugins[plugin]
			if !known {
				return call
			}
			return revisionRe.ReplaceAllString(call, `revision = "`+want+`"`)
		})

		after = goToolchainRe.ReplaceAllStringFunc(after, func(call string) string {
			call = versionRe.ReplaceAllString(call, `version = "`+v.GoToolchain+`"`)
			if goHashes != "" {
				// The hashes are specific to the version, so they have to travel with it.
				call = hashesRe.ReplaceAllString(call, goHashes)
			}
			return call
		})

		if after == before {
			return nil
		}
		if err := os.WriteFile(path, []byte(after), 0644); err != nil {
			return err
		}
		changed = append(changed, path)
		return nil
	})
	return changed, err
}

// pluginPins reads the plugin revisions a build file pins, keyed by plugin name.
func pluginPins(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	pins := map[string]string{}
	for _, call := range pluginRepoRe.FindAllStringSubmatch(string(b), -1) {
		plugin := firstMatch(pluginRe, call[1])
		if plugin == "" {
			plugin = firstMatch(nameRe, call[1])
		}
		if plugin != "" {
			pins[plugin] = firstMatch(revisionRe, call[1])
		}
	}
	return pins, nil
}

// goToolchain reads the version and the whole hashes block from a go_toolchain rule.
func goToolchain(path string) (version, hashes string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("reading %s: %w", path, err)
	}
	call := goToolchainRe.FindString(string(b))
	if call == "" {
		return "", "", fmt.Errorf("no go_toolchain rule found in %s", path)
	}
	return firstMatch(versionRe, call), hashesRe.FindString(call), nil
}

func loadVersions(path string) (*versions, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	v := &versions{}
	if err := json.Unmarshal(b, v); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(v.Plugins) == 0 {
		return nil, fmt.Errorf("%s lists no plugins", path)
	}
	return v, nil
}

// checkRepo walks a test repo and checks every version it pins.
func checkRepo(root string, v *versions, versionsFile string) ([]string, error) {
	local := map[string]bool{}
	for _, l := range v.Local {
		local[l] = true
	}

	var problems []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// plz-out is build output, not source we should be policing.
			if d.Name() == "plz-out" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isBuildFile(d.Name()) {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel := relative(root, path)

		for _, call := range pluginRepoRe.FindAllStringSubmatch(string(contents), -1) {
			body := call[1]
			// plugin_repo's subrepo takes its name from `plugin` where given, else `name`.
			plugin := firstMatch(pluginRe, body)
			if plugin == "" {
				plugin = firstMatch(nameRe, body)
			}
			if plugin == "" || local[plugin] {
				continue
			}
			want, known := v.Plugins[plugin]
			if !known {
				problems = append(problems, fmt.Sprintf(
					"%s: plugin %q is not in %s.\n"+
						"\tAdd it under \"plugins\" so its version is kept in step with the other test repos,\n"+
						"\tor under \"local\" if it's served from inside this repo and has no upstream.",
					rel, plugin, versionsFile))
				continue
			}
			got := firstMatch(revisionRe, body)
			if got != want {
				problems = append(problems, fmt.Sprintf(
					"%s: plugin %q is pinned to %q, expected %q (from %s).",
					rel, plugin, got, want, versionsFile))
			}
		}

		for _, call := range goToolchainRe.FindAllStringSubmatch(string(contents), -1) {
			got := firstMatch(versionRe, call[1])
			if got != v.GoToolchain {
				problems = append(problems, fmt.Sprintf(
					"%s: go_toolchain is version %q, expected %q (from %s).\n"+
						"\tRemember the hashes are version-specific and need updating too.",
					rel, got, v.GoToolchain, versionsFile))
			}
		}
		return nil
	})
	return problems, err
}

// checkPlugins verifies the canonical list agrees with the please repo's own plugin pins, so that
// this file doesn't become yet another version of the truth that drifts on its own.
func checkPlugins(path string, v *versions, versionsFile string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var problems []string
	for _, call := range pluginRepoRe.FindAllStringSubmatch(string(contents), -1) {
		body := call[1]
		plugin := firstMatch(pluginRe, body)
		if plugin == "" {
			plugin = firstMatch(nameRe, body)
		}
		want, shared := v.Plugins[plugin]
		if !shared {
			continue // The please repo uses plugins the test repos don't; that's fine.
		}
		if got := firstMatch(revisionRe, body); got != want {
			problems = append(problems, fmt.Sprintf(
				"%s pins plugin %q to %q but %s says %q.\n"+
					"\tThe test repos build against the same plugins as this repo, so these have to agree.",
				path, plugin, got, versionsFile, want))
		}
	}
	return problems, nil
}

func checkToolchain(path string, v *versions, versionsFile string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	call := goToolchainRe.FindStringSubmatch(string(contents))
	if call == nil {
		return nil, fmt.Errorf("no go_toolchain rule found in %s", path)
	}
	if got := firstMatch(versionRe, call[1]); got != v.GoToolchain {
		return []string{fmt.Sprintf(
			"%s uses Go %q but %s says %q.\n"+
				"\tThe test repos should build with the same Go as this repo.",
			path, got, versionsFile, v.GoToolchain)}, nil
	}
	return nil, nil
}

// isBuildFile covers the various names the test repos give their build files.
func isBuildFile(name string) bool {
	return name == "BUILD" || name == "BUILD_FILE" || strings.HasPrefix(name, "BUILD.")
}

func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// relative renders a path relative to the repo being checked, since the absolute one is a test
// sandbox directory that means nothing to whoever has to fix this.
func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
