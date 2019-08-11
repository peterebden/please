package lsp

import (
	"path"
	"strings"

	"github.com/sourcegraph/go-lsp"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/parse/asp"
)

func (h *Handler) completion(params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	doc := h.doc(params.TextDocument.URI)
	pos := params.Position
	if doc.AST == nil {
		h.parse(doc, doc.Text())
	}
	exprs := asp.ExpressionsAtPos(doc.AST, asp.Position{Line: pos.Line + 1, Column: pos.Character + 1})
	if len(exprs) == 0 {
		// For now there are no completion suggestions available on incomplete ASTs.
		// TODO(peterebden): Do some kind of best-effort thing.
		return &lsp.CompletionList{}, nil
	} else if expr := exprs[len(exprs)-1]; expr.Val != nil && expr.Val.String != "" {
		if strings.HasPrefix(expr.Val.String, `"//`) || strings.HasPrefix(expr.Val.String, `":`) {
			return h.completeLabel(doc, stringLiteral(expr.Val.String))
		}
	}
	return &lsp.CompletionList{}, nil
}

// completeLabel provides completions for a thing that looks like a build label.
func (h *Handler) completeLabel(doc *doc, partial string) (*lsp.CompletionList, error) {
	list := &lsp.CompletionList{}
	if idx := strings.IndexByte(partial, ':'); idx != -1 {
		// We know exactly which package it's in. "Just" look in there.
		labelName := partial
		if idx == len(labelName)-1 {
			labelName += "all" // Won't be a valid build label without this.
		}
		pkgName := path.Base(doc.Filename)
		pkgLabel := core.BuildLabel{PackageName: pkgName, Name: "all"}
		label, err := core.TryParseBuildLabel(labelName, pkgName)
		if err != nil {
			return nil, err
		}
		m := map[string]bool{}
		if pkg := h.state.Graph.PackageByLabel(label); pkg != nil {
			for _, t := range pkg.AllTargets() {
				if ((label.Name == "all" && !strings.HasPrefix(t.Label.Name, "_")) || strings.HasPrefix(t.Label.Name, label.Name)) && pkgLabel.CanSee(h.state, t) {
					s := t.Label.ShortString(core.BuildLabel{PackageName: pkgName})
					if !strings.HasPrefix(s, partial) {
						s = t.Label.String() // Don't abbreviate it if we end up losing part of what's there
					}
					list.Items = append(list.Items, lsp.CompletionItem{
						Label:    s,
						Kind:     lsp.CIKText,
						TextEdit: &lsp.TextEdit{NewText: strings.TrimPrefix(s, partial)},
					})
					m[s] = true
				}
			}
		}
		if idx == 0 || pkgName == label.PackageName {
			// We are in the current document, provide local completions from it.
			// This handles the case where a user added something locally but hasn't saved it yet.
			for _, target := range h.allTargets(doc) {
				if (label.Name == "all" && !strings.HasPrefix(label.Name, "_")) || strings.HasPrefix(target, label.Name) {
					if s := ":" + target; !m[s] {
						list.Items = append(list.Items, lsp.CompletionItem{
							Label:    s,
							Kind:     lsp.CIKText,
							TextEdit: &lsp.TextEdit{NewText: strings.TrimPrefix(s, partial)},
						})
					}
				}
			}
		}
		return list, nil
	}
	// OK, it doesn't specify a package yet. Find any relevant ones.
	prefix := strings.TrimLeft(partial, "/")
	list.IsIncomplete = true
	for name := range h.state.Graph.PackageMap() {
		if strings.HasPrefix(name, prefix) {
			s := "//" + name + ":"
			list.Items = append(list.Items, lsp.CompletionItem{
				Label:    s,
				Kind:     lsp.CIKText,
				TextEdit: &lsp.TextEdit{NewText: strings.TrimPrefix(s, partial)},
			})
		}
	}
	return list, nil
}

// allTargets provides a list of all target names for a document.
func (h *Handler) allTargets(doc *doc) []string {
	ret := []string{}
	asp.WalkAST(doc.AST, func(call *asp.Call) bool {
		for _, arg := range call.Arguments {
			if arg.Name == "name" && arg.Value.Val != nil && arg.Value.Val.String != "" {
				ret = append(ret, stringLiteral(arg.Value.Val.String))
			}
		}
		return false
	})
	return ret
}

// stringLiteral converts a parsed string literal (which is still surrounded by quotes) to an unquoted version.
func stringLiteral(s string) string {
	return s[1 : len(s)-1]
}
