package asp

import (
	"bytes"
	"fmt"
	"math"
)

// An instruction is an instruction code that tells us the kind of thing to do next.
type instruction uint8

const (
	insLoadNamed instruction = iota
	insLoadLocal
	insAssignLocal
	insPropertyLookup
)

// compile takes a series of statements and compiles them into bytecode.
func compile(stmts []*Statement) []byte {
	c := compiler{locals: map[string]uint8{}}
	c.compileStatements(stmts)
	return c.code.Bytes()
}

// compileFunc compiles a function definition into bytecode.
func compileFunc(def *FuncDef) []byte {
	c := compiler{locals: make(map[string]uint8, len(def.Arguments))}
	for _, arg := range def.Arguments {
		c.AddLocal(arg.Name)
	}
	c.compileStatements(def.Statements)
	return c.code.Bytes()
}

type compiler struct {
	locals map[string]uint8 // map of local name -> stack index
	code   bytes.Buffer
}

func (c *compiler) CompileStatements(stmts []*Statement) {
	for _, stmt := range stmts {
		if id := stmt.Ident; id != nil {
			buf.WriteByte(insLoadLocal)
			if ac := id.Action; ac != nil {

			}
		} else {
			panic(fmt.Errorf("unsupported statement %s", stmt))
		}
	}
}

// AddLocal adds a new local to the bytecode, returning its index.
func (c *compiler) AddLocal(name string) uint8 {
	l := len(c.locals)
	if l == math.MaxUint8 {
		// 255 local variables will be enough for anyone, right?
		panic("Too many local variables in function")
	}
	c.locals[name] = uint8(l)
	return uint8(l)
}

// load emits a
func (c *compiler) loadLocal(name string) {
	c.code.WriteByte(insLoadLocal)
	c.code.WriteByte(
}
