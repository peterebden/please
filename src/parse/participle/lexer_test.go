package participle

import (
	"strings"
	"testing"

	"github.com/alecthomas/participle/lexer"
	"github.com/stretchr/testify/assert"
)

func TestSymbols(t *testing.T) {
	s := NewLexer().Symbols()
	assert.Equal(t, lexer.EOF, s["EOF"]) // Seems this should be consistent
}

func assertToken(t *testing.T, tok lexer.Token, tokenType rune, value string, line, column, offset int) {
	assert.EqualValues(t, tokenType, tok.Type)
	assert.Equal(t, value, tok.Value)
	assert.Equal(t, line, tok.Pos.Line)
	assert.Equal(t, column, tok.Pos.Column)
	assert.Equal(t, offset, tok.Pos.Offset)
}

func TestLexBasic(t *testing.T) {
	l := NewLexer().Lex(strings.NewReader("hello world"))
	assertToken(t, l.Next(), Ident, "hello", 1, 1, 1)
	assertToken(t, l.Peek(), Ident, "world", 1, 7, 7)
	assertToken(t, l.Next(), Ident, "world", 1, 7, 7)
	assertToken(t, l.Next(), EOF, "", 1, 12, 12)
}

func TestLexMultiline(t *testing.T) {
	l := NewLexer().Lex(strings.NewReader("hello\nworld\n"))
	assertToken(t, l.Next(), Ident, "hello", 1, 1, 1)
	assertToken(t, l.Next(), EOL, "", 1, 6, 6)
	assertToken(t, l.Next(), Ident, "world", 2, 1, 7)
	assertToken(t, l.Peek(), EOL, "", 2, 6, 12)
	assertToken(t, l.Next(), EOL, "", 2, 6, 12)
	assertToken(t, l.Next(), EOF, "", 3, 1, 13)
}
