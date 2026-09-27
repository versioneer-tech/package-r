// Package metrics provides packageR Prometheus metrics.
package metrics

import (
	"context"
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rclone/rclone/fs/accounting"

	"github.com/versioneer-tech/package-r/rclonefs"
)

const namespace = "package_r"

// VFSStatsProvider returns aggregate VFS statistics.
type VFSStatsProvider interface {
	Stats() rclonefs.Stats
}

// Metrics owns the collectors exposed by one packageR process.
type Metrics struct {
	registry      *prometheus.Registry
	handler       http.Handler
	httpRequests  *prometheus.CounterVec
	httpDuration  *prometheus.HistogramVec
	httpInFlight  prometheus.Gauge
	logins        *prometheus.CounterVec
	tokenRenewals prometheus.Counter
	tusUploads    *prometheus.CounterVec
	presigns      *prometheus.CounterVec
}

// New creates an isolated registry with packageR, rclone, and VFS metrics.
func New(ctx context.Context, vfs VFSStatsProvider) *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being handled.",
		}),
		logins: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "logins_total",
			Help:      "Total number of login results by authentication method.",
		}, []string{"method", "result"}),
		tokenRenewals: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "token_renewals_total",
			Help:      "Total number of successfully renewed packageR session tokens.",
		}),
		tusUploads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "tus",
			Name:      "uploads_total",
			Help:      "Total number of TUS upload lifecycle events.",
		}, []string{"result"}),
		presigns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "presign",
			Name:      "requests_total",
			Help:      "Total number of object-storage presign results.",
		}, []string{"result"}),
	}

	m.registry.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.httpInFlight,
		m.logins,
		m.tokenRenewals,
		m.tusUploads,
		m.presigns,
		accounting.NewRcloneCollector(ctx),
	)
	if vfs != nil {
		m.registry.MustRegister(newVFSCollector(vfs))
	}

	for _, method := range []string{"json", "proxy", "hook", "unknown"} {
		for _, result := range []string{"success", "failure"} {
			m.logins.WithLabelValues(method, result).Add(0)
		}
	}
	for _, result := range []string{"started", "completed", "aborted", "failed"} {
		m.tusUploads.WithLabelValues(result).Add(0)
	}
	for _, result := range []string{"success", "failure"} {
		m.presigns.WithLabelValues(result).Add(0)
	}

	m.handler = promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	return m
}

// Handler returns the Prometheus/OpenMetrics endpoint handler.
func (m *Metrics) Handler() http.Handler {
	return m.handler
}

// HTTPMiddleware records aggregate HTTP request metrics.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeTemplate(r)
		if route == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		m.httpInFlight.Inc()
		defer m.httpInFlight.Dec()

		result := httpsnoop.CaptureMetrics(next, w, r)
		m.httpDuration.WithLabelValues(r.Method, route).Observe(result.Duration.Seconds())
		m.httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(result.Code)).Inc()
	})
}

// ObserveLogin records one completed login attempt.
func (m *Metrics) ObserveLogin(method string, success bool) {
	if m == nil {
		return
	}
	method = normalizedAuthMethod(method)
	result := "failure"
	if success {
		result = "success"
	}
	m.logins.WithLabelValues(method, result).Inc()
}

// ObserveTokenRenewal records one successfully renewed session token.
func (m *Metrics) ObserveTokenRenewal() {
	if m != nil {
		m.tokenRenewals.Inc()
	}
}

// ObserveTUS records a TUS lifecycle event.
func (m *Metrics) ObserveTUS(result string) {
	if m != nil {
		m.tusUploads.WithLabelValues(result).Inc()
	}
}

// ObservePresign records one object-storage presign result.
func (m *Metrics) ObservePresign(success bool) {
	if m == nil {
		return
	}
	result := "failure"
	if success {
		result = "success"
	}
	m.presigns.WithLabelValues(result).Inc()
}

func routeTemplate(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return "unmatched"
	}
	template, err := route.GetPathTemplate()
	if err != nil || template == "" {
		return "unmatched"
	}
	return template
}

func normalizedAuthMethod(method string) string {
	switch method {
	case "json", "proxy", "hook":
		return method
	default:
		return "unknown"
	}
}
