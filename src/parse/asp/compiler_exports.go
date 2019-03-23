package asp

// These are exported for use of the AOT compiler.
// TODO(peterebden): find a better scheme that doesn't expose so much.
type PyObject = pyObject
type PyBool = pyBool
type PyInt = pyInt
type PyString = pyString
type PyList = pyList
type PyDict = pyDict
type PyFunc = pyFunc
type PyConfig = pyConfig
type Scope = scope

// A NativeFunc is the signature of a function implemented in native code.
type NativeFunc func(*Scope, []PyObject) PyObject

// NewFunc creates a new Func instance
func NewFunc(name string, scope *scope, args []string, argIndices map[string]int, defaults []PyObject, types [][]string, returnType string, nativeCode NativeFunc) *PyFunc {
	return &pyFunc{
		name:       name,
		scope:      scope,
		args:       args,
		argIndices: argIndices,
		constants:  defaults,
		types:      types,
		nativeCode: nativeCode,
		returnType: returnType,
	}
}
