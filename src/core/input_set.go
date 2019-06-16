package core

// An InputSet holds a series of BuildInputs, which can optionally be split
// into named groups. This is used to handle a common idiom where we allow
// specifying a set of things as either named or not.
type InputSet struct {
	items []inputSetItem
}

type inputSetItem struct {
	Name   string
	Inputs []BuildInput
}

// Add adds a single unnamed input to this set.
func (s *InputSet) Add(input BuildInput) {
	switch len(s.items) {
	case 0:
		s.items = append(s.items, inputSetItem{Inputs: []BuildInput{input}})
	case 1:
		if s.items[0].Name != "" {
			panic("Adding unnamed input to an InputSet that already has a named input")
		}
		s.items[0].Inputs = append(s.items[0].Inputs, input)
	default:
		panic("Adding unnamed input to an InputSet that already has named inputs")
	}
}

// AddNamed adds a single named input to this set.
func (s *InputSet) AddNamed(name string, input BuildInput) {
	for i, item := range s.items {
		if item.Name == name {
			s.items[i].Inputs = append(item.Inputs, input)
			return
		}
	}
	s.items = append(s.items, inputSetItem{Name: name, Inputs: []BuildInput{input}})
}

// Set sets the contents of this input set to those given. They're unnamed.
func (s *InputSet) Set(inputs []BuildInput) {
	s.items = []inputSetItem{{Inputs: inputs}}
}

// AddAllNamed adds the given set of named inputs. No checking for duplicates is done.
func (s *InputSet) AddAllNamed(name string, inputs []BuildInput) {
	s.items = append(s.items, inputSetItem{Name: name, Inputs: inputs})
}

// All returns all of the inputs from this set as a single slice.
func (s *InputSet) All() []BuildInput {
	// Quiet little optimisation; don't make another set & copy if we already have one to hand
	if len(s.items) == 1 {
		return s.items[0].Inputs
	}
	ret := make([]BuildInput, 0, s.Count())
	for _, i := range s.items {
		ret = append(ret, i.Inputs...)
	}
	return ret
}

// Named returns all of the inputs with the given name
func (s *InputSet) Named(name string) []BuildInput {
	for _, item := range s.items {
		if item.Name == name {
			return item.Inputs
		}
	}
	return nil
}

// Count returns the total number of inputs in this set
func (s *InputSet) Count() int {
	n := 0
	for _, i := range s.items {
		n += len(i.Inputs)
	}
	return n
}

// Names returns the current set of names in the set.
func (s *InputSet) Names() []string {
	ret := make([]string, len(s.items))
	for i, item := range s.items {
		ret[i] = item.Name
	}
	return ret
}

// IsNamed returns true if this set has named entries in it.
func (s *InputSet) IsNamed() bool {
	return len(s.items) > 0 && s.items[0].Name != ""
}
