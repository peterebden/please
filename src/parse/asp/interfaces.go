package asp

// TODO(peterebden): change this to a test file when I'm not on a plane...

func testInterfacesAreFulfilled() {
	var f freezable
	var len lengthable
	var it iterable
	var idx indexable
	var idxa indexAssignable

	s := pyString("")
	len = s
	it = s
	idx = s

	l := pyList{}
	f = l
	len = l
	it = l
	idx = l
	idxa = l

	fl := pyFrozenList{}
	f = fl
	len = fl
	it = fl
	idx = fl
	idxa = fl

	// N.B. indexable and indexAssignable aren't used on dicts. Maybe they should be?
	d := pyDict{}
	f = d
	len = d

	fd := pyFrozenDict{}
	f = fd
	len = fd

	dv := dictView{}
	len = dv
	it = dv

	f = f
	len = len
	it = it
	idx = idx
	idxa = idxa
}
