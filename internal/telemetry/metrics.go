package telemetry

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Registry *prometheus.Registry
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	Orders   *prometheus.CounterVec
	Payments *prometheus.CounterVec
	Retries  prometheus.Counter
	Backlog  prometheus.Gauge
	Oldest   prometheus.Gauge
}

func New(service, version string) *Metrics {
	r := prometheus.NewRegistry()
	labels := prometheus.Labels{"service": service, "version": version}
	m := &Metrics{Registry: r,
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ticket_http_requests_total", Help: "HTTP responses by normalized route.", ConstLabels: labels}, []string{"method", "route", "status"}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ticket_http_duration_seconds", Help: "HTTP request duration.", ConstLabels: labels, Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 5}}, []string{"method", "route"}),
		Orders:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ticket_orders_created_total", Help: "Accepted orders.", ConstLabels: labels}, []string{"scenario"}),
		Payments: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ticket_payments_total", Help: "Completed simulated payments.", ConstLabels: labels}, []string{"outcome"}),
		Retries:  prometheus.NewCounter(prometheus.CounterOpts{Name: "ticket_payment_retries_total", Help: "Scheduled payment retries.", ConstLabels: labels}),
		Backlog:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "ticket_payment_backlog", Help: "Unfinished payment jobs (database-wide).", ConstLabels: labels}),
		Oldest:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "ticket_payment_oldest_seconds", Help: "Age of oldest unfinished job.", ConstLabels: labels}),
	}
	r.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}), m.Requests, m.Duration, m.Orders, m.Payments, m.Retries, m.Backlog, m.Oldest)
	return m
}
