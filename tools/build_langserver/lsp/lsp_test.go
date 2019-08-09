package lsp

import (
	"encoding/json"
	"testing"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	h := NewHandler()
	result := &lsp.InitializeResult{}
	err := h.Request("initialize", &lsp.InitializeParams{
		Capabilities: lsp.ClientCapabilities{},
		RootURI:      lsp.DocumentURI("file://tools/build_langserver/lsp/test_data"),
	}, result)
	assert.NoError(t, err)
	assert.True(t, result.Capabilities.TextDocumentSync.Options.OpenClose)
}

func TestInitializeNoURI(t *testing.T) {
	h := NewHandler()
	result := &lsp.InitializeResult{}
	err := h.Request("initialize", &lsp.InitializeParams{
		Capabilities: lsp.ClientCapabilities{},
	}, result)
	assert.Error(t, err)
}

// Request is a slightly higher-level wrapper for testing that handles JSON serialisation.
func (h *Handler) Request(method string, req, resp interface{}) *jsonrpc2.Error {
	b, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("failed to encode request: %s", err)
	}
	msg := json.RawMessage(b)
	i, e := h.handle(method, &msg)
	if e != nil {
		return e
	}
	// serialise and deserialise, great...
	b, err = json.Marshal(i)
	if err != nil {
		log.Fatalf("failed to encode response: %s", err)
	} else if err := json.Unmarshal(b, resp); err != nil {
		log.Fatalf("failed to decode response: %s", err)
	}
	return e
}
