package lsp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/sourcegraph/go-lsp"

	"github.com/thought-machine/please/src/parse/asp"
)

// A doc is a representation of a document that's opened by the editor.
type doc struct {
	// The raw content of the document.
	Content []string
	// Parsed version of it
	Statements []*asp.Statement
	Mutex      sync.Mutex
}

func (h *Handler) didOpen(params *lsp.DidOpenTextDocumentParams) (*struct{}, error) {
	uri := fromURI(params.TextDocument.URI)
	content := params.TextDocument.Text
	d := &doc{
		Content: strings.Split(content, "\n"),
	}
	go h.parse(d, uri, content)
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.docs[uri] = d
	return nil, nil
}

// parse parses the given document and updates its statements.
func (h *Handler) parse(d *doc, uri, content string) {
	// The parser interface doesn't have ParseData which we want here.
	// We don't really want to make that a global thing on the state.
	if stmts, err := h.parser.ParseData([]byte(content), uri); err != nil {
		log.Warning("Error parsing %s: %s", uri, err)
	} else {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()
		d.Statements = stmts
	}
	// TODO(peterebden): We might want to add diagnostics here post-load.
}

func (h *Handler) didChange(params *lsp.DidChangeTextDocumentParams) (*struct{}, error) {
	uri := fromURI(params.TextDocument.URI)
	h.mutex.Lock()
	d := h.docs[uri]
	h.mutex.Unlock()
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	// Synchronise changes into the doc's contents
	for _, change := range params.ContentChanges {
		if change.Range != nil {
			return nil, fmt.Errorf("non-incremental change received")
		}
		d.Content = strings.Split(change.Text, "\n")
		go h.parse(d, uri, change.Text)
	}
	return nil, nil
}
