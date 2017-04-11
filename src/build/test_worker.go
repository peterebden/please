// Package main implements a very minimal remote worker that is used in worker_test.go.
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/golang/protobuf/proto"

	pb "build/proto/worker"
)

func main() {
	for {
		var l int32
		if err := binary.Read(os.Stdin, binary.LittleEndian, &l); err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		b := make([]byte, l)
		if _, err := os.Stdin.Read(b); err != nil {
			panic(err)
		}
		req := pb.BuildRequest{}
		if err := proto.Unmarshal(b, &req); err != nil {
			panic(err)
		}
		resp := pb.BuildResponse{
			Rule:    req.Rule,
			Success: true,
		}
		for _, label := range req.Labels {
			if label == "cobol" {
				resp.Success = false
				fmt.Fprintln(os.Stderr, "COBOL is not supported, you must be joking")
				resp.Messages = append(resp.Messages, "COBOL is not supported, you must be joking")
			}
		}
		b, err := proto.Marshal(&resp)
		if err != nil {
			panic(err)
		}
		if err := binary.Write(os.Stdout, binary.LittleEndian, int32(len(b))); err != nil {
			panic(err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			panic(err)
		}
	}
}
