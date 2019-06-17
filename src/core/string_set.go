package core

// A StringSet holds a series of string, which can optionally be split
// into named groups. This is used to handle a common idiom where we allow
// specifying a set of things as either named or not.
//
// This is just about the one place in plz where the ability to write generics would
// actually be useful. When Go 2 comes along we can unify this with InputSet.
type StringSet struct {
	items []stringSetItem
}

type stringSetItem struct {
	Name    string
	Strings []string
}

// Add adds a single unnamed input to this set.
func (s *StringSet) Add(input string) {
	switch len(s.items) {
	case 0:
		s.items = append(s.items, stringSetItem{Strings: []string{input}})
	case 1:
		if s.items[0].Name != "" {
			panic("Adding unnamed input to an StringSet that already has a named input")
		}
		s.items[0].Strings = append(s.items[0].Strings, input)
	default:
		panic("Adding unnamed input to an StringSet that already has named inputs")
	}
}

// AddNamed adds a single named input to this set.
func (s *StringSet) AddNamed(name string, input string) {
	for i, item := range s.items {
		if item.Name == name {
			s.items[i].Strings = append(item.Strings, input)
			return
		}
	}
	s.items = append(s.items, stringSetItem{Name: name, Strings: []string{input}})
}

// Set sets the contents of this input set to those given. They're unnamed.
func (s *StringSet) Set(inputs []string) {
	s.items = []stringSetItem{{Strings: inputs}}
}

// AddAllNamed adds the given set of named inputs. No checking for duplicates is done.
func (s *StringSet) AddAllNamed(name string, inputs []string) {
	s.items = append(s.items, stringSetItem{Name: name, Strings: inputs})
}

// All returns all of the inputs from this set as a single slice.
func (s *StringSet) All() []string {
	// Quiet little optimisation; don't make another set & copy if we already have one to hand
	if len(s.items) == 1 {
		return s.items[0].Strings
	} else if len(s.items) == 0 {
		return nil
	}
	ret := make([]string, 0, s.Count())
	for _, i := range s.items {
		ret = append(ret, i.Strings...)
	}
	return ret
}

// Named returns all of the inputs with the given name
func (s *StringSet) Named(name string) []string {
	for _, item := range s.items {
		if item.Name == name {
			return item.Strings
		}
	}
	return nil
}

// Count returns the total number of inputs in this set
func (s *StringSet) Count() int {
	n := 0
	for _, i := range s.items {
		n += len(i.Strings)
	}
	return n
}

// IsEmpty returns true if this set contains no entries.
func (s *StringSet) IsEmpty() bool {
	return len(s.items) == 0
}

// Names returns the current set of names in the set.
func (s *StringSet) Names() []string {
	if !s.IsNamed() {
		return nil
	}
	ret := make([]string, len(s.items))
	for i, item := range s.items {
		ret[i] = item.Name
	}
	return ret
}

// IsNamed returns true if this set has named entries in it.
func (s *StringSet) IsNamed() bool {
	return len(s.items) > 0 && s.items[0].Name != ""
}

func (s *StringSet) Contains(input string) bool {
	for _, item := range s.items {
		for _, i := range item.Strings {
			if i == input {
				return true
			}
		}
	}
	return false
}
