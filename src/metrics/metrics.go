// Package metrics provides a generic internal interface to metrics for Please
// It's pretty heavily based on Prometheus but serves to separate us from the actual Prometheus implementation.
package metrics

import (
	"github.com/thought-machine/please/src/cli/logging"
	"github.com/thought-machine/please/src/core"
)

var log = logging.Log

// Metrics is the interface we require from an implementation of metrics
type Metrics interface {
	RegisterCounter(counter *Counter) Incrementer
	RegisterHistogram(histogram *Histogram) Observer
	Push(config *core.Configuration)
}

var implementation Metrics

// SetImplementation provides a backing implementation of metrics
func SetImplementation(impl Metrics) {
	for _, counter := range counters {
		counter.counter = impl.RegisterCounter(counter)
	}
	for _, histogram := range histograms {
		histogram.hist = impl.RegisterHistogram(histogram)
	}
	implementation = impl
}

// Push makes a single push to the metrics backend
func Push(config *core.Configuration) {
	if implementation != nil {
		implementation.Push(config)
	}
}

// An Incrementer is the interface needed backing a Counter
type Incrementer interface {
	Inc()
}

// A Counter is a metric that counts up a unitless quantity.
type Counter struct {
	Subsystem, Name, Help string
	counter               Incrementer
}

// Inc increments the counter by one.
func (counter *Counter) Inc() {
	counter.counter.Inc()
}

type noopCounter struct{}

func (n noopCounter) Inc() {}

var counters []*Counter

// NewCounter creates & registers a new counter.
// This should be called statically at init time.
func NewCounter(subsystem, name, help string) *Counter {
	counter := &Counter{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		counter:   noopCounter{},
	}
	counters = append(counters, counter)
	return counter
}

// An Observer is the interface needed backing a Histogram
type Observer interface {
	Observe(float64)
}

// A Histogram counts individual observations of values in buckets.
type Histogram struct {
	Subsystem, Name, Help string
	hist                  Observer
}

// Observe adds an observation to the histogram
func (hist *Histogram) Observe(duration float64) {
	hist.hist.Observe(duration)
}

type noopHistogram struct{}

func (n noopHistogram) Observe(float64) {}

var histograms []*Histogram

// NewHistogram creates & registers a new histogram.
// This should be called statically at init time.
func NewHistogram(subsystem, name, help string, buckets []float64) *Histogram {
	histogram := &Histogram{
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		hist:      noopHistogram{},
	}
	histograms = append(histograms, histogram)
	return histogram
}
