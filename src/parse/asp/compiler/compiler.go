// Package compiler implements an ahead-of-time compiler to Go code.
// Right now this only works for the builtin rules, mostly because we don't have
// support for Go plugins.
package compiler

import (
	"bytes"
	"fmt"
	"strings"

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
var True = asp.True
var False = asp.False
var None = asp.None
`

// Compile compiles a single input.
func Compile(statements []*asp.Statement) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	c := compiler{
		w:         &bytes.Buffer{},
		functions: map[string]*asp.FuncDef{},
		locals:    map[string]local{},
	}
	// Grab all the function definitions now so we can call between them accurately.
	for _, stmt := range statements {
		if stmt.FuncDef != nil {
			c.functions[stmt.FuncDef.Name] = stmt.FuncDef
		}
	}
	c.Emitln("package rules")
	c.Emitln("")
	c.Emitln("func Rules(s *Scope) {")
	c.CompileStatements(statements)
	c.Emitln("}")
	return c.w.Bytes(), nil
}

type compiler struct {
	w         *bytes.Buffer
	indent    string
	pos       asp.Position
	functions map[string]*asp.FuncDef
	locals    map[string]local
}

type local struct {
	// The name that we refer to this variable as in generated code
	GenName string
	// The type of the variable ("int", "str", etc, or "object" for an untyped var)
	Type string
}

func (c *compiler) Error(msg string, args ...interface{}) {
	panic(fmt.Errorf("%s: %s", c.pos, fmt.Sprintf(msg, args...)))
}

func (c *compiler) Assert(condition bool, msg string, args ...interface{}) {
	if !condition {
		c.Error(msg, args...)
	}
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

func (c *compiler) Emitp(pre, post string, args ...interface{}) {
	w := &bytes.Buffer{}
	w.WriteString(pre)
	w.Write(c.w.Bytes())
	c.w = w
	c.Emitf(post, args...)
}

func (c *compiler) CompileStatements(stmts []*asp.Statement) {
	c.Indent()
	defer c.Unindent()
	for _, stmt := range stmts {
		c.pos = stmt.Pos
		if stmt.FuncDef != nil {
			c.compileFunc(stmt.FuncDef)
			c.setLocal(stmt.FuncDef.Name, stmt.FuncDef.Name, "func")
		} else if stmt.If != nil {
			c.compileIf(stmt.If)
		} else if stmt.For != nil {
			c.compileFor(stmt.For)
		} else if stmt.Return != nil {
			c.Emitfi("return ")
			c.compileExprs(stmt.Return.Values)
			c.Emitf("\n")
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
			c.Error("Expression has no effect")
		} else if stmt.Pass {
		} else if stmt.Continue {
			c.Emitln("continue")
		} else {
			c.Error("Unhandled statement")
		}
	}
}

func (c *compiler) compileFunc(def *asp.FuncDef) {
	// Here we generate a specialised function implementation that accepts concrete argument types.
	c.Emitln("")
	c.Emitfi("// %s_ is the specialised implementation of %s\n", def.Name, def.Name)
	c.Emitfi("%s_ := func(s_ *Scope", def.Name)
	for _, arg := range def.Arguments {
		if len(arg.Type) == 1 {
			c.Emitf(", %s %s", arg.Name, arg.Type[0]) // Special case where the type is certain.
		} else {
			c.Emitf(", %s Object", arg.Name)
		}
	}
	c.Emitf(") {\n")
	locals := c.overrideLocals(def.Arguments, true)
	c.CompileStatements(def.Statements)
	c.locals = locals
	c.Emitln("}")

	// This is the generic function that can be called from other asp code.
	c.Emitfi("// %s is the generic implementation that can be called from other asp code\n", def.Name)
	c.Emitfi(`s_.Set("%s", NewFunc("%s", s_,`, def.Name, def.Name)
	c.Emitf("\n")
	c.Indent()
	c.compileFunctionArgs(def.Arguments, def.Return)
	c.Emitfi("func (s *scope, args []PyObject) PyObject {\n")
	c.Indent()
	c.Emitfi("return %s_(s_,\n", def.Name)
	c.Indent()
	for i, arg := range def.Arguments {
		if len(arg.Type) == 1 {
			c.Emitfi("args[%d].(%s%s),\n", i, strings.ToUpper(arg.Type[0][:1]), arg.Type[0][1:])
		} else {
			c.Emitfi("args[%d],\n", i)
		}
	}
	c.Unindent()
	c.Emitfi(")\n")
	c.Unindent()
	c.Emitln("},")
	c.Unindent()
	c.Emitln("))")
}

func (c *compiler) compileFunctionArgs(args []asp.Argument, returnType string) {
	c.Emitfi("[]string{")
	for _, arg := range args {
		c.Emitf(`"%s",`, arg.Name)
	}
	c.Emitf("},\n")
	c.Emitfi("map[string]int{")
	for i, arg := range args {
		c.Emitf(`"%s": %d,`, arg.Name, i)
	}
	c.Emitf("},\n")
	c.Emitfi("[]PyObject{")
	for _, arg := range args {
		if arg.Value == nil {
			c.Emitf("nil, ")
		} else {
			c.compileExpr(arg.Value)
		}
	}
	c.Emitf("},\n")
	c.Emitfi("[][]string{")
	for _, arg := range args {
		c.Emitf("{")
		for _, t := range arg.Type {
			c.Emitf(`"%s",`, t)
		}
		c.Emitf("},")
	}
	c.Emitf("},\n")
	c.Emitfi(`"%s",`, returnType)
	c.Emitf("\n")
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
	locals := c.overrideLocalNames(f.Names)
	c.CompileStatements(f.Statements)
	c.locals = locals
	c.Emitln("}")
}

func (c *compiler) compileIdentStatement(ident *asp.IdentStatement) {
	if ident.Index != nil {
		c.Emitfi("%s[", ident.Name)
		c.compileExpr(ident.Index.Expr)
		c.Emitf("] = ")
		if ident.Index.Assign != nil {
			c.compileExpr(ident.Index.Assign)
			c.Emitf("\n")
		} else {
			c.Emitfi("%s[", ident.Name)
			c.compileExpr(ident.Index.Expr)
			c.Emitf("] + ")
			c.compileExpr(ident.Index.AugAssign)
			c.Emitf("\n")
		}
	} else if ident.Unpack != nil {
		c.Emitf("%s", ident.Name)
		for _, name := range ident.Unpack.Names {
			c.Emitf(", %s", name)
		}
		c.overrideLocalNames(append(ident.Unpack.Names, ident.Name))
		c.Emitf(" = ")
		c.compileExpr(ident.Unpack.Expr)
		c.Emitf("\n")
	} else if ident.Action != nil {
		if ident.Action.Property != nil {
			c.compileIdentExpr(ident.Action.Property)
		} else if ident.Action.Call != nil {
			c.compileCall(ident.Name, ident.Action.Call)
		} else if ident.Action.Assign != nil {
			c.Emitfi("%s = ", ident.Name)
			c.compileExpr(ident.Action.Assign)
		} else if ident.Action.AugAssign != nil {
			c.Emitfi("%s += ", ident.Name)
			c.compileExpr(ident.Action.AugAssign)
		}
		c.Emitf("\n")
	}
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
	c.pos = expr.Pos
	// At various points we may need to prepend to the buffer.
	w := c.w
	c.w = &bytes.Buffer{}
	defer func() {
		w.Write(c.w.Bytes())
		c.w = w
	}()

	if expr.UnaryOp != nil {
		if expr.UnaryOp.Op == "not" {
			c.Emitf("!") // TODO(peterebden): probably need something more advanced here...
		} else {
			c.Emitf("-")
		}
		c.compileValueExpr(&expr.UnaryOp.Expr)
	} else {
		c.compileValueExpr(expr.Val)
	}
	for _, op := range expr.Op {
		c.compileOp(op)
	}
}

func (c *compiler) compileOp(op asp.OpExpression) {
	switch op.Op {
	case asp.And:
		c.Emitp("(", " && ")
		defer c.Emitf(")")
	case asp.Or:
		c.Emitp("(", " || ")
		defer c.Emitf(")")
	case asp.Equal:
		c.Emitp("(", " == ")
		defer c.Emitf(")")
	case asp.NotEqual:
		c.Emitp("(", " != ")
		defer c.Emitf(")")
	case asp.In, asp.NotIn:
		c.Emitf(".Operator(%s, ", op.Op)
		c.compileExpr(op.Expr)
		c.Emitf(")")
	case asp.Add, asp.Subtract, asp.LessThan, asp.GreaterThan:
		c.Emitf(" %c ", op.Op)
		c.compileExpr(op.Expr)
	case asp.LessThanOrEqual:
		c.Emitf(" <= ")
		c.compileExpr(op.Expr)
	case asp.GreaterThanOrEqual:
		c.Emitf(" >= ")
		c.compileExpr(op.Expr)

	default:
		c.Error("Unimplemented operation %s", op.Op)
	}
	c.compileExpr(op.Expr)
}

func (c *compiler) compileIdentExpr(expr *asp.IdentExpr) {
	// Specialisation for the config object.
	// This implies users can't override it with a local var - that is also generally true in
	// the interpreter though.
	if expr.Name == "CONFIG" && len(expr.Action) == 1 && expr.Action[0].Property != nil {
		c.Emitf(`s_.ConfigStr("%s")`, expr.Action[0].Property.Name)
		return
	}
	c.compileVar(expr.Name)
	for _, action := range expr.Action {
		if action.Property != nil {
			c.Emitf(".Property(%s)", action.Property.Name)
		} else {
			c.compileCall(expr.Name, action.Call)
		}
	}
}

// compileVar compiles a variable lookup by name.
// This is less simple than one might think - it can get overridden in various ways.
func (c *compiler) compileVar(name string) {
	if local, present := c.locals[name]; present {
		c.Emitf("%s", local.GenName)
	} else {
		log.Warning("Unknown local variable '%s' at %s", name, c.pos)
		c.Emitf("%s", name)
	}
}

func (c *compiler) compileValueExpr(val *asp.ValueExpression) {
	if val.String != "" {
		c.Emitf("%s", val.String)
	} else if val.FString != nil {
		c.compileFString(val.FString)
	} else if val.Int != nil {
		c.Emitf("%d", val.Int.Int)
	} else if val.Bool != "" {
		c.Emitf("%s", val.Bool)
	} else if val.List != nil {
		c.compileList(val.List)
	} else if val.Tuple != nil {
		c.compileList(val.Tuple)
	} else if val.Dict != nil {
		c.compileDict(val.Dict)
	} else if val.Lambda != nil {
		c.compileLambda(val.Lambda)
	} else if val.Ident != nil {
		c.compileIdentExpr(val.Ident)
	} else if val.Slice != nil {
		c.compileSlice(val.Slice)
	} else if val.Property != nil {
		c.Emitf(`.Property("%s")`, val.Property.Name)
	} else if val.Call != nil {
		// TODO(peterebden): Obviously need to sort out <unknown>. We probably need to be able to
		//                   identify operations on builtins
		c.compileCall("<unknown>", val.Call)
	} else {
		c.Assert(false, "Unhandled expression")
	}
}

func (c *compiler) compileFString(f *asp.FString) {
	for _, v := range f.Vars {
		c.Emitf(`"%s" + `, v.Prefix)
		if v.Var != "" {
			c.compileVar(v.Var)
		} else {
			c.Emitf(`s_.ConfigStr("%s")`, v.Config)
		}
		c.Emitf(" + ")
	}
	c.Emitf(`"%s"`, f.Suffix)
}

func (c *compiler) compileList(l *asp.List) {
	c.Assert(l.Comprehension == nil, "Comprehensions not yet supported")
	c.Emitf("pyList{")
	for _, v := range l.Values {
		c.compileExpr(v)
		c.Emitf(", ")
	}
	c.Emitf("}")
}

func (c *compiler) compileDict(d *asp.Dict) {
	c.Assert(d.Comprehension == nil, "Comprehensions not yet supported")
	c.Emitf("pyDict{")
	for _, item := range d.Items {
		c.compileExpr(&item.Key)
		c.Emitf(": ")
		c.compileExpr(&item.Value)
		c.Emitf(", ")
	}
	c.Emitf("}")
}

func (c *compiler) compileLambda(l *asp.Lambda) {
	// N.B. Lambdas always compile to unspecialised functions. Usually they're passed as pre-
	//      or post-build functions in which case they have to have the generic calling convention
	//      anyways. In some cases (e.g. proto rules) they're not; we might think about that more
	//      at some point, but the indirection there probably defeats trying to do more
	//      specialisation anyway.
	//      OTOH this is probably not a super efficient process - pyFunc's aren't that lightweight
	//      and we are creating them pretty dynamically here.
	c.Emitf(`NewFunc("<lambda>", s_, \n`)
	c.compileFunctionArgs(l.Arguments, "")
	c.Emitfi("func (s_ *scope, args_ []PyObject) PyObject {\n")
	c.Indent()
	c.Emitf("return ")
	locals := c.overrideLocals(l.Arguments, false)
	c.compileExpr(&l.Expr)
	c.locals = locals
	c.Unindent()
	c.Emitf("}")
}

func (c *compiler) compileSlice(s *asp.Slice) {
	c.Emitf("[")
	if s.Start != nil {
		// Recall that Go slices match Python's in many ways, but do not support constructions like
		// [:-1] to index back from the end. We can't desugar that here because we might not have
		// a name for the object to get its length (although it might be possible eventually if
		// we wrapped it up in a little function - which may be a good idea since one could have
		// an arbitrary expression that resolved to a negative number.
		c.Assert(s.Start.Val.Int == nil || s.Start.Val.Int.Int >= 0, "Negative slice indices aren't supported")
		c.compileExpr(s.Start)
	}
	c.Emitf(":")
	if s.End != nil {
		c.Assert(s.End.Val.Int == nil || s.End.Val.Int.Int >= 0, "Negative slice indices aren't supported")
		c.compileExpr(s.End)
	}
	c.Emitf("]")
}

func (c *compiler) compileCall(name string, call *asp.Call) {
	f, present := c.functions[name]
	c.Assert(present, "Unknown function %s, cannot specialise", name)
	c.Emitf(".(*Func).Call(s_")
	// We need to call the arguments in definition order, but they don't have to be passed that way.
	args := map[string]asp.CallArgument{}
	for i, arg := range call.Arguments {
		if arg.Name != "" {
			args[arg.Name] = arg
		} else {
			args[f.Arguments[i].Name] = arg
		}
	}
	for _, arg := range f.Arguments {
		c.Emitf(",\n")
		c.Emitfi("")
		if callarg, present := args[arg.Name]; present {
			c.compileExpr(&callarg.Value)
		} else if arg.Value != nil {
			c.compileExpr(arg.Value)
		} else {
			c.Error("Missing required argument %s to %s", arg.Name, name)
		}
	}
	c.Emitf(")")
}

// overrideLocals maps a set of local variable names to argument indices.
func (c *compiler) overrideLocals(args []asp.Argument, named bool) map[string]local {
	locals := c.locals
	c.locals = make(map[string]local, len(locals)+len(args))
	for k, v := range locals {
		c.locals[k] = v
	}
	for i, arg := range args {
		if !named {
			// Unnamed variables can't be typed (because they're coming from a []pyObject)
			c.setLocal(arg.Name, fmt.Sprintf("args_[%d]", i), "object")
		} else if len(arg.Type) == 1 {
			c.setLocal(arg.Name, arg.Name, arg.Type[0])
		} else {
			c.setLocal(arg.Name, arg.Name, "object")
		}
	}
	return locals
}

// overrideLocalNames maps a set of local names to argument indices.
// TODO(peterebden): General TODO here about adding typing information.
func (c *compiler) overrideLocalNames(names []string) map[string]local {
	locals := c.locals
	c.locals = make(map[string]local, len(locals)+len(names))
	for k, v := range locals {
		c.locals[k] = v
	}
	for _, name := range names {
		c.setLocal(name, name, "object")
	}
	return locals
}

// setLocal sets a single local variable.
func (c *compiler) setLocal(name, genName, typ string) {
	c.locals[name] = local{GenName: genName, Type: typ}
}
