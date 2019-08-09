package lsp

import (
	"encoding/json"
	"os"
	"path"
	"strings"
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

func TestDidOpen(t *testing.T) {
	h := initHandler()
	const content = `
go_library(
    name = "test",
    srcs = ["lsp.go"],
    deps = [
        "//third_party/go:lsp",
    ],
)
`
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/BUILD",
			Text: content,
		},
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, content, h.CurrentContent("test/BUILD"))
}

// initHandler is a wrapper around creating a new handler and initializing it, which is
// more convenient for most tests.
func initHandler() *Handler {
	h := NewHandler()
	result := &lsp.InitializeResult{}
	if err := h.Request("initialize", &lsp.InitializeParams{
		Capabilities: lsp.ClientCapabilities{},
		RootURI:      lsp.DocumentURI("file://" + path.Join(os.Getenv("TEST_DIR"), "tools/build_langserver/lsp/test_data")),
	}, result); err != nil {
		log.Fatalf("init failed: %s", err)
	}
	return h
}

// Request is a slightly higher-level wrapper for testing that handles JSON serialisation.
func (h *Handler) Request(method string, req, resp interface{}) *jsonrpc2.Error {
	b, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("failed to encode request: %s", err)
	}
	msg := json.RawMessage(b)
	i, e := h.handle(method, &msg)
	if e != nil || resp == nil {
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

// CurrentContent returns the current contents of a document.
func (h *Handler) CurrentContent(doc string) string {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	d := h.docs[doc]
	if d == nil {
		log.Error("unknown doc %s", doc)
		return ""
	}
	return strings.Join(d.Content, "\n")
}
