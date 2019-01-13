// Package main implements a build provider for Please that understands Go files.
//
// N.B. This cannoy depend on any third-party packages since it is fundamental to the
//      go_get rule that would fetch them.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/thought-machine/please/tools/go_provider/mod"
	"github.com/thought-machine/please/tools/go_provider/provide"
)

type Request struct {
	Rule    string   `json:"rule"`
	Options []string `json:"options"`
}

type Response struct {
	Rule      string   `json:"rule"`
	Success   bool     `json:"success"`
	Messages  []string `json:"messages"`
	BuildFile string   `json:"build_file"`
}

func provideFile(ch chan<- *Response, rule, arg string, f func(string) (string, error)) {
	contents, err := f(arg)
	resp := &Response{
		Rule:      rule,
		BuildFile: contents,
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
		if len(req.Options) > 1 && req.Options[0] == "mod" {
			go provideMod(ch, req.Rule, req.Options[1], mod.Provide)
		} else {
			go provideFile(ch, req.Rule, req.Rule, provide.Parse)
		}
	}
}
