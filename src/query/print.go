package query

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"core"
)

// QueryPrint produces a Python call which would (hopefully) regenerate the same build rule if run.
// This is of course not ideal since they were almost certainly created as a java_library
// or some similar wrapper rule, but we've lost that information by now.
func QueryPrint(graph *core.BuildGraph, labels []core.BuildLabel) {
	for _, label := range labels {
		fmt.Fprintf(os.Stderr, "%s:\n", label)
		newPrinter(os.Stdout, graph.TargetOrDie(label), 2).PrintTarget()
	}
}

// specialFields is a mapping of field name -> any special casing relating to how to print it.
var specialFields = map[string]func(*printer) (string, bool){
	"name": func(p *printer) (string, bool) {
		return "'" + p.target.Label.Name + "'", true
	},
	"building_description": func(p *printer) (string, bool) {
		return p.target.BuildingDescription, p.target.BuildingDescription != core.DefaultBuildingDescription
	},
	"deps": func(p *printer) (string, bool) {
		return p.genericPrint(reflect.ValueOf(p.target.DeclaredDependenciesStrict()))
	},
	"visibility": func(p *printer) (string, bool) {
		if len(p.target.Visibility) == 1 && p.target.Visibility[0] == core.WholeGraph[0] {
			return "['PUBLIC']", true
		}
		return p.genericPrint(reflect.ValueOf(p.target.Visibility))
	},
	"container": func(p *printer) (string, bool) {
		if p.target.ContainerSettings == nil {
			return "True", p.target.Containerise
		}
		return p.genericPrint(reflect.ValueOf(p.target.ContainerSettings.ToMap()))
	},
}

// fieldPrecedence defines a specific ordering for fields.
var fieldPrecedence = map[string]int{
	"name":       -100,
	"srcs":       -90,
	"visibility": 90,
	"deps":       100,
}

// A printer is responsible for creating the output of 'plz query print'.
type printer struct {
	w          io.Writer
	target     *core.BuildTarget
	indent     int
	doneFields map[string]bool
}

// newPrinter creates a new printer instance.
func newPrinter(w io.Writer, target *core.BuildTarget, indent int) *printer {
	return &printer{
		w:          w,
		target:     target,
		indent:     indent,
		doneFields: make(map[string]bool, 50), // Leave enough space for all of BuildTarget's fields.
	}
}

// printf is an internal function which prints to the internal writer with an indent.
func (p *printer) printf(msg string, args ...interface{}) {
	fmt.Fprint(p.w, strings.Repeat(" ", p.indent))
	fmt.Fprintf(p.w, msg, args...)
}

// PrintTarget prints an entire build target.
func (p *printer) PrintTarget() {
	if p.target.IsFilegroup {
		p.printf("filegroup(\n")
	} else {
		p.printf("build_rule(\n")
	}
	p.indent += 4
	v := reflect.ValueOf(p.target).Elem()
	t := v.Type()
	f := make(orderedFields, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f[i].structIndex = i
		f[i].printIndex = i
		if index, present := fieldPrecedence[p.fieldName(t.Field(i))]; present {
			f[i].printIndex = index
		}
	}
	sort.Sort(f)
	for _, orderedField := range f {
		p.printField(t.Field(orderedField.structIndex), v.Field(orderedField.structIndex))
	}
	p.indent -= 4
	p.printf(")\n\n")
}

// fieldName returns the name we'll use to print a field.
func (p *printer) fieldName(f reflect.StructField) string {
	if name := f.Tag.Get("name"); name != "" {
		return name
	}
	// We don't bother specifying on some fields when it's equivalent other than case.
	return strings.ToLower(f.Name)
}

// printField prints a single field of a build target.
func (p *printer) printField(f reflect.StructField, v reflect.Value) {
	if f.Tag.Get("print") == "false" { // Indicates not to print the field.
		return
	}
	name := p.fieldName(f)
	if p.doneFields[name] {
		return
	}
	if customFunc, present := specialFields[name]; present {
		if contents, shouldPrint := customFunc(p); shouldPrint {
			p.printf("%s = %s,\n", name, contents)
			p.doneFields[name] = true
		}
	} else if contents, shouldPrint := p.genericPrint(v); shouldPrint {
		p.printf("%s = %s,\n", name, contents)
		p.doneFields[name] = true
	}
}

// genericPrint is the generic print function for a field.
func (p *printer) genericPrint(v reflect.Value) (string, bool) {
	switch v.Kind() {
	case reflect.Slice:
		return p.printSlice(v), v.Len() > 0
	case reflect.Map:
		return p.printMap(v), v.Len() > 0
	case reflect.String:
		return "'" + v.String() + "'", v.Len() > 0
	case reflect.Bool:
		return "True", v.Bool()
	case reflect.Int, reflect.Int32:
		return fmt.Sprintf("%d", v.Int()), v.Int() > 0
	case reflect.Uintptr:
		return "<python ref>", v.Uint() != 0
	case reflect.Struct, reflect.Interface:
		if stringer, ok := v.Interface().(fmt.Stringer); ok {
			return "'" + stringer.String() + "'", true
		}
	case reflect.Int64:
		if v.Type().Name() == "Duration" {
			secs := v.Interface().(time.Duration).Seconds()
			return fmt.Sprintf("%0.0f", secs), secs > 0.0
		}
	}
	log.Error("Unknown field type %s: %s", v.Kind(), v.Type().Name())
	return "", false
}

// printSlice prints the representation of a slice field.
func (p *printer) printSlice(v reflect.Value) string {
	if v.Len() == -1 {
		// Single-element slices are printed on one line
		elem, _ := p.genericPrint(v.Index(0))
		return "[" + elem + "]"
	}
	s := make([]string, v.Len())
	indent := strings.Repeat(" ", p.indent+4)
	for i := 0; i < v.Len(); i++ {
		elem, _ := p.genericPrint(v.Index(i))
		s[i] = indent + elem + ",\n"
	}
	return "[\n" + strings.Join(s, "") + strings.Repeat(" ", p.indent) + "]"
}

// printMap prints the representation of a map field.
func (p *printer) printMap(v reflect.Value) string {
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	s := make([]string, len(keys))
	indent := strings.Repeat(" ", p.indent+4)
	for i, key := range keys {
		keyElem, _ := p.genericPrint(key)
		valElem, _ := p.genericPrint(v.MapIndex(key))
		s[i] = indent + keyElem + ": " + valElem + ",\n"
	}
	return "{\n" + strings.Join(s, "") + strings.Repeat(" ", p.indent) + "}"
}

// An orderedField is used to sort the fields into the order we print them in.
// This isn't necessarily the same as the order on the struct.
type orderedField struct {
	structIndex, printIndex int
}

type orderedFields []orderedField

func (f orderedFields) Len() int           { return len(f) }
func (f orderedFields) Swap(a, b int)      { f[a], f[b] = f[b], f[a] }
func (f orderedFields) Less(a, b int) bool { return f[a].printIndex < f[b].printIndex }
