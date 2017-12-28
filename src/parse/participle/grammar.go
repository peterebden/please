package participle

// A fileInput is the top-level structure of a BUILD file.
type fileInput struct {
	Statements []*statement `{ @@ }`
	EOF        string       `@EOF`
}

// A statement is the type we work with externally the most; it's a single Python statement.
type statement struct {
	FuncDef    *funcDef    `  @@`
	Pass       string      `| @"pass"`
	Expression *expression `| @@`
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

type expression struct {
	String string `  @String`
	Int    int    `| @Int`
	Ident  *ident `| @@`
}

type ident struct {
	Name   string `@Ident`
	Action struct {
		Property *ident `  "." @@`
		Call     *call  `| @@`
	} `[ @@ ]`
}

type call struct {
	Arguments []*argument `"(" { @@ [ "," ] } ")"`
}
