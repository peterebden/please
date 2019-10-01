// Package main implements a simple binary that we can use like an autoformatter.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/thought-machine/please/src/cli"
)

var opts struct {
	InPlace bool `short:"w" long:"inplace" description:"Rewrite file in-place"`
	Args    struct {
		Filenames []string `required:"true" description:"Files to rewrite"`
	} `positional-args:"true"`
}

const content = "correctly formatted"

func main() {
	cli.ParseFlagsOrDie("test", &opts)
	for _, filename := range opts.Args.Filenames {
		if opts.InPlace {
			ioutil.WriteFile(filename, content, 0644)
		} else {
			fmt.Println(content)
		}
	}
}
