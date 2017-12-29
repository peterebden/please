package participle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBasic(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/basic.build")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(statements))
	assert.NotNil(t, statements[0].FuncDef)
	assert.Equal(t, "test", statements[0].FuncDef.Name)
	assert.Equal(t, 1, len(statements[0].FuncDef.Arguments))
	assert.Equal(t, "x", statements[0].FuncDef.Arguments[0].Name)
	assert.Equal(t, 1, len(statements[0].FuncDef.Statements))
	assert.Equal(t, "pass", statements[0].FuncDef.Statements[0].Pass)
}

func TestParseDefaultArguments(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/default_arguments.build")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(statements))
	assert.NotNil(t, statements[0].FuncDef)
	assert.Equal(t, "test", statements[0].FuncDef.Name)
	assert.Equal(t, 3, len(statements[0].FuncDef.Arguments))
	assert.Equal(t, 1, len(statements[0].FuncDef.Statements))
	assert.Equal(t, "pass", statements[0].FuncDef.Statements[0].Pass)

	args := statements[0].FuncDef.Arguments
	assert.Equal(t, "name", args[0].Name)
	assert.Equal(t, "name", args[0].Value.String)
	assert.Equal(t, "timeout", args[1].Name)
	assert.Equal(t, 10, args[1].Value.Int)
	assert.Equal(t, "args", args[2].Name)
	assert.Equal(t, "CONFIG", args[2].Value.Ident.Name)
	assert.Equal(t, "ARGS", args[2].Value.Ident.Action.Property.Name)
}

func TestParseFunctionCalls(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/function_call.build")
	assert.NoError(t, err)
	assert.Equal(t, 5, len(statements))

	assert.NotNil(t, statements[0].Ident.Action.Call)
	assert.Equal(t, "package", statements[0].Ident.Name)
	assert.Equal(t, 0, len(statements[0].Ident.Action.Call.NamedArguments))

	assert.NotNil(t, statements[2].Ident.Action.Call)
	assert.Equal(t, "package", statements[2].Ident.Name)
	assert.Equal(t, 1, len(statements[2].Ident.Action.Call.NamedArguments))
	arg := statements[2].Ident.Action.Call.NamedArguments[0]
	assert.Equal(t, "default_visibility", arg.Name)
	assert.NotNil(t, arg.Value.List)
	assert.Equal(t, 1, len(arg.Value.List.Values))
	assert.Equal(t, "PUBLIC", arg.Value.List.Values[0].String)

	assert.NotNil(t, statements[3].Ident.Action.Call)
	assert.Equal(t, "python_library", statements[3].Ident.Name)
	assert.Equal(t, 2, len(statements[3].Ident.Action.Call.NamedArguments))
	args := statements[3].Ident.Action.Call.NamedArguments
	assert.Equal(t, "name", args[0].Name)
	assert.Equal(t, "lib", args[0].Value.String)
	assert.Equal(t, "srcs", args[1].Name)
	assert.NotNil(t, args[1].Value.List)
	assert.Equal(t, 2, len(args[1].Value.List.Values))
	assert.Equal(t, "lib1.py", args[1].Value.List.Values[0].String)
	assert.Equal(t, "lib2.py", args[1].Value.List.Values[1].String)

	assert.NotNil(t, statements[4].Ident.Action.Call)
	assert.Equal(t, "subinclude", statements[4].Ident.Name)
	assert.NotNil(t, statements[4].Ident.Action.Call)
	assert.Equal(t, 1, len(statements[4].Ident.Action.Call.Arguments))
	assert.Equal(t, 0, len(statements[4].Ident.Action.Call.NamedArguments))
	assert.Equal(t, "//build_defs:version", statements[4].Ident.Action.Call.Arguments[0].String)
}

func TestParseAssignments(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/assignments.build")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(statements))

	assert.NotNil(t, statements[0].Ident.Action.Assign)
	assert.Equal(t, "x", statements[0].Ident.Name)
	ass := statements[0].Ident.Action.Assign.Dict
	assert.NotNil(t, ass)
	assert.Equal(t, 3, len(ass.Items))
	assert.Equal(t, "mickey", ass.Items[0].Key)
	assert.Equal(t, 3, ass.Items[0].Value.Int)
	assert.Equal(t, "donald", ass.Items[1].Key)
	assert.Equal(t, "sora", ass.Items[1].Value.String)
	assert.Equal(t, "goofy", ass.Items[2].Key)
	assert.Equal(t, "riku", ass.Items[2].Value.Ident.Name)
}

func TestForStatement(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/for_statement.build")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(statements))

	assert.NotNil(t, statements[0].Ident.Action.Assign)
	assert.Equal(t, "LANGUAGES", statements[0].Ident.Name)
	assert.Equal(t, 2, len(statements[0].Ident.Action.Assign.List.Values))

	assert.NotNil(t, statements[1].For)
	assert.Equal(t, []string{"language"}, statements[1].For.Names)
	assert.Equal(t, "LANGUAGES", statements[1].For.Expr.Ident.Name)
	assert.Equal(t, 1, len(statements[1].For.Statements))
}

func TestOperators(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/operators.build")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(statements))

	assert.NotNil(t, statements[0].Ident.Action.Call)
	assert.Equal(t, "genrule", statements[0].Ident.Name)
	assert.Equal(t, 2, len(statements[0].Ident.Action.Call.NamedArguments))

	arg := statements[0].Ident.Action.Call.NamedArguments[1]
	assert.Equal(t, "srcs", arg.Name)
	assert.NotNil(t, arg.Value.List)
	assert.Equal(t, 1, len(arg.Value.List.Values))
	assert.Equal(t, "//something:test_go", arg.Value.List.Values[0].String)
	assert.NotNil(t, arg.Value.Op)
	assert.Equal(t, "+", arg.Value.Op.Op)
	call := arg.Value.Op.Expr.Ident.Action.Call
	assert.Equal(t, "glob", arg.Value.Op.Expr.Ident.Name)
	assert.NotNil(t, call)
	assert.Equal(t, 1, len(call.Arguments))
	assert.NotNil(t, call.Arguments[0].List)
	assert.Equal(t, 1, len(call.Arguments[0].List.Values))
	assert.Equal(t, "*.go", call.Arguments[0].List.Values[0].String)
}

func TestIndexing(t *testing.T) {
	statements, err := NewParser().parse("src/parse/participle/test_data/indexing.build")
	assert.NoError(t, err)
	assert.Equal(t, 5, len(statements))

	assert.Equal(t, "x", statements[0].Ident.Name)
	assert.NotNil(t, statements[0].Ident.Action.Assign)
	assert.Equal(t, "test", statements[0].Ident.Action.Assign.String)

	assert.Equal(t, "y", statements[1].Ident.Name)
	assert.NotNil(t, statements[1].Ident.Action.Assign)
	assert.Equal(t, "x", statements[1].Ident.Action.Assign.Ident.Name)
	assert.NotNil(t, statements[1].Ident.Action.Assign.Slice)
	assert.Equal(t, 2, statements[1].Ident.Action.Assign.Slice.Start)
	assert.Equal(t, "", statements[1].Ident.Action.Assign.Slice.Colon)
	assert.Equal(t, 0, statements[1].Ident.Action.Assign.Slice.End)

	assert.Equal(t, "z", statements[2].Ident.Name)
	assert.NotNil(t, statements[2].Ident.Action.Assign)
	assert.Equal(t, "x", statements[2].Ident.Action.Assign.Ident.Name)
	assert.NotNil(t, statements[2].Ident.Action.Assign.Slice)
	assert.Equal(t, 1, statements[2].Ident.Action.Assign.Slice.Start)
	assert.Equal(t, ":", statements[2].Ident.Action.Assign.Slice.Colon)
	assert.Equal(t, -1, statements[2].Ident.Action.Assign.Slice.End)

	assert.Equal(t, "a", statements[3].Ident.Name)
	assert.NotNil(t, statements[3].Ident.Action.Assign)
	assert.Equal(t, "x", statements[3].Ident.Action.Assign.Ident.Name)
	assert.NotNil(t, statements[3].Ident.Action.Assign.Slice)
	assert.Equal(t, 2, statements[3].Ident.Action.Assign.Slice.Start)
	assert.Equal(t, ":", statements[3].Ident.Action.Assign.Slice.Colon)
	assert.Equal(t, 0, statements[3].Ident.Action.Assign.Slice.End)

	assert.Equal(t, "b", statements[4].Ident.Name)
	assert.NotNil(t, statements[4].Ident.Action.Assign)
	assert.Equal(t, "x", statements[4].Ident.Action.Assign.Ident.Name)
	assert.NotNil(t, statements[4].Ident.Action.Assign.Slice)
	assert.Equal(t, 0, statements[4].Ident.Action.Assign.Slice.Start)
	assert.Equal(t, ":", statements[4].Ident.Action.Assign.Slice.Colon)
	assert.Equal(t, 2, statements[4].Ident.Action.Assign.Slice.End)
}
