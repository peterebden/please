package participle

// A fileInput is the top-level structure of a BUILD file.
type fileInput struct {
	Statements []*statement `{ @@ }`
	EOF        string       `@EOF`
}

// A statement is the type we work with externally the most; it's a single Python statement.
// Note that some mildly excessive fiddling is needed since the parser we're using doesn't
// support backoff (i.e. if an earlier entry matches to its completion but can't consume
// following tokens, it doesn't then make another choice :( )
type statement struct {
	FuncDef *funcDef `  @@`
	Pass    string   `| @"pass"`
	Literal *literal `| @@`
	Ident   *ident   `| @@`
}

type funcDef struct {
	Name       string       `"def" @Ident`
	Arguments  []*argument  `"(" [ @@ { "," @@ } ] ")"`
	Colon      string       `@Colon`
	Statements []*statement `@@`
	End        string       `@Unindent`
}

type argument struct {
	Name  string     `@Ident`
	Value expression `[ "=" @@ ]`
}

type literal struct {
	String string `  @String`
	Int    int    `| @Int`
	List   *list  `| "[" @@ "]"`
	Dict   *dict  `| "{" @@ "}"`
}

type expression struct {
	String string `  @String`
	Int    int    `| @Int`
	List   *list  `| "[" @@ "]"`
	Dict   *dict  `| "{" @@ "}"`
	Ident  *ident `| @@`
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
	Arguments      []*literal  `{ @@ [ "," ] }`
	NamedArguments []*argument `{ @@ [ "," ] }`
}

type list struct {
	Values []*literal `{ @@ [ "," ] }`
}

type dict struct {
	Items []*dictItem `{ @@ [ "," ] }`
}

type dictItem struct {
	Key   string     `@String ":"`
	Value expression `@@`
}
