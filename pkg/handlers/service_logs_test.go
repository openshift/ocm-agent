package handlers_test

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	sdk "github.com/openshift-online/ocm-sdk-go"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	"github.com/openshift/ocm-agent/pkg/handlers"
)

func TestOCMClient(t *testing.T) {
	RegisterFailHandler(Fail)
}

var _ = Describe("ServiceLogsHandler", func() {
	var (
		ocmConnection *sdk.Connection
		ocmClient     handlers.OCMClient
		serviceLog    *handlers.ServiceLog
		err           error
		mockServer    *Server
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

		ocmClient = handlers.NewOcmClient(ocmConnection)
		serviceLog = testconst.NewTestServiceLog(
			handlers.ServiceLogActivePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogActiveDesc,
			testconst.TestHostedClusterID,
			testconst.TestNotification.Severity,
			testconst.TestNotification.LogType,
			testconst.TestNotification.References,
		)

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
})
