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
	"github.com/openshift-online/ocm-sdk-go/logging"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	"github.com/prometheus/alertmanager/template"
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
		mockLogger             *logging.Logger
		transportWrapper       sdk.TransportWrapper
		clusterUUID            string
		clusterID              string
		clusterResponse        string
		clusterListReponse     string
		upgradePolicy1         string
		upgradePolicy2         string
		limitedSupportReason   *cmv1.LimitedSupportReason
		limitedSupportReasonID string
		testAlert              template.Alert
		upgradePolicyID        string
		upgraePolicyVersion    string
		upgradeNextRun         string
		upgradePolicyState     string
		upgradePoliciesList    string
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
		clusterID = "abcd1234efgh5678ijkl9123mnopqrst"
		upgradePolicyID = "aaaaaa-bbbbbb-cccccc-dddddd"
		upgraePolicyVersion = "4.16.1"
		upgradeNextRun = "2020-06-20T00:00:00Z"
		limitedSupportReasonID = "D6BB28C8-0EB2-4EB3-B843-91802F1538C4"
		limitedSupportReason, _ = cmv1.NewLimitedSupportReason().Summary("Test limited support").Details("Limited support due to test").DetectionType(cmv1.DetectionTypeManual).Build()
		testAlert = testconst.NewTestAlert(true, false)
		mockServer.AppendHandlers(
			RespondWith(http.StatusCreated, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
		)
		clusterResponse = `{"kind":"Cluster","id":"` + clusterID + `","externalID":"` + clusterUUID + `"}`
		clusterListReponse = `{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [` + clusterResponse + `]}`
		upgradePolicy1 = `{"kind":"UpgradePolicy","id":"` + upgradePolicyID + `","cluster_id":"` + clusterID + `","upgrade_type":"manual","version":"` + upgraePolicyVersion + `","next_run":"` + upgradeNextRun + `"}`
		upgradePolicy2 = `{"kind":"UpgradePolicy","id":"` + upgradePolicyID + `-222","cluster_id":"` + clusterID + `","upgrade_type":"manual","version":"` + upgraePolicyVersion + `","next_run":"` + upgradeNextRun + `"}`
		upgradePolicyState = `{
				"kind": "UpgradePolicyState",
				"id": "PolicyStateID",
				"href": "href",
				"description": "Upgrade Pending",
				"value": "pending"
		}`
		upgradePoliciesList = `{"page":1,"size":2,"total":2,"items": [` + upgradePolicy1 + `,` + upgradePolicy2 + `]}`
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Context("OCM Connection", func() {
		It("OCM connection should not return an error when building a new connection", func() {
			connBuilder := NewConnection()
			connBuilder = connBuilder.Logger(mockLogger)
			connBuilder.TransportWrapper(transportWrapper)
			conn, err := connBuilder.Build(mockServer.URL(), clusterUUID, MakeTokenString("Bearer", 15*time.Minute))
			Expect(err).NotTo(HaveOccurred())
			Expect(conn).ShouldNot(BeNil())
			Expect(conn.Logger()).ShouldNot(BeNil())

		})
	})

	Context("Internal ID from External", func() {
		It("should return internal cluster id without errors", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
				RespondWith(
					http.StatusOK,
					clusterListReponse,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			internalID, err := GetInternalIDByExternalID(clusterUUID, ocmConnection)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(internalID).ShouldNot(BeNil())
			Expect(internalID).To(Equal(clusterID))
		})

		It("should return an error when the cluster could not be found", func() {
			internalID, err := GetInternalIDByExternalID("a-ghost-cluster-id", ocmConnection)
			Expect(err).Should(HaveOccurred())
			Expect(internalID).Should(BeEmpty())
		})
	})

	Context("Service log builder", func() {
		It("service log should not return an error", func() {
			slbuilder := NewServiceLogBuilder(testconst.ServiceLogSummary, testconst.ServiceLogActiveDesc, testconst.TestNotification.ResolvedDesc,
				testconst.TestHostedClusterID, testconst.TestNotification.Severity, testconst.TestNotification.LogType, testconst.TestNotification.References)
			Expect(slbuilder).ShouldNot(BeNil())
		})
	})

	Context("Replace place holders in the given string with the alert labels and annotations", func() {
		It("shoudn't have errors when the replacement label/annotation found in the template", func() {
			replaceString, err := replacePlaceHoldersInString("failure regarding the alert '${alertname}'. The initial issue ", &testAlert)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(replaceString).To(Equal("failure regarding the alert 'TestAlertName'. The initial issue "))
		})
		It("should error when the replacement label/annotation not found in the template", func() {
			replaceString, err := replacePlaceHoldersInString("failure regarding the alert '${ALERT_NAME}'. The initial issue ", &testAlert)
			Expect(err).Should(HaveOccurred())
			Expect(replaceString).To(Equal("failure regarding the alert '${ALERT_NAME}'. The initial issue "))
		})
	})
	Context("Get Cluster", func() {
		It("should return the cluster without an error", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", clusterID)),
				RespondWith(
					http.StatusOK,
					clusterResponse,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			cluster, _, err := ocmClient.GetCluster(clusterID)
			Expect(cluster).ShouldNot(BeNil())
			Expect(cluster.ID()).To(Equal(clusterID))
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("should return an empty cluster object when clusterID doesn't exist", func() {
			cluster, _, err := ocmClient.GetCluster("a-ghost-cluster-id")
			Expect(cluster).ShouldNot(BeNil())
			Expect(cluster.ID()).Should(BeEmpty())
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
	Context("Upgrade policy", func() {
		It("should not return an error for valid upgrade policy for a given cluster", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", clusterID, upgradePolicyID)),
				RespondWith(
					http.StatusOK,
					upgradePolicy1,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))

			upgradePolicy, _, err := ocmClient.GetUpgradePolicy(clusterID, upgradePolicyID)
			Expect(upgradePolicy).ShouldNot(BeNil())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(upgradePolicy.ClusterID()).To(Equal(clusterID))
			Expect(upgradePolicy.ID()).To(Equal(upgradePolicyID))
		})
		It("should return an error for invalid upgrade policyID for a given cluster", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", clusterID, "not_a_policy_id")),
				RespondWith(
					http.StatusNotFound,
					`"Upgrade policy not found"`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicy, _, err := ocmClient.GetUpgradePolicy(clusterID, "not_a_policy_id")
			Expect(upgradePolicy).Should(BeNil())
			Expect(err).Should(HaveOccurred())

		})
		It("should not return an error when fetching the policy state", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
				RespondWith(
					http.StatusOK,
					upgradePolicyState,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyState, _, err := ocmClient.GetUpgradePolicyState(clusterID, upgradePolicyID)
			Expect(upgradePolicyState).ShouldNot(BeNil())
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("should return an error when fetching the policy state when not found", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
				RespondWith(
					http.StatusNotFound,
					`"Upgrade policy state not found"`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyState, _, err := ocmClient.GetUpgradePolicyState(clusterID, upgradePolicyID)
			Expect(upgradePolicyState).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
		It("should not return an error when fetching upgrade policies", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", clusterID)),
				RespondWith(
					http.StatusOK,
					upgradePoliciesList,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyArray, _, err := ocmClient.GetUpgradePolicies(clusterID)
			Expect(upgradePolicyArray).ShouldNot(BeNil())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(upgradePolicyArray)).To(Equal(2))
		})
		It("should return an error fetching upgrade policies when not found", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", clusterID)),
				RespondWith(
					http.StatusNotFound,
					`"Upgrade policies not found "`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyState, _, err := ocmClient.GetUpgradePolicies(clusterID)
			Expect(upgradePolicyState).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
		It("should not return an error when updating policy state update", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("PATCH", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
				RespondWith(
					http.StatusOK,
					upgradePolicyState,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyStateObj, _, err := ocmClient.UpdateUpgradePolicyState(clusterID, upgradePolicyID, &cmv1.UpgradePolicyState{})
			Expect(upgradePolicyState).ShouldNot(BeNil())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(upgradePolicyStateObj.Value()).To(Equal(cmv1.UpgradePolicyStateValuePending))
		})
		It("should return an error when fetching the policy state when not found", func() {
			mockServer.SetHandler(0, CombineHandlers(
				VerifyRequest("PATCH", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
				RespondWith(
					http.StatusNotFound,
					"Not found",
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			upgradePolicyState, _, err := ocmClient.UpdateUpgradePolicyState(clusterID, upgradePolicyID, &cmv1.UpgradePolicyState{})
			Expect(upgradePolicyState).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
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

	Context("500 Error Handling", func() {
		var (
			mockServer *Server
			ocmClient  OCMClient
		)

		BeforeEach(func() {
			mockServer = NewServer()
			accessToken := MakeTokenString("Bearer", 15*time.Minute)
			ocmConnection, err := sdk.NewConnectionBuilder().
				URL(mockServer.URL()).
				Tokens(accessToken).
				Build()
			Expect(err).NotTo(HaveOccurred())
			ocmClient = NewOcmClient(ocmConnection)
		})

		AfterEach(func() {
			mockServer.Close()
		})

		It("should handle 500 error when getting cluster", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			cluster, _, err := ocmClient.GetCluster(clusterID)
			Expect(err).Should(HaveOccurred())
			Expect(cluster).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when getting upgrade policies", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", clusterID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			upgradePolicies, _, err := ocmClient.GetUpgradePolicies(clusterID)
			Expect(err).Should(HaveOccurred())
			Expect(upgradePolicies).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when getting upgrade policy", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			upgradePolicy, _, err := ocmClient.GetUpgradePolicy(clusterID, upgradePolicyID)
			Expect(err).Should(HaveOccurred())
			Expect(upgradePolicy).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when getting upgrade policy state", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			upgradePolicyState, _, err := ocmClient.GetUpgradePolicyState(clusterID, upgradePolicyID)
			Expect(err).Should(HaveOccurred())
			Expect(upgradePolicyState).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when updating upgrade policy state", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("PATCH", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("PATCH", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("PATCH", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", clusterID, upgradePolicyID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			upgradePolicyState, _, err := ocmClient.UpdateUpgradePolicyState(clusterID, upgradePolicyID, &cmv1.UpgradePolicyState{})
			Expect(err).Should(HaveOccurred())
			Expect(upgradePolicyState).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when sending service log", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("POST", "/api/service_logs/v1/cluster_logs"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("POST", "/api/service_logs/v1/cluster_logs"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("POST", "/api/service_logs/v1/cluster_logs"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			err := ocmClient.SendServiceLog(serviceLog)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when sending limited support", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
					RespondWith(
						http.StatusOK,
						`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
						http.Header{"Content-Type": []string{"application/json"}},
					),
				),
				CombineHandlers(
					VerifyRequest("POST", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("POST", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("POST", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			err := ocmClient.SendLimitedSupport(clusterUUID, limitedSupportReason)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when getting limited support reasons", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
					RespondWith(
						http.StatusOK,
						`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
						http.Header{"Content-Type": []string{"application/json"}},
					),
				),
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons"),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			limitedSupportReasons, err := ocmClient.GetLimitedSupportReasons(clusterUUID)
			Expect(err).Should(HaveOccurred())
			Expect(limitedSupportReasons).Should(BeNil())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})

		It("should handle 500 error when removing limited support", func() {
			mockServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/api/clusters_mgmt/v1/clusters"),
					RespondWith(
						http.StatusOK,
						`{"kind":"ClusterList","page":1,"size":1,"total":1,"items": [{"kind":"Cluster","id":"internal-id"}]}`,
						http.Header{"Content-Type": []string{"application/json"}},
					),
				),
				CombineHandlers(
					VerifyRequest("DELETE", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/%s", limitedSupportReasonID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("DELETE", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/%s", limitedSupportReasonID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
				CombineHandlers(
					VerifyRequest("DELETE", fmt.Sprintf("/api/clusters_mgmt/v1/clusters/internal-id/limited_support_reasons/%s", limitedSupportReasonID)),
					RespondWith(http.StatusInternalServerError, `{"kind": "Error", "reason": "Internal server error"}`, http.Header{"Content-Type": []string{"application/json"}}),
				),
			)
			err := ocmClient.RemoveLimitedSupport(clusterUUID, limitedSupportReasonID)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("500"))
		})
	})
})
