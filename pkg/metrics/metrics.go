package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift/ocm-agent/pkg/consts"
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

	MetricRequestFailure = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_request_failure",
			Help: "Indicates that OCM Agent could not successfully process a request",
		}, []string{"path"})

	MetricResponseFailure = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_response_failure",
			Help: "Indicates that the call to the OCM service endpoint failed",
		}, []string{"ocm_service"})

	metricServiceLogSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocm_agent_service_log_sent",
			Help: "A count of service log sent based on managedNotification template for the current session",
		}, []string{"ocm_service", "template", "state"})

	metricFailedServiceLogsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocm_agent_failed_service_logs_total",
			Help: "A count of service logs which failed to be sent. This includes service logs which failed to be formatted.",
		}, []string{"ocm_service", "template"})

	metricServiceLogSentTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_service_log_sent_total",
			Help: "A total number of service log being sent based on managedNotification template",
		}, []string{"ocm_service", "template"})

	metricPullSecretInvalid = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ocm_agent_pull_secret_invalid",
			Help: "Pull Secret auth token is not valid",
		}, []string{})

	metricsList = []prometheus.Collector{
		metricRequestsTotal,
		metricFailedRequestsTotal,
		metricRequestsByService,
		MetricRequestFailure,
		MetricResponseFailure,
		metricServiceLogSent,
		metricFailedServiceLogsTotal,
		metricServiceLogSentTotal,
		metricPullSecretInvalid,
	}
)

func init() {
	for _, m := range metricsList {
		_ = prometheus.Register(m)
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
		path := getRouteName(r)
		if path != consts.LivezPath && path != consts.ReadyzPath {
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

// getRouteName safely extracts route from the request, preferring gorilla mux route if available
func getRouteName(r *http.Request) string {
	if mux.CurrentRoute(r) != nil {
		if path, err := mux.CurrentRoute(r).GetPathTemplate(); err == nil {
			if len(path) > 0 {
				return path
			}
		}
	}
	return r.RequestURI
}

// SetResponseMetricFailure sets the metric when a call to the external service has failed
func SetResponseMetricFailure(service string) {
	MetricResponseFailure.With(prometheus.Labels{
		"ocm_service": service,
	}).Set(float64(1))
}

// SetRequestMetricFailure sets the metric when a call on ocm agent service with path has failed
func SetRequestMetricFailure(path string) {
	MetricRequestFailure.With(prometheus.Labels{
		"path": path,
	}).Set(float64(1))
}

// CountServiceLogSent counts the total number of service log sent by notification template
func CountServiceLogSent(template, state string) {
	metricServiceLogSent.With(prometheus.Labels{
		"ocm_service": "service_logs",
		"template":    template,
		"state":       state,
	}).Inc()
}

// CountFailedServiceLogs counts the total number of failed service logs (by notification template)
func CountFailedServiceLogs(template string) {
	metricFailedServiceLogsTotal.With(prometheus.Labels{
		"ocm_service": "service_logs",
		"template":    template,
	}).Inc()
}

// SetTotalServiceLogCount used to set the total sent service log number based on the managedNotification status
func SetTotalServiceLogCount(template string, count int32) {
	metricServiceLogSentTotal.With(prometheus.Labels{
		"ocm_service": "service_logs",
		"template":    template,
	}).Set(float64(count))
}

// SetPullSecretInvalidMetricSuccess sets the metric when ocm connection is successful
func SetPullSecretInvalidMetricSuccess() {
	metricPullSecretInvalid.WithLabelValues().Set(float64(0))
}

// SetPullSecretInvalidMetricFailure sets the metric when ocm connection can not be initiated
func SetPullSecretInvalidMetricFailure() {
	metricPullSecretInvalid.WithLabelValues().Set(float64(1))
}

// ResetMetric reset the metric with Gauge values
func ResetMetric(m *prometheus.GaugeVec) {
	m.Reset()
}
