package test_data

import "embed"

//go:embed hello.txt
var hello string

//go:embed files/*.txt
var txt embed.FS
