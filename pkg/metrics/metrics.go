package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift/ocm-agent/pkg/handlers"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocm_agent_requests_total",
			Help: "A count of total requests to ocm agent service",
		}, []string{})

	metricRequestsByService = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocm_agent_requests_by_service",
			Help: "A count of total requests to ocm agent based on sub service",
		}, []string{"path"})

	metricFailedRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocm_agent_failed_requests_total",
			Help: "A count of total failed requests received by the OCM Agent service",
		}, []string{})

	metricRequestFailure = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_request_failure",
			Help: "Indicates that OCM Agent could not successfully process a request",
		}, []string{"path"})

	metricResponseFailure = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_response_failure",
			Help: "Indicates that the call to the OCM service endpoint failed",
		}, []string{"service"})

	metricsList = []prometheus.Collector{
		metricRequestsTotal,
		metricFailedRequestsTotal,
		metricRequestsByService,
		metricRequestFailure,
		metricResponseFailure,
	}
)

func init() {
	for _, m := range metricsList {
		prometheus.Register(m)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// NewResponseWriter rewrites the response based on the existing response
func NewResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

// WriteHeader writes the http return code to the response
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// A middleware to collect all the requests received by the web service
func PrometheusMiddleware(ph http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path, _ := mux.CurrentRoute(r).GetPathTemplate()
		if path != handlers.LivezPath && path != handlers.ReadyzPath {
			metricRequestsTotal.WithLabelValues().Inc()
			metricRequestsByService.WithLabelValues(path).Inc()
		}

		rw := NewResponseWriter(w)
		ph.ServeHTTP(rw, r)
		statusCode := rw.statusCode
		if statusCode != http.StatusOK {
			metricFailedRequestsTotal.WithLabelValues().Inc()
			SetRequestMetricFailure(path)
		}
	})
}

// SetResponseMetricFailure sets the metric when the call from ocm agent got failed
func SetResponseMetricFailure(endpoint string) {
	metricResponseFailure.With(prometheus.Labels{
		"service": endpoint,
	}).Set(float64(1))
}

// SetRequestMetricFailure sets the metric when the call to ocm agent got failed
func SetRequestMetricFailure(endpoint string) {
	metricRequestFailure.With(prometheus.Labels{
		"path": endpoint,
	}).Set(float64(1))
}
