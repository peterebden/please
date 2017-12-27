// Package participle implements an experimental BUILD-language parser using Participle
// (github.com/alecthomas/participle) in native Go.
package participle

import (
	"os"

	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"gopkg.in/op/go-logging.v1"

	"core"
)

var log = logging.MustGetLogger("participle")

type parser struct {
	parser *participle.Parser
}

// NewParser creates a new parser instance. One is normally sufficient for a process lifetime.
func NewParser() *parser {
	lexer, err := lexer.EBNF(ebnf)
	if err != nil {
		log.Fatalf("%s", err)
	}
	p, err := participle.Build(&fileInput{})
	if err != nil {
		log.Fatalf("%s", err)
	}
	return parser{parser: p}
}

// Parse parses the contents of a single file in the BUILD language.
func (p *parser) ParseFile(state *core.BuildState, pkg *core.Package, filename string) error {
	statements, err := p.parse(filename)
	if err != nil {
		return err
	}
	return p.interpret(state, pkg, statements)
}

// parse reads the given file and parses it into a set of statements.
func (p *parser) parse(filename string) ([]*statement, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	input := &fileInput{}
	if err := p.parser.Parse(f, input); err != nil {
		return nil, err
	}
	return input.Statements, nil
}

// interpret runs a series of statements in the context of the given package.
func (p *parser) interpret(state *core.BuildState, pkg *core.Package) error {
	return nil
}
