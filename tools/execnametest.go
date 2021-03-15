package main

import (
	"fmt"
	"os"
)

func main() {
	exec, err := os.Executable()
	fmt.Printf("%s %s\n", exec, err)
}
