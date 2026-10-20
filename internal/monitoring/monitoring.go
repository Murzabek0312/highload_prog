package monitoring

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Monitor struct {
	mux      *http.ServeMux
	routes   []string
	duration *prometheus.HistogramVec
	requests *prometheus.CounterVec
}

func New(mux *http.ServeMux, pod, datacenter, deployment string) *Monitor {
	labels := prometheus.Labels{
		"pod":        pod,
		"datacenter": datacenter,
		"deployment": deployment,
	}

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests.",
			ConstLabels: labels,
		},
		[]string{"route", "status"})

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request handling duration in seconds",
			ConstLabels: labels,
		},
		[]string{"route"})

	registry := prometheus.NewRegistry()
	registry.MustRegister(duration, requests)

	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return &Monitor{
		mux:      mux,
		duration: duration,
		requests: requests,
	}
}

func (m *Monitor) HandleFunc(pattern string, handler http.HandlerFunc) {
	observer := m.duration.WithLabelValues(pattern)

	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		writer := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		start := time.Now()

		defer func() {
			observer.Observe(time.Since(start).Seconds())
			m.requests.WithLabelValues(pattern,
				strconv.Itoa(writer.status)).Inc()
		}()

		handler(writer, r)
	})

	m.routes = append(m.routes, pattern)
}

func (m *Monitor) Routes() []string {
	return slices.Clone(m.routes)
}
