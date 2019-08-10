package asp

import (
	"reflect"
	"strings"
)

// FindTarget returns the top-level call in a BUILD file that corresponds to a target
// of the given name (or nil if one does not exist).
func FindTarget(statements []*Statement, name string) *Statement {
	for _, statement := range statements {
		if ident := statement.Ident; ident != nil && ident.Action != nil && ident.Action.Call != nil {
			for _, arg := range ident.Action.Call.Arguments {
				if arg.Name == "name" {
					if arg.Value.Val != nil && arg.Value.Val.String != "" && strings.Trim(arg.Value.Val.String, `"`) == name {
						return statement
					}
				}
			}
		}
	}
	return nil
}

// NextStatement finds the statement that follows the given one.
// This is often useful to find the extent of a statement in source code.
// It will return nil if there is not one following it.
func NextStatement(statements []*Statement, statement *Statement) *Statement {
	for i, s := range statements {
		if s == statement && i < len(statements)-1 {
			return statements[i+1]
		}
	}
	return nil
}

// GetExtents returns the "extents" of a statement, i.e. the lines that it covers in source.
// The caller must pass a value for the maximum extent of the file; we can't detect it here
// because the AST only contains positions for the beginning of the statements.
func GetExtents(statements []*Statement, statement *Statement, max int) (int, int) {
	next := NextStatement(statements, statement)
	if next == nil {
		// Assume it reaches to the end of the file
		return statement.Pos.Line, max
	}
	return statement.Pos.Line, next.Pos.Line - 1
}

// FindArgument finds an argument of any one of the given names, or nil if there isn't one.
// The statement must be a function call (e.g. as returned by FindTarget).
func FindArgument(statement *Statement, args ...string) *CallArgument {
	for i, a := range statement.Ident.Action.Call.Arguments {
		for _, arg := range args {
			if a.Name == arg {
				return &statement.Ident.Action.Call.Arguments[i]
			}
		}
	}
	return nil
}

// StatementOrExpressionFromAST is a wrapper around WalkAST to find the relevant statement
// and expression at a given position. Either may be null if they cannot be located.
func StatementOrExpressionFromAST(ast []*Statement, pos Position) (statement *Statement, expression *Expression) {
	WalkAST(ast, func(stmt *Statement) bool {
		if withinRange(pos, stmt.Pos, stmt.EndPos) {
			statement = stmt
			return true
		}
		return false
	}, func(expr *Expression) bool {
		if withinRange(pos, expr.Pos, expr.EndPos) {
			expression = expr
			return true
		}
		return false
	})
	return
}

// WalkAST is a generic function that walks through the ast recursively,
// It accepts two callback functions, one called on each statement encountered and
// one on each expression. Either can be nil.
// If the callback returns true, the node will be further visited; if false it (and
// all children) will be skipped.
func WalkAST(ast []*Statement, stmt func(*Statement) bool, expr func(*Expression) bool) {
	for _, node := range ast {
		walkAST(reflect.ValueOf(node), stmt, expr)
	}
}

func walkAST(v reflect.Value, stmt func(*Statement) bool, expr func(*Expression) bool) {
	callbacks := func(v reflect.Value) bool {
		if s, ok := v.Interface().(*Statement); ok && stmt != nil && !stmt(s) {
			return false
		} else if e, ok := v.Interface().(*Expression); ok && expr != nil && !expr(e) {
			return false
		}
		return true
	}

	if v.Kind() == reflect.Ptr && !v.IsNil() {
		if callbacks(v) {
			walkAST(v.Elem(), stmt, expr)
		}
	} else if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			walkAST(v.Index(i), stmt, expr)
		}
	} else if v.Kind() == reflect.Struct {
		if callbacks(v.Addr()) {
			for i := 0; i < v.NumField(); i++ {
				walkAST(v.Field(i), stmt, expr)
			}
		}
	}
}

// withinRange checks if the input position is within the range of the Expression
func withinRange(needle, start, end Position) bool {
	if needle.Line < start.Line || needle.Line > end.Line {
		return false
	} else if needle.Line == start.Line && needle.Column < start.Column {
		return false
	} else if needle.Line == end.Line && needle.Column > end.Column {
		return false
	}
	return true
}
