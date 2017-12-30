package participle

import (
	"io"
	"io/ioutil"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/participle/lexer"
)

// Token types.
const (
	EOF = -(iota + 1)
	Ident
	Int
	String
	EOL
	Unindent
	Colon    // Would prefer not to have this but literal colons seem to deeply upset the parser.
	Operator // Similarly this, it doesn't seem to be able to handle == otherwise.
)

// indentation is the number of spaces we recognise for indentation - right now it's four spaces only.
const indentation = 4

// symbols defines our mapping of lexer symbols.
var symbols = map[string]rune{
	"EOF":      EOF,
	"Ident":    Ident,
	"Int":      Int,
	"String":   String,
	"EOL":      EOL,
	"Unindent": Unindent,
	"Colon":    Colon,
}

// A definition is an implementation of participle's lexer.Definition,
// which is a singleton that defines how to create individual lexer instances.
type definition struct{}

// NewLexer is a slightly nicer interface to creating a new lexer definition.
func NewLexer() lexer.Definition {
	return &definition{}
}

// Lex implements the lexer.Definition interface.
func (d *definition) Lex(r io.Reader) lexer.Lexer {
	// Read the entire file upfront to avoid bufio etc.
	// This should work OK as long as BUILD files are relatively small.
	b, err := ioutil.ReadAll(r)
	if err != nil {
		lexer.Panic(lexer.Position{Filename: lexer.NameOfReader(r)}, err.Error())
	}
	l := &lex{
		b:        append(b, 0, 0), // Null-terminating the buffer makes things easier later.
		filename: lexer.NameOfReader(r),
		indents:  []int{0},
	}
	l.Next() // Initial value is zero, this forces it to populate itself.
	// Discard any leading newlines, they are just an annoyance.
	for l.Peek().Type == EOL {
		l.Next()
	}
	return l
}

// Symbols implements the lexer.Definition interface.
func (d *definition) Symbols() map[string]rune {
	return symbols
}

// A lex implements participle's lexer.Lexer, which is a lexer for a single BUILD file.
type lex struct {
	b      []byte
	i      int
	line   int
	col    int
	indent int
	// The next token. We always look one token ahead in order to facilitate both Peek() and Next().
	next     lexer.Token
	filename string
	// Used to track how many braces we're within.
	braces int
	// Pending unindent tokens. This is a bit yuck but means the parser doesn't need to
	// concern itself about indentation (which is good because our parser doesn't do that...)
	unindents int
	// Current levels of indentation
	indents []int
	// Remember whether the last token we output was an end-of-line so we don't emit multiple in sequence.
	lastEOL bool
}

// Peek at the next token
func (l *lex) Peek() lexer.Token {
	return l.next
}

// Next consumes and returns the next token.
func (l *lex) Next() lexer.Token {
	ret := l.next
	l.next = l.nextToken()
	l.lastEOL = l.next.Type == EOL || l.next.Type == Unindent
	return ret
}

// nextToken consumes and returns the next token.
func (l *lex) nextToken() lexer.Token {
	// Strip any spaces
	for l.b[l.i] == ' ' {
		l.i++
		l.col++
	}
	pos := lexer.Position{
		Filename: l.filename,
		// These are all 1-indexed for niceness.
		Offset: l.i + 1,
		Line:   l.line + 1,
		Column: l.col + 1,
	}
	if l.unindents > 0 {
		l.unindents--
		return lexer.Token{Type: Unindent, Pos: pos}
	}
	b := l.b[l.i]
	rawString := b == 'r' && (l.b[l.i+1] == '"' || l.b[l.i+1] == '\'')
	if rawString {
		l.i++
		l.col++
		b = l.b[l.i]
	} else if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b >= utf8.RuneSelf {
		return l.consumeIdent(pos)
	}
	l.i++
	l.col++
	switch b {
	case 0:
		// End of file (we null terminate it above so this is easy to spot)
		return lexer.Token{Type: EOF, Pos: pos}
	case '\n':
		// End of line, read indent to next non-space character
		lastIndent := l.indent
		l.line++
		l.col = 0
		indent := 0
		for l.b[l.i] == ' ' {
			l.i++
			l.col++
			indent++
		}
		if l.b[l.i] == '\n' {
			return l.nextToken()
		}
		if l.braces == 0 {
			l.indent = indent
		}
		if lastIndent > l.indent && l.braces == 0 {
			pos.Line++ // Works better if it's at the new position
			pos.Column = l.col + 1
			for l.indents[len(l.indents)-1] > l.indent {
				l.unindents++
				l.indents = l.indents[:len(l.indents)-1]
			}
			if l.indent != l.indents[len(l.indents)-1] {
				lexer.Panic(pos, "Unexpected indent")
			}
		} else if lastIndent != l.indent {
			l.indents = append(l.indents, l.indent)
		}
		if l.braces == 0 && !l.lastEOL {
			return lexer.Token{Type: EOL, Pos: pos}
		}
		return l.nextToken()
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.consumeInteger(b, pos)
	case '"', '\'':
		// String literal, consume to end.
		if l.b[l.i] == b && l.b[l.i+1] == b {
			l.i += 2 // Jump over initial quote
			l.col += 2
			return l.consumeString(b, pos, true, rawString)
		}
		return l.consumeString(b, pos, false, rawString)
	case ':':
		// As noted above, literal colons seem to break the parser.
		return lexer.Token{Type: Colon, Value: ":", Pos: pos}
	case '(', '[', '{':
		l.braces++
		return lexer.Token{Type: rune(b), Value: string(b), Pos: pos}
	case ')', ']', '}':
		if l.braces > 0 { // Don't let it go negative, it fouls things up
			l.braces--
		}
		return lexer.Token{Type: rune(b), Value: string(b), Pos: pos}
	case '=', '!', '+', '<', '>':
		// Look ahead one byte to see if this is an augmented assignment or comparison.
		if l.b[l.i] == '=' {
			l.i++
			l.col++
			return lexer.Token{Type: Operator, Value: string([]byte{b, l.b[l.i-1]}), Pos: pos}
		}
		fallthrough
	case ',', '.', '%':
		return lexer.Token{Type: rune(b), Value: string(b), Pos: pos}
	case '#':
		// Comment character, consume to end of line.
		for l.b[l.i] != '\n' {
			l.i++
			l.col++
		}
		return l.nextToken() // Comments aren't tokens themselves.
	case '-':
		// We lex unary - with the integer if possible.
		if l.b[l.i] >= '0' && l.b[l.i] <= '9' {
			return l.consumeInteger(b, pos)
		}
		return lexer.Token{Type: rune(b), Value: string(b), Pos: pos}
	default:
		lexer.Panicf(pos, "Unknown character %c", b)
	}
	panic("unreachable")
}

// consumeInteger consumes all characters until the end of an integer literal is reached.
func (l *lex) consumeInteger(initial byte, pos lexer.Position) lexer.Token {
	s := make([]byte, 1, 10)
	s[0] = initial
	for c := l.b[l.i]; c >= '0' && c <= '9'; c = l.b[l.i] {
		l.i++
		l.col++
		s = append(s, c)
	}
	return lexer.Token{Type: Int, Value: string(s), Pos: pos}
}

// consumeString consumes all characters until the end of a string literal is reached.
func (l *lex) consumeString(quote byte, pos lexer.Position, multiline, raw bool) lexer.Token {
	s := make([]byte, 1, 100) // 100 chars is typically enough for a single string literal.
	s[0] = '"'
	escaped := false
	for {
		c := l.b[l.i]
		l.i++
		l.col++
		if escaped {
			if c == 'n' {
				s = append(s, '\n')
			} else {
				s = append(s, c)
			}
			escaped = false
			continue
		}
		switch c {
		case quote:
			if !multiline || (l.b[l.i] == quote && l.b[l.i+1] == quote) {
				s = append(s, '"')
				if multiline {
					l.i += 2
					l.col += 2
				}
				return lexer.Token{Type: String, Value: string(s), Pos: pos}
			}
		case '\n':
			if multiline {
				l.line++
				l.col = 0
				s = append(s, c)
				continue
			}
			fallthrough
		case 0:
			lexer.Panic(pos, "Unterminated string literal")
		case '\\':
			if !raw {
				escaped = true
				continue
			}
			fallthrough
		default:
			s = append(s, c)
		}
	}
}

// consumeIdent consumes all characters of an identifier.
func (l *lex) consumeIdent(pos lexer.Position) lexer.Token {
	s := make([]rune, 0, 100)
	for {
		c := rune(l.b[l.i])
		if c >= utf8.RuneSelf {
			// Multi-byte encoded in utf-8.
			r, n := utf8.DecodeRune(l.b[l.i:])
			c = r
			l.i += n
			l.col += n
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				lexer.Panicf(pos, "Illegal Unicode identifier %c", c)
			}
			s = append(s, c)
			continue
		}
		l.i++
		l.col++
		switch c {
		case ' ':
			// End of identifier, but no unconsuming needed.
			return lexer.Token{Type: Ident, Value: string(s), Pos: pos}
		case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			s = append(s, c)
		default:
			// End of identifier. Unconsume the last character so it gets handled next time.
			l.i--
			l.col--
			return lexer.Token{Type: Ident, Value: string(s), Pos: pos}
		}
	}
}
