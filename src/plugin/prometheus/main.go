package main

import (
	"github.com/thought-machine/please/src/metrics/prometheus"
)

// Register registers Prometheus metrics for this instance of Please.
func Register() {
	prometheus.Register()
}

// Required for a plugin but not used.
func main() {}
