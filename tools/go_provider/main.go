// Package main implements a build provider for Please that understands Go files.
//
// N.B. This cannot depend on any third-party packages since it is fundamental to the
//      go_get rule that would fetch them.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/thought-machine/please/tools/go_provider/provide"
)

type Request struct {
	Rule    string   `json:"rule"`
	Options []string `json:"opts"`
}

type Response struct {
	Rule      string   `json:"rule"`
	Success   bool     `json:"success"`
	Messages  []string `json:"messages"`
	BuildFile string   `json:"build_file"`
}

func provideFile(ch chan<- *Response, rule, arg string) {
	contents, err := provide.ProvideDir(arg)
	resp := &Response{
		Rule:      rule,
		BuildFile: contents,
		Success:   err == nil,
	}
	if err != nil {
		resp.Messages = []string{err.Error()}
	}
	ch <- resp
}

func main() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	ch := make(chan *Response, 10)
	go func() {
		for resp := range ch {
			if err := encoder.Encode(resp); err != nil {
				log.Printf("Failed to encode message: %s", err)
			}
		}
	}()
	for {
		req := &Request{}
		if err := decoder.Decode(req); err != nil {
			log.Printf("Failed to decode incoming message: %s", err)
			continue
		}
		go provideFile(ch, req.Rule, req.Rule)
	}
}
