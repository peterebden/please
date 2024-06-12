package asp

import (
	"iter"
)

// A tree implements a simple append-only tree structure, which can act as an ordered map
type tree struct {
	root *node
	len  int
	cap  []node
}

type node struct {
	K           string
	V           pyObject
	Left, Right *node
}

// Insert adds the given node to this tree, overwriting any previous element with the same key.
func (t *tree) Insert(k string, v pyObject) {
	t.insert(&t.root, k, v)
	t.len++
}

func (t *tree) insert(n **node, k string, v pyObject) {
	if *n == nil {
		if len(t.cap) == 0 {
			*n = &node{K: k, V: v}
		} else {
			t.cap[0].K = k
			t.cap[0].V = v
			*n = &t.cap[0]
			t.cap = t.cap[1:]
		}
	} else if no := *n; no.K == k {
		no.V = v
	} else if no.K > k {
		t.insert(&no.Left, k, v)
	} else {
		t.insert(&no.Right, k, v)
	}
	// Clearly we should rebalance here...
}

// Get returns the item with the given key, or nil if it doesn't exist.
func (t *tree) Get(k string) pyObject {
	return t.get(t.root, k)
}

func (t *tree) get(n *node, k string) pyObject {
	if n == nil {
		return nil
	} else if n.K == k {
		return n.V
	} else if n.K > k {
		return t.get(n.Left, k)
	} else {
		return t.get(n.Right, k)
	}
}

// Keys returns an iterator over the set of keys for this tree
func (t *tree) Keys() sequence {
	return func(yield func(pyObject) bool) {
		t.root.keys(yield)
	}
}

// Values returns an iterator over the set of values for this tree
func (t *tree) Values() sequence {
	return func(yield func(pyObject) bool) {
		t.root.values(yield)
	}
}

// Items returns an iterator over the set of key-value pairs for this tree
func (t *tree) Items() sequence {
	return func(yield func(pyObject) bool) {
		t.root.items(yield)
	}
}

// KVs returns an iterator over the raw set of key-value pairs for this tree
func (t *tree) KVs() iter.Seq2[string, pyObject] {
	return func(yield func(string, pyObject) bool) {
		t.root.kvs(yield)
	}
}

func (n *node) keys(yield func(pyObject) bool) bool {
	if n == nil {
		return true // doesn't terminate further iteration
	}
	return n.Left.keys(yield) && yield(pyString(n.K)) && n.Right.keys(yield)
}

func (n *node) values(yield func(pyObject) bool) bool {
	if n == nil {
		return true // doesn't terminate further iteration
	}
	return n.Left.values(yield) && yield(n.V) && n.Right.values(yield)
}

func (n *node) items(yield func(pyObject) bool) bool {
	if n == nil {
		return true // doesn't terminate further iteration
	}
	return n.Left.items(yield) && yield(pyList{pyString(n.K), n.V}) && n.Right.items(yield)
}

func (n *node) kvs(yield func(string, pyObject) bool) bool {
	if n == nil {
		return true // doesn't terminate further iteration
	}
	return n.Left.kvs(yield) && yield(n.K, n.V) && n.Right.kvs(yield)
}
