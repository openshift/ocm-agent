package ocm

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
)

func TestOCMClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ocm client Suite")
}

var _ = Describe("OCM client Handler", func() {
	var (
		ocmConnection          *sdk.Connection
		ocmClient              OCMClient
		serviceLog             *ServiceLog
		err                    error
		mockServer             *Server
		clusterUUID            string
		limitedSupportReason   *cmv1.LimitedSupportReason
		limitedSupportReasonID string
	)

	// Setup the testing environment to make mock server, OCM connection and mock token.
	BeforeEach(func() {
		mockServer = NewServer()

		accessToken := MakeTokenString("Bearer", 15*time.Minute)
		ocmConnection, err = sdk.NewConnectionBuilder().
			URL(mockServer.URL()).
			Tokens(accessToken).
			Build()
		Expect(err).NotTo(HaveOccurred())

		ocmClient = NewOcmClient(ocmConnection)
		serviceLog = testconst.NewTestServiceLog(
			ServiceLogActivePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogActiveDesc,
			testconst.TestHostedClusterID,
			testconst.TestNotification.Severity,
			testconst.TestNotification.LogType,
			testconst.TestNotification.References,
		)

		clusterUUID = "BD845DE4-5C16-4067-A868-15B02D55CCEF"
		limitedSupportReasonID = "D6BB28C8-0EB2-4EB3-B843-91802F1538C4"
		limitedSupportReason, _ = cmv1.NewLimitedSupportReason().Summary("Test limited support").Details("Limited support due to test").DetectionType(cmv1.DetectionTypeManual).Build()

		mockServer.AppendHandlers(
			RespondWith(http.StatusCreated, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
		)
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Context("Posting a service log", func() {
		It("should not return an error on successful post", func() {
			err := ocmClient.SendServiceLog(serviceLog)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error on failed post", func() {
			// Setup the mock server to respond with an error for this specific test case
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("POST", "/api/service_logs/v1/cluster_logs"),
				RespondWith(
					http.StatusInternalServerError,
					`{"kind": "Error", "id": "400", "href": "/api/service_logs/v1/errors/400", "code": "SERVICE-LOGS-400", "reason": "An internal server error occurred"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			err := ocmClient.SendServiceLog(serviceLog)
			Expect(err).To(HaveOccurred())

			expectedErrorMessage := "can't post service log: status is 500, identifier is '400' and code is 'SERVICE-LOGS-400': An internal server error occurred"
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})

	})
	Context("Limit support", func() {
		It("should not return an error on successful post", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("POST", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
				RespondWith(
					http.StatusCreated,
					`{"kind": "LimitedSupportReason", "details": "Limited support due to test","detection_type": "manual","summary": "Test limited support"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			err := ocmClient.SendLimitedSupport(clusterUUID, limitedSupportReason)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error when no internal id was found", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":0,"size":0,"total":0,"items": []}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			err := ocmClient.SendLimitedSupport(clusterUUID, limitedSupportReason)
			Expect(err).To(HaveOccurred())

			expectedErrorMessage := fmt.Sprintf("can't get internal id: cluster with external id %s not found in OCM database", clusterUUID)
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})

		It("should return an error on failed post", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("POST", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
				RespondWith(
					http.StatusInternalServerError,
					`{"kind": "Error", "id": "400", "href": "/api/clusters_mgmt/v1/errors/400", "code": "CLUSTERS-MGMT-400", "reason": "An internal server error occurred"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			err := ocmClient.SendLimitedSupport(clusterUUID, limitedSupportReason)
			Expect(err).To(HaveOccurred())
		})

		It("should not return an error on successfully delete limited support", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			requestPath := fmt.Sprintf("/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/%s", limitedSupportReasonID)
			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("DELETE", requestPath),
				RespondWith(
					http.StatusOK,
					``,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			err := ocmClient.RemoveLimitedSupport(clusterUUID, limitedSupportReasonID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error on failed deletion", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			requestPath := fmt.Sprintf("/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/%s", limitedSupportReasonID)
			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("DELETE", requestPath),
				RespondWith(
					http.StatusInternalServerError,
					`{"kind": "Error", "id": "400", "href": "/api/clusters_mgmt/v1/errors/400", "code": "CLUSTERS-MGMT-400", "reason": "An internal server error occurred"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			err := ocmClient.RemoveLimitedSupport(clusterUUID, limitedSupportReasonID)
			Expect(err).To(HaveOccurred())
		})

		It("should not return an error on successfully get limited support", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
				RespondWith(
					http.StatusOK,
					`{"kind":"LimitedSupportReasonList","page":1,"size":1,"total":1,"items": 
						[
							{
								"kind":"LimitedSupportReason",
      							"id":"bef7fe9e-105d-11f0-ad74-0a580a800465",
      							"href":"/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/bef7fe9e-105d-11f0-ad74-0a580a800465",
      							"summary":"TEST",
      							"details":"TEST_DETAIL"
				            }
						]
					}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			limitedSupportReasons, err := ocmClient.GetLimitedSupportReasons(clusterUUID)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(limitedSupportReasons)).To(Equal(1))
		})

		It("should return an error on failed to get limited support", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			mockServer.AppendHandlers(CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
				RespondWith(
					http.StatusNotFound,
					`{"kind": "Error", "id": "404", "href": "/api/clusters_mgmt/v1/errors/404", "code": "CLUSTERS-MGMT-404", "reason": "The requested resource doesn't exist"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			limitedSupportReasons, err := ocmClient.GetLimitedSupportReasons(clusterUUID)
			Expect(err).To(HaveOccurred())
			expectedErrorMessage := "can't get limited support reasons: status is 404, identifier is '404' and code is 'CLUSTERS-MGMT-404': The requested resource doesn't exist"
			Expect(err.Error()).To(Equal(expectedErrorMessage))

			Expect(limitedSupportReasons).To(BeNil())

		})
	})
})
