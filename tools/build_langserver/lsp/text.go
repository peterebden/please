package lsp

import (
	"strings"

	"github.com/sourcegraph/go-lsp"

	"github.com/thought-machine/please/src/parse/asp"
)

func (h *Handler) didOpen(params *lsp.DidOpenTextDocumentParams) (*struct{}, error) {
	uri := fromURI(params.TextDocument.URI)
	content := params.TextDocument.Text
	d := &doc{
		Content: strings.Split(content, "\n"),
	}
	h.parse(d, uri, content)
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.docs[uri] = d
	return nil, nil
}

// A doc is a representation of a document that's opened by the editor.
type doc struct {
	// The raw content of the document.
	Content []string
	// Parsed version of it
	Statements []*asp.Statement
}

// parse parses the given document and updates its statements.
func (h *Handler) parse(d *doc, uri, content string) {
	// The parser interface doesn't have ParseData which we want here.
	// We don't really want to make that a global thing on the state.
	if stmts, err := h.parser.ParseData([]byte(content), uri); err != nil {
		log.Warning("Error parsing %s: %s", uri, err)
	} else {
		d.Statements = stmts
	}
	// TODO(peterebden): We might want to add diagnostics here post-load.
}
