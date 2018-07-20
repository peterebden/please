//+build !bootstrap

package remote

// A buildServer handles remote build requests.
// It interacts a little with an eventServer; we serve both at once but the
// events track an individual build, whereas the build server stays up indefinitely.
type buildServer struct {
	Callback    CallbackFunc
	EventServer *eventServer
}

func (b *buildServer)
