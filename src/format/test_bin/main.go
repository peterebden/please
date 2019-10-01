// Package main implements a simple binary that we can use like an autoformatter.
package main

import (
	"fmt"
	"strings"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/fs"
)

var opts struct {
	InPlace bool `short:"w" long:"inplace" description:"Rewrite file in-place"`
	Args    struct {
		Filenames []string `required:"true" description:"Files to rewrite"`
	} `positional-args:"true"`
}

const content = "correctly formatted\n"

func main() {
	cli.ParseFlagsOrDie("test", &opts)
	for _, filename := range opts.Args.Filenames {
		if opts.InPlace {
			if err := fs.WriteFile(strings.NewReader(content), filename, 0644); err != nil {
				panic(err)
			}
		} else {
			fmt.Print(content)
		}
	}
}
