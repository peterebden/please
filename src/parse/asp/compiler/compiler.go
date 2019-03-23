// Package compiler implements an ahead-of-time compiler to Go code.
// Right now this only works for the builtin rules, mostly because we don't have
// support for Go plugins.
package compiler

import (
	"bytes"
	"fmt"

	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/parse/asp"
)

var log = logging.MustGetLogger("compiler")

// TypesFile is the contents of a standalone file that is assumed to be compiled with
// the others. It contains one-off type definitions to keep the others clean.
const TypesFile = `
package rules

import "github.com/thought-machine/please/src/parse/asp"

type Object = asp.PyObject
type Bool = asp.PyBool
type Int = asp.PyInt
type String = asp.PyString
type List = asp.PyList
type Dict = asp.PyDict
type Func = asp.PyFunc
type Config = asp.PyConfig
type Scope = asp.Scope

var NewFunc = asp.NewFunc
`

// Compile compiles a single input.
func Compile(statements []*asp.Statement) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	c := compiler{}
	c.Emitln("package rules")
	c.Emitln("")
	c.Emitln("func Rules(s *Scope) {")
	c.CompileStatements(statements)
	c.Emitln("}")
	return c.w.Bytes(), nil
}

type compiler struct {
	w      bytes.Buffer
	indent string
}

func (c *compiler) Error(pos asp.Position, msg string, args ...interface{}) {
	panic(fmt.Errorf("%s:%d:%d: %s", pos.Filename, pos.Line, pos.Column, fmt.Sprintf(msg, args...)))
}

func (c *compiler) Indent() {
	c.indent += "    "
}

func (c *compiler) Unindent() {
	c.indent = c.indent[4:]
}

func (c *compiler) Emitln(s string) {
	c.EmitIndent()
	c.w.WriteString(s)
	c.w.WriteByte('\n')
}

func (c *compiler) Emitfi(format string, args ...interface{}) {
	c.EmitIndent()
	c.Emitf(format, args...)
}

func (c *compiler) Emitf(format string, args ...interface{}) {
	c.w.WriteString(fmt.Sprintf(format, args...))
}

func (c *compiler) EmitIndent() {
	c.w.WriteString(c.indent)
}

func (c *compiler) CompileStatements(stmts []*asp.Statement) {
	c.Indent()
	defer c.Unindent()
	for _, stmt := range stmts {
		if stmt.FuncDef != nil {
			c.compileFunc(stmt.FuncDef)
		} else if stmt.If != nil {
			c.compileIf(stmt.If)
		} else if stmt.For != nil {
			c.compileFor(stmt.For)
		} else if stmt.Return != nil {
			c.Emitfi("return ")
			c.compileExprs(stmt.Return.Values)
		} else if stmt.Raise != nil {
			c.Emitfi("panic(")
			c.compileExpr(stmt.Raise)
			c.Emitf(")\n")
		} else if stmt.Assert != nil {
			c.compileIf(&asp.IfStatement{
				Condition: *stmt.Assert.Expr,
				Statements: []*asp.Statement{
					&asp.Statement{Raise: &asp.Expression{Val: &asp.ValueExpression{String: stmt.Assert.Message}}},
				},
			})
		} else if stmt.Ident != nil {
			c.compileIdentStatement(stmt.Ident)
		} else if stmt.Literal != nil {
			c.Error(stmt.Pos, "Expression has no effect")
		} else if stmt.Pass {
		} else if stmt.Continue {
			c.Emitln("continue")
		} else {
			c.Error(stmt.Pos, "Unhandled statement")
		}
	}
}

func (c *compiler) compileFunc(def *asp.FuncDef) {
	c.Emitfi(`s.Set("%s", NewFunc("%s", s,`, def.Name, def.Name)
	c.Emitf("\n")
	c.Indent()
	c.Emitfi("[]string{")
	for _, arg := range def.Arguments {
		c.Emitf(`"%s",`, arg.Name)
	}
	c.Emitf("},\n")
	c.Emitfi("map[string]int{")
	for i, arg := range def.Arguments {
		c.Emitf(`"%s": %d,`, arg.Name, i)
	}
	c.Emitf("},\n")
	c.Emitfi("[]PyObject{")
	for _, arg := range def.Arguments {
		if arg.Value == nil {
			c.Emitf("nil, ")
		} else {
			c.compileExpr(arg.Value)
		}
	}
	c.Emitf("},\n")
	c.Emitfi("[][]string{")
	for _, arg := range def.Arguments {
		c.Emitf("{")
		for _, t := range arg.Type {
			c.Emitf(`"%s",`, t)
		}
		c.Emitf("},")
	}
	c.Emitf("},\n")
	c.Emitfi(`"%s",`, def.Return)
	c.Emitf("\n")
	c.Emitfi("func (s *scope, args []PyObject) PyObject {\n")
	c.CompileStatements(def.Statements)
	c.Emitln("},")
	c.Unindent()
	c.Emitln("))")
}

func (c *compiler) compileIf(ifs *asp.IfStatement) {
	c.Emitfi("if ")
	c.compileExpr(&ifs.Condition)
	c.Emitf(" {\n")
	c.CompileStatements(ifs.Statements)
	for _, elif := range ifs.Elif {
		c.Emitfi("} else if ")
		c.compileExpr(&elif.Condition)
		c.Emitf(" {\n")
		c.CompileStatements(elif.Statements)
	}
	if len(ifs.ElseStatements) > 0 {
		c.Emitln("} else {")
		c.CompileStatements(ifs.ElseStatements)
	}
	c.Emitln("}")
}

func (c *compiler) compileFor(f *asp.ForStatement) {
	x := "x_"
	if len(f.Names) == 1 {
		x = f.Names[0]
	}
	c.Emitfi("for _, %s := range ", x)
	c.compileExpr(&f.Expr)
	c.Emitf(" {\n")
	if len(f.Names) > 1 {
		for i, name := range f.Names {
			c.Emitfi("    %s = %s[%d]\n", name, x, i)
		}
	}
	c.CompileStatements(f.Statements)
	c.Emitln("}")
}

func (c *compiler) compileIdentStatement(ident *asp.IdentStatement) {

}

func (c *compiler) compileExprs(exprs []*asp.Expression) {
	if len(exprs) == 1 {
		c.compileExpr(exprs[0])
		return
	}
	c.Emitf("List{")
	for _, expr := range exprs {
		c.compileExpr(expr)
		c.Emitf(", ")
	}
	c.Emitf("}\n")
}

func (c *compiler) compileExpr(expr *asp.Expression) {

}
