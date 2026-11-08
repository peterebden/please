package fs

import (
	iofs "io/fs"
	"testing"
)

func TestHostFSImplementsStatFS(t *testing.T) {
	var statFS iofs.StatFS = HostFS
	statFS = statFS
}

func TestHostFSImplementsReadDirFS(t *testing.T) {
	var rdFS iofs.ReadDirFS = HostFS
	rdFS = rdFS
}
