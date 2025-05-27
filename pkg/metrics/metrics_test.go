package metrics

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/testutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Webhook Handlers", func() {

	var (
		testService         = "TestService"
		testMetricLabelName = "testMetricLabelName"
		testPath            = "/test-path"
		testState           = "test-state"
		testTemplate        = "test-template"
		testAlertName       = "testAlertName"
		server              *ghttp.Server
	)

	BeforeEach(func() {
		resetMetrics()
		server = ghttp.NewServer()
	})
	AfterEach(func() {
		server.Close()
	})

	Context("Prometheus Middleware", func() {
		var (
			resp *http.Response
			err  error
		)

		When("testing a successful call", func() {
			var (
				reqTotalHelpHeader = `
# HELP ocm_agent_requests_total A count of total requests to ocm agent service
# TYPE ocm_agent_requests_total counter
`
				reqTotalValueHeader = "ocm_agent_requests_total "

				reqServiceHelpHeader = `
# HELP ocm_agent_requests_by_service A count of total requests to ocm agent based on sub service
# TYPE ocm_agent_requests_by_service counter
`
				reqServiceValueHeader = fmt.Sprintf(`ocm_agent_requests_by_service{path="%s"}`, testPath)
			)
			BeforeEach(func() {
				// add handler to the server
				promHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
				testHandler := PrometheusMiddleware(promHandler)
				server.AppendHandlers(testHandler.ServeHTTP)
				resp, err = http.Get(server.URL() + testPath)
			})
			It("increments the success counters", func() {
				Expect(err).To(BeNil())
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
				expectedTotalMetric := fmt.Sprintf("%s%s%d\n", reqTotalHelpHeader, reqTotalValueHeader, 1)
				err = testutil.CollectAndCompare(metricRequestsTotal, strings.NewReader(expectedTotalMetric))
				Expect(err).To(BeNil())
				expectedServiceMetric := fmt.Sprintf("%s%s%d\n", reqServiceHelpHeader, reqServiceValueHeader, 1)
				err = testutil.CollectAndCompare(metricRequestsByService, strings.NewReader(expectedServiceMetric))
				Expect(err).To(BeNil())
			})
		})

		When("testing an unsuccessful call", func() {
			var (
				failedReqTotalHeader = `
# HELP ocm_agent_failed_requests_total A count of total failed requests received by the OCM Agent service
# TYPE ocm_agent_failed_requests_total counter
`
				failedReqValueHeader = "ocm_agent_failed_requests_total "
			)
			BeforeEach(func() {
				// add handler to the server
				promHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				})
				testHandler := PrometheusMiddleware(promHandler)
				server.AppendHandlers(testHandler.ServeHTTP)
				resp, err = http.Get(server.URL() + testPath)
			})
			It("increments the failure counter", func() {
				Expect(err).To(BeNil())
				Expect(resp.StatusCode).Should(Equal(http.StatusInternalServerError))
				expectedTotalMetric := fmt.Sprintf("%s%s%d\n", failedReqTotalHeader, failedReqValueHeader, 1)
				err = testutil.CollectAndCompare(metricFailedRequestsTotal, strings.NewReader(expectedTotalMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Response Failure metric", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_response_failure Indicates that the call to the OCM service endpoint failed
# TYPE ocm_agent_response_failure gauge
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_response_failure{alert_name = "%s", notification_name="%s", ocm_service="%s"} `, testAlertName, testMetricLabelName, testService)
		)
		When("the metric is set", func() {
			It("does so correctly", func() {
				SetResponseMetricFailure(testService, testMetricLabelName, testAlertName)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(MetricResponseFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
		When("the metric is reset", func() {
			It("does so correctly", func() {
				ResetResponseMetricFailure(testService, testMetricLabelName, testAlertName)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 0)
				err := testutil.CollectAndCompare(MetricResponseFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Request Failure metric", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_request_failure Indicates that OCM Agent could not successfully process a request
# TYPE ocm_agent_request_failure gauge
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_request_failure{path="%s"} `, testPath)
		)
		When("the metric is set", func() {
			It("does so correctly", func() {
				SetRequestMetricFailure(testPath)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(MetricRequestFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Service Log Sent metric", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_service_log_sent A count of service log sent based on managedNotification template for the current session
# TYPE ocm_agent_service_log_sent counter
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_service_log_sent{ocm_service="service_logs",state="%s",template="%s"} `, testState, testTemplate)
		)
		When("the metric is set once", func() {
			It("does so correctly", func() {
				CountServiceLogSent(testTemplate, testState)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricServiceLogSent, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
		When("the metric is set twice", func() {
			It("increments the metric", func() {
				CountServiceLogSent(testTemplate, testState)
				CountServiceLogSent(testTemplate, testState)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 2)
				err := testutil.CollectAndCompare(metricServiceLogSent, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

	})

	Context("Limited Support Sent metrics are updated correctly", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_limited_support_sent_total A total number of limited support being sent based on fleetNotification template
# TYPE ocm_agent_limited_support_sent_total counter
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_limited_support_sent_total{ocm_service="clusters_mgmt",template="%s"} `, testTemplate)
		)

		When("the metric is incremented", func() {
			It("increments the total sent limited support number correctly", func() {
				IncrementLimitedSupportSentCount(testTemplate)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricLimitedSupportSentTotal, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Limited Support Removed metrics are updated correctly", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_limited_support_removed_total A total number of limited support removed based on fleetNotification template
# TYPE ocm_agent_limited_support_removed_total counter
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_limited_support_removed_total{ocm_service="clusters_mgmt",template="%s"} `, testTemplate)
		)

		When("the metric is incremented", func() {
			It("increments the total removed limited support reasons number correctly", func() {
				IncrementLimitedSupportRemovedCount(testTemplate)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricLimitedSupportRemovedTotal, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Failed Limited Support Send metrics are updated correctly", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_limited_support_send_failure_total A total number of failures for limited support posts based on fleetNotification template
# TYPE ocm_agent_limited_support_send_failure_total counter
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_limited_support_send_failure_total{ocm_service="clusters_mgmt",template="%s"} `, testTemplate)
		)

		When("the metric is incremented", func() {
			It("increments the total number of failed limited support posts correctly", func() {
				IncrementFailedLimitedSupportSend(testTemplate)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricFailedLimitedSupportSendsTotal, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Failed Limited Support Removal metrics are updated correctly", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_limited_support_removal_failure_total A total number of failures for limited support removals based on fleetNotification template
# TYPE ocm_agent_limited_support_removal_failure_total counter
`
			metricValueHeader = fmt.Sprintf(`ocm_agent_limited_support_removal_failure_total{ocm_service="clusters_mgmt",template="%s"} `, testTemplate)
		)

		When("the metric is incremented", func() {
			It("increments the total number of failed limited support removals correctly", func() {
				IncrementFailedLimitedSupportRemoved(testTemplate)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricFailedLimitedSupportRemovalsTotal, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Pull Secret Invalid metric", func() {
		var (
			metricHelpHeader = `
# HELP ocm_agent_pull_secret_invalid Pull Secret auth token is not valid
# TYPE ocm_agent_pull_secret_invalid gauge
`
			metricValueHeader = "ocm_agent_pull_secret_invalid{} "
		)
		When("the metric is set", func() {
			It("does so correctly", func() {
				SetPullSecretInvalidMetricFailure()
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(metricPullSecretInvalid, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})
	})
})

func resetMetrics() {
	metricServiceLogSent.Reset()
	metricFailedServiceLogsTotal.Reset()
	MetricRequestFailure.Reset()
	MetricResponseFailure.Reset()
	metricRequestsTotal.Reset()
	metricFailedRequestsTotal.Reset()
	metricRequestsByService.Reset()
	metricPullSecretInvalid.Reset()
	metricFailedLimitedSupportRemovalsTotal.Reset()
	metricFailedLimitedSupportSendsTotal.Reset()
	metricLimitedSupportRemovedTotal.Reset()
	metricLimitedSupportSentTotal.Reset()
}
