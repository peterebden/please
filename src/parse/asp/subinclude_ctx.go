package asp

import (
	"context"
)

// subincludeChain records the chain of subincluded files that led to the current one.
// It's carried on the context so it follows the goroutine chain doing the work, which can
// descend into parsing other packages inline.
type subincludeChain struct {
	parent *subincludeChain
	path   string
}

type subincludeKey struct{}

// withSubinclude returns a context that records that we are subincluding the given path.
func withSubinclude(ctx context.Context, path string) context.Context {
	parent, _ := ctx.Value(subincludeKey{}).(*subincludeChain)
	return context.WithValue(ctx, subincludeKey{}, &subincludeChain{parent: parent, path: path})
}

// hasSubinclude returns true if the given path is already being subincluded on this chain.
func hasSubinclude(ctx context.Context, path string) bool {
	for c, _ := ctx.Value(subincludeKey{}).(*subincludeChain); c != nil; c = c.parent {
		if c.path == path {
			return true
		}
	}
	return false
}

// allSubincludes returns the chain of subincludes leading to the current point, outermost first.
func allSubincludes(ctx context.Context) []string {
	var ret []string
	for c, _ := ctx.Value(subincludeKey{}).(*subincludeChain); c != nil; c = c.parent {
		ret = append(ret, c.path)
	}
	for i, j := 0, len(ret)-1; i < j; i, j = i+1, j-1 {
		ret[i], ret[j] = ret[j], ret[i]
	}
	return ret
}
