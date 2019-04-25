// Package main implements a provider of Go build files. It is typically used within
// go_module rules to provide BUILD files for the newly fetched libraries.
//
// Since this is part of the mechanism that fetches third-party libraries, it cannot
// itself depend on any and must limit itself to the standard library only.
package main

import (
	"log"
	"os"

	"github.com/thought-machine/please/tools/please_go_provide/provide"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s <import path> <directory> [//dependency1 //dependency2 ...]", os.Args[0])
	}
	if err := provide.Write(os.Args[1], os.Args[2], os.Args[2:]); err != nil {
		log.Fatalf("Failed to write BUILD files: %s", err)
	}
}
