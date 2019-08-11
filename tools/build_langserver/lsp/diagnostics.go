package lsp

import (
	"context"
	"path"
	"time"

	"github.com/sourcegraph/go-lsp"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/parse/asp"
)

// diagSource
const diagSource = "plz tool langserver"

func (h *Handler) diagnose(d *doc, ast []*asp.Statement) {
	diags := []lsp.Diagnostic{}
	pkgLabel := core.BuildLabel{
		PackageName: path.Dir(d.Filename),
		Name:        "all",
	}
	asp.WalkAST(ast, func(expr *asp.Expression) bool {
		if expr.Val != nil && expr.Val.String != "" {
			if s := stringLiteral(expr.Val.String); core.LooksLikeABuildLabel(s) {
				if l, err := core.TryParseBuildLabel(s, pkgLabel.PackageName); err == nil {
					if t := h.state.Graph.Target(l); t != nil {
						if !pkgLabel.CanSee(h.state, t) {
							diags = append(diags, lsp.Diagnostic{
								Range: lsp.Range{
									// -1 because asp.Positions are 1-indexed but lsp Positions are 0-indexed.
									// Further fiddling on Column to fix quotes.
									Start: lsp.Position{Line: expr.Pos.Line - 1, Character: expr.Pos.Column},
									End:   lsp.Position{Line: expr.EndPos.Line - 1, Character: expr.EndPos.Column - 1},
								},
								Severity: lsp.Error,
								Source:   diagSource,
								Message:  "Target " + t.Label.String() + " is not visible to this package",
							})
						}
					} else if h.state.Graph.PackageByLabel(l) != nil {
						// Package exists but target doesn't, issue a diagnostic for that.
						diags = append(diags, lsp.Diagnostic{
							Range: lsp.Range{
								Start: lsp.Position{Line: expr.Pos.Line - 1, Character: expr.Pos.Column},
								End:   lsp.Position{Line: expr.EndPos.Line - 1, Character: expr.EndPos.Column - 1},
							},
							Severity: lsp.Error,
							Source:   diagSource,
							Message:  "Target " + s + " does not exist",
						})
					}
				}
			}
			return false
		}
		return true
	})
	// Always publish here; if we have none we might have published before, and we would
	// then need to clear them.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h.Conn.Notify(ctx, "textDocument/publishDiagnostics", &lsp.PublishDiagnosticsParams{
		URI:         lsp.DocumentURI("file://" + path.Join(h.root, d.Filename)),
		Diagnostics: diags,
	})
}
