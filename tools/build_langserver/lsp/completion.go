package lsp

import (
	"github.com/sourcegraph/go-lsp"

	"github.com/thought-machine/please/src/parse/asp"
)

func (h *Handler) completion(params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	doc := h.doc(params.TextDocument.URI)
	pos := params.Position
}
