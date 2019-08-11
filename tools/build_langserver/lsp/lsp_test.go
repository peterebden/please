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

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/core"
)

func init() {
	cli.InitLogging(6)
}

func TestInitialize(t *testing.T) {
	h := NewHandler()
	result := &lsp.InitializeResult{}
	err := h.Request("initialize", &lsp.InitializeParams{
		Capabilities: lsp.ClientCapabilities{},
		RootURI:      lsp.DocumentURI("file://tools/build_langserver/lsp/test_data"),
	}, result)
	assert.NoError(t, err)
	assert.True(t, result.Capabilities.TextDocumentSync.Options.OpenClose)
	assert.Equal(t, path.Join(os.Getenv("TEST_DIR"), "tools/build_langserver/lsp/test_data"), h.root)
}

func TestInitializeNoURI(t *testing.T) {
	h := NewHandler()
	result := &lsp.InitializeResult{}
	err := h.Request("initialize", &lsp.InitializeParams{
		Capabilities: lsp.ClientCapabilities{},
	}, result)
	assert.Error(t, err)
}

const testContent = `
go_library(
    name = "lsp",
    srcs = ["lsp.go"],
    deps = [
        "//third_party/go:lsp",
    ],
)
`

const testContent2 = `
go_library(
    name = "lsp",
    srcs = ["lsp.go"],
    deps = [
        "//third_party/go:lsp",
    ],
)

go_test(
    name = "lsp_test",
    srcs = ["lsp_test.go"],
    deps = [
        ":lsp",
        "//third_party/go:testify",
    ],
)
`

func TestDidOpen(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testContent,
		},
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, testContent, h.CurrentContent("test/test.build"))
}

func TestDidChange(t *testing.T) {
	// TODO(peterebden): change this when we support incremental changes.
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testContent,
		},
	}, nil)
	assert.NoError(t, err)
	err = h.Request("textDocument/didChange", &lsp.DidChangeTextDocumentParams{
		TextDocument: lsp.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
		},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{
			{
				Text: testContent2,
			},
		},
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, testContent2, h.CurrentContent("test/test.build"))
}

func TestDidSave(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testContent,
		},
	}, nil)
	assert.NoError(t, err)
	err = h.Request("textDocument/didSave", &lsp.DidSaveTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: "file://test/test.build",
		},
	}, nil)
	assert.NoError(t, err)
}

func TestDidClose(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testContent,
		},
	}, nil)
	assert.NoError(t, err)
	err = h.Request("textDocument/didClose", &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: "file://test/test.build",
		},
	}, nil)
	assert.NoError(t, err)
}

const testFormattingContent = `go_test(
    name = "lsp_test",
    srcs = ["lsp_test.go"],
    deps = [":lsp","//third_party/go:testify"],
)
`

func TestFormatting(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testFormattingContent,
		},
	}, nil)
	assert.NoError(t, err)
	edits := []lsp.TextEdit{}
	err = h.Request("textDocument/formatting", &lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: "file://test/test.build",
		},
	}, &edits)
	assert.NoError(t, err)
	assert.Equal(t, []lsp.TextEdit{
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 3, Character: 0},
				End:   lsp.Position{Line: 3, Character: 47},
			},
			NewText: `    deps = [`,
		},
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 4, Character: 0},
				End:   lsp.Position{Line: 4, Character: 1},
			},
			NewText: `        ":lsp",`,
		},
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 0},
				End:   lsp.Position{Line: 5, Character: 0},
			},
			NewText: `        "//third_party/go:testify",`,
		},
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 6, Character: 0},
				End:   lsp.Position{Line: 6, Character: 0},
			},
			NewText: "    ],\n)\n",
		},
	}, edits)
}

func TestShutdown(t *testing.T) {
	h := initHandler()
	c := &closer{}
	h.Conn = c
	err := h.Request("shutdown", &struct{}{}, nil)
	assert.NoError(t, err)
	// Shouldn't be closed yet
	assert.False(t, c.Closed)
	err = h.Request("exit", &struct{}{}, nil)
	assert.NoError(t, err)
	assert.True(t, c.Closed)
}

type closer struct {
	Closed bool
}

func (c *closer) Close() error {
	c.Closed = true
	return nil
}

const testCompletionContent = `
go_library(
    name = "test",
    srcs = glob(["*.go"]),
    deps = [
        "//src/core:"
    ],
)
`

func TestCompletion(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testCompletionContent,
		},
	}, nil)
	assert.NoError(t, err)
	h.WaitForPackage("src/core")
	completions := &lsp.CompletionList{}
	err = h.Request("textDocument/completion", &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
			Position: lsp.Position{
				Line:      5,
				Character: 20,
			},
		},
	}, completions)
	assert.NoError(t, err)
	assert.Equal(t, &lsp.CompletionList{
		IsIncomplete: false,
		Items: []lsp.CompletionItem{
			{
				Label:            "//src/core:core",
				Kind:             lsp.CIKText,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "core"},
			},
		},
	}, completions)
}

const testCompletionContentInMemory = `
go_library(
    name = "test",
    srcs = glob(["*.go"], exclude=["*_test.go"]),
    deps = [
        "//src/core"
    ],
)

go_test(
    name = "test_test",
    srcs = glob(["*_test.go"]),
    deps = [
        ":",
    ],
)
`

func TestCompletionInMemory(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testCompletionContentInMemory,
		},
	}, nil)
	assert.NoError(t, err)
	h.WaitForPackage("src/core")
	completions := &lsp.CompletionList{}
	err = h.Request("textDocument/completion", &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
			Position: lsp.Position{
				Line:      13,
				Character: 9,
			},
		},
	}, completions)
	assert.NoError(t, err)
	assert.Equal(t, &lsp.CompletionList{
		IsIncomplete: false,
		Items: []lsp.CompletionItem{
			{
				Label:            ":test",
				Kind:             lsp.CIKText,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "test"},
			},
			// TODO(peterebden): We should filter this out really...
			{
				Label:            ":test_test",
				Kind:             lsp.CIKText,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "test_test"},
			},
		},
	}, completions)
}

const testCompletionContentPartial = `
go_library(
    name = "test",
    srcs = glob(["*.go"]),
    deps = [
        "//src/core:
`

func TestCompletionPartial(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testCompletionContentPartial,
		},
	}, nil)
	assert.NoError(t, err)
	h.WaitForPackage("src/core")
	completions := &lsp.CompletionList{}
	err = h.Request("textDocument/completion", &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
			Position: lsp.Position{
				Line:      5,
				Character: 20,
			},
		},
	}, completions)
	assert.NoError(t, err)
	assert.Equal(t, &lsp.CompletionList{
		IsIncomplete: false,
		Items: []lsp.CompletionItem{
			{
				Label:            "//src/core:core",
				Kind:             lsp.CIKText,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "core"},
			},
		},
	}, completions)
}

const testCompletionContentFunction = `
go_library(
    name = "test",
    srcs = glob(["*.go"]),
    deps = [
        "//src/core:core",
    ],
)
`

func TestCompletionFunction(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testCompletionContentFunction,
		},
	}, nil)
	assert.NoError(t, err)
	h.WaitForPackage("src/core")
	completions := &lsp.CompletionList{}
	err = h.Request("textDocument/completion", &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
			Position: lsp.Position{
				Line:      1,
				Character: 6,
			},
		},
	}, completions)
	assert.NoError(t, err)
	assert.Equal(t, &lsp.CompletionList{
		IsIncomplete: false,
		Items: []lsp.CompletionItem{
			{
				Label:            "go_library",
				Kind:             lsp.CIKFunction,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "rary"},
				Detail:           h.builtins["go_library"].Docstring,
			},
		},
	}, completions)
}

const testCompletionContentPartialFunction = `
go_libr
`

func TestCompletionPartialFunction(t *testing.T) {
	h := initHandler()
	err := h.Request("textDocument/didOpen", &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  "file://test/test.build",
			Text: testCompletionContentPartialFunction,
		},
	}, nil)
	assert.NoError(t, err)
	h.WaitForPackage("src/core")
	completions := &lsp.CompletionList{}
	err = h.Request("textDocument/completion", &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: "file://test/test.build",
			},
			Position: lsp.Position{
				Line:      1,
				Character: 6,
			},
		},
	}, completions)
	assert.NoError(t, err)
	assert.Equal(t, &lsp.CompletionList{
		IsIncomplete: false,
		Items: []lsp.CompletionItem{
			{
				Label:            "go_library",
				Kind:             lsp.CIKFunction,
				InsertTextFormat: lsp.ITFPlainText,
				TextEdit:         &lsp.TextEdit{NewText: "rary"},
				Detail:           h.builtins["go_library"].Docstring,
			},
		},
	}, completions)
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

// WaitForPackage blocks until the given package has been parsed.
func (h *Handler) WaitForPackage(pkg string) {
	for result := range h.state.Results() {
		if result.Status == core.PackageParsed && result.Label.PackageName == pkg {
			return
		}
	}
}
