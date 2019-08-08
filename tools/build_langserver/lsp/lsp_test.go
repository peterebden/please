package lsp

import (
	"testing"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
)

func newServer() *jsonrpc2.Conn {
	h := NewHandler()
	c := jsonrpc2.NewConn(
			jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),

}
