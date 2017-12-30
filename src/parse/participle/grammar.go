package participle

// A fileInput is the top-level structure of a BUILD file.
type fileInput struct {
	Statements []*statement `{ @@ }`
	EOF        string       `EOF`
}

// A statement is the type we work with externally the most; it's a single Python statement.
// Note that some mildly excessive fiddling is needed since the parser we're using doesn't
// support backoff (i.e. if an earlier entry matches to its completion but can't consume
// following tokens, it doesn't then make another choice :( )
type statement struct {
	Pass    string          `( @"pass" EOL`
	FuncDef *funcDef        `| @@`
	For     *forStatement   `| @@`
	If      *ifStatement    `| @@`
	Return  *expression     `| "return" @@ EOL`
	Raise   *expression     `| "raise" @@ EOL`
	Literal *literal        `| @@ EOL`
	Ident   *identStatement `| @@ EOL)`
}

type funcDef struct {
	Name       string       `"def" @Ident`
	Arguments  []*argument  `"(" [ @@ { "," @@ } ] ")" Colon EOL`
	Statements []*statement `{ @@ } Unindent`
}

type forStatement struct {
	Names      []string     `"for" @Ident [ { "," @Ident } ] "in"`
	Expr       expression   `@@ Colon EOL`
	Statements []*statement `{ @@ } Unindent`
}

type ifStatement struct {
	Condition  expression   `"if" @@ Colon EOL`
	Statements []*statement `{ @@ } Unindent`
	Elif       []struct {
		Condition  *expression  `"elif" @@ Colon EOL`
		Statements []*statement `{ @@ } Unindent`
	} `{ @@ }`
	ElseStatements []*statement `[ "else" Colon EOL { @@ } Unindent ]`
}

type argument struct {
	Name  string     `@Ident`
	Value expression `[ "=" @@ ]`
}

type literal struct {
	UnaryOp  *unaryOp  `( @@`
	String   string    `| @String`
	Int      int       `| @Int`
	List     *list     `| "[" @@ "]"`
	Dict     *dict     `| "{" @@ "}"`
	Tuple    *list     `| "(" @@ ")" )` // Tuples don't have a separate implementation.
	Lambda   *lambda   `| "lambda" @@`
	Op       *operator `[ @@ ]`
	Slice    *slice    `[ @@ ]`
	If       *inlineIf `[ @@ ]`
	Property *ident    `[ ( "." @@`
	Call     *call     `| "(" @@ ")" ) ]`
}

type expression struct {
	UnaryOp  *unaryOp  `( @@`
	String   string    `| @String`
	Int      int       `| @Int`
	List     *list     `| "[" @@ "]"`
	Dict     *dict     `| "{" @@ "}"`
	Tuple    *list     `| "(" @@ ")"`
	Lambda   *lambda   `| "lambda" @@`
	Ident    *ident    `| @@ )`
	Op       *operator `[ @@ ]`
	Slice    *slice    `[ @@ ]`
	If       *inlineIf `[ @@ ]`
	Property *ident    `[ ( "." @@`
	Call     *call     `| "(" @@ ")" ) ]`
}

type unaryOp struct {
	Op   string     `@( "-" | "not" )`
	Expr expression `@@`
}

type identStatement struct {
	Name   string `@Ident`
	Action struct {
		Property    *ident          `  "." @@`
		Call        *call           `| "(" @@ ")"`
		Assign      *expression     `| "=" @@`
		AugAssign   *expression     `| "+=" @@`
		Destructure *identStatement `| "," @@`
	} `[ @@ ]`
}

type ident struct {
	Name   string `@Ident`
	Action struct {
		Property *ident      `  "." @@`
		Call     *call       `| "(" @@ ")"`
		Assign   *expression `| "=" @@`
	} `[ @@ ]`
}

type call struct {
	Arguments []*expression `[ @@ ] { "," [ @@ ] }`
}

type list struct {
	Values        []*expression  `[ @@ ] { "," [ @@ ] }`
	Comprehension *comprehension `[ @@ ]`
}

type dict struct {
	Items         []*dictItem    `[ @@ ] { "," [ @@ ] }`
	Comprehension *comprehension `[ @@ ]`
}

type dictItem struct {
	Key   string     `@( Ident | String ) ":"`
	Value expression `@@`
}

type operator struct {
	Op   string      `@("+" | "%" | "<" | ">" | "and" | "or" | "is" | "in" | "not" "in" | "not" | "==" | "!=")`
	Expr *expression `@@`
}

type slice struct {
	// Implements indexing as well as slicing.
	Start *expression `"[" [ @@ ]`
	Colon string      `[ @":" ]`
	End   *expression `[ @@ ] "]"`
}

type inlineIf struct {
	Condition *expression `"if" @@`
	Else      *expression `[ "else" @@ ]`
}

type comprehension struct {
	Names []string    `"for" @Ident [ { "," @Ident } ] "in"`
	Expr  *expression `@@`
	If    *expression `[ "if" @@ ]`
}

type lambda struct {
	Arguments []*argument `[ @@ { "," @@ } ] Colon`
	Expr      expression  `@@`
}
