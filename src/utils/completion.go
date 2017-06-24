// +build nobootstrap

package utils

import (
	"fmt"
	"os"
	"strings"
)

// PrintCompletionScript prints Please's completion script to stdout.
// If one of bash or zsh is passed then it prints that script, otherwise it attempts
// autodetection and returns false if the shell appears to be neither.
func PrintCompletionScript(bash, zsh bool) bool {
	if bash || zsh {
		return printCompletionScript(bash, zsh)
	} else {
		return printCompletionScript(bash || isShell("bash"), zsh || isShell("zsh"))
	}
}

func printCompletionScript(bash, zsh bool) bool {
	if zsh {
		fmt.Printf("%s\n", MustAsset("plz_complete.zsh"))
	} else if bash {
		fmt.Printf("%s\n", MustAsset("plz_complete.sh"))
	}
	return zsh || bash
}

// isShell returns true if the current shell appears to be the given type.
func isShell(shell string) bool {
	// This is not entirely ideal; $SHELL is the user's preferred login shell, not necessarily
	// the current shell we are running under. It does not seem easy to find that from a subprocess
	// though without doing system-specific stuff to find the ppid's command, so for now we
	// assume that the combination of login shell & override flags is sufficient.
	return strings.Contains(os.Getenv("SHELL"), shell)
}
