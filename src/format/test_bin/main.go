// Package main implements a simple binary that we can use like an autoformatter.
package main

import (
	"flag"
	"io/ioutil"
)

func main() {
	inplace := flag.Bool("w", false, "Write file in-place")
}
