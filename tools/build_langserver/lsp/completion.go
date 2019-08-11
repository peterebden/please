package lsp

import (
	"path"
	"path/filepath"
	"strings"
	"unicode"

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
	if exprs := asp.ExpressionsAtPos(doc.AST, asp.Position{Line: pos.Line + 1, Column: pos.Character + 1}); len(exprs) == 0 {
		return h.completeUnparsed(doc, pos.Line, pos.Character)
	} else if expr := exprs[len(exprs)-1]; expr.Val != nil && expr.Val.String != "" {
		return h.completeString(doc, stringLiteral(expr.Val.String), pos.Line, pos.Character)
	}
	// TODO(peterebden): Completion of non-string names (functions etc)
	return &lsp.CompletionList{}, nil
}

// completeLabel provides completions for a thing that looks like a build label.
func (h *Handler) completeLabel(doc *doc, partial string, line, col int) (*lsp.CompletionList, error) {
	if partial == "//" {
		return &lsp.CompletionList{}, nil // Don't complete the whole repo...
	}
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
					list.Items = append(list.Items, completionItem(s, partial, line, col))
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
						list.Items = append(list.Items, completionItem(s, partial, line, col))
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
			list.Items = append(list.Items, completionItem(s, partial, line, col))
		}
	}
	return list, nil
}

// completeUnparsed does a best-effort completion when we don't have an AST to work from.
func (h *Handler) completeUnparsed(doc *doc, line, col int) (*lsp.CompletionList, error) {
	lines := doc.Lines()
	l := lines[line][:col] // Don't care about anything after the column we're at
	if strings.Count(l, `"`)%2 == 1 {
		// Odd number of quotes in the line, so assume the last one is unclosed.
		return h.completeString(doc, l[strings.LastIndexByte(l, '"')+1:], line, col)
	} else if strings.Count(l, `'`)%2 == 1 {
		// Same thing but with single quotes; they aren't canonical formatting but
		// they are legal to use.
		return h.completeString(doc, l[strings.LastIndexByte(l, '\'')+1:], line, col)
	}
	// Not (apparently) in a string, take the last lexical token
	r := []rune(l) // unicode!
	for i := len(r) - 1; i >= 0; i-- {
		if !unicode.IsLetter(r[i]) && r[i] != '_' {
			return h.completeIdent(doc, string(r[i+1:]), line, col)
		}
	}
	return h.completeIdent(doc, l, line, col)
}

// completeString completes a string literal, either as a build label or as a file.
func (h *Handler) completeString(doc *doc, s string, line, col int) (*lsp.CompletionList, error) {
	if s == "" {
		return &lsp.CompletionList{}, nil
	} else if core.LooksLikeABuildLabel(s) {
		return h.completeLabel(doc, s, line, col)
	}
	// Not a label, assume file.
	matches, _ := filepath.Glob(path.Join(h.root, path.Dir(doc.Filename), s+"*"))
	list := &lsp.CompletionList{
		Items: make([]lsp.CompletionItem, len(matches)),
	}
	for i, match := range matches {
		list.Items[i] = completionItem(match, s, line, col)
	}
	return list, nil
}

// completeIdent completes an arbitrary identifier
func (h *Handler) completeIdent(doc *doc, s string, line, col int) (*lsp.CompletionList, error) {
	list := &lsp.CompletionList{}
	for name, f := range h.builtins {
		if strings.HasPrefix(name, s) {
			item := completionItem(name, s, line, col)
			item.Documentation = f.Docstring
			list.Items = append(list.Items, item)
		}
	}
	// TODO(peterebden): Additional text edits for non-builtin functions
	// TODO(peterebden): Completion of arguments
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

func completionItem(label, prefix string, line, col int) lsp.CompletionItem {
	return lsp.CompletionItem{
		Label:            label,
		Kind:             lsp.CIKValue,
		InsertTextFormat: lsp.ITFPlainText,
		TextEdit: &lsp.TextEdit{
			NewText: strings.TrimPrefix(label, prefix),
			Range: lsp.Range{
				Start: lsp.Position{Line: line, Character: col},
				End:   lsp.Position{Line: line, Character: col},
			},
		},
	}
}
