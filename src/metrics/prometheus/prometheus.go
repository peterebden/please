// Package prometheus provides an implementation of metrics for Please using Prometheus as a backendpackage metrics
package prometheus

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"

	"github.com/thought-machine/please/src/cli/logging"
	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/metrics"
)

var log = logging.Log

// Register registers this implementation as the active one for Please
func Register() {
	metrics.SetImplementation(&prom{
		registerer: prometheus.WrapRegistererWith(prometheus.Labels{
			"version": core.PleaseVersion,
		}, prometheus.DefaultRegisterer),
	})
}

// prom is the concrete implementation of metrics using Prometheus
type prom struct {
	registerer prometheus.Registerer
}

// Push performs a single push of all registered metrics to the pushgateway (if configured).
func (p *prom) Push(config *core.Configuration) {
	if family, err := prometheus.DefaultGatherer.Gather(); err == nil {
		for _, fam := range family {
			for _, metric := range fam.Metric {
				if metric.Counter != nil {
					log.Debug("Metric recorded: %s: %0.0f", *fam.Name, *metric.Counter.Value)
				}
			}
		}
	}

	if err := push.New(config.Metrics.PrometheusGatewayURL, "please").
		Client(&http.Client{Timeout: time.Duration(config.Metrics.Timeout)}).
		Gatherer(prometheus.DefaultGatherer).Format(expfmt.FmtText).
		Push(); err != nil {
		log.Warning("Error pushing Prometheus metrics: %s", err)
	}
}

// RegisterCounter registers a new counter with Prometheus
func (p *prom) RegisterCounter(counter *metrics.Counter) metrics.Incrementer {
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "plz",
		Subsystem: counter.Subsystem,
		Name:      counter.Name,
		Help:      counter.Help,
	})
	p.registerer.MustRegister(c)
	return c
}

// RegisterHistogram registers a new histogram with Prometheus
func (p *prom) RegisterHistogram(hist *metrics.Histogram) metrics.Observer {
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "plz",
		Subsystem: hist.Subsystem,
		Name:      hist.Name,
		Help:      hist.Help,
		Buckets:   hist.Buckets,
	})
	p.registerer.MustRegister(h)
	return h
}
