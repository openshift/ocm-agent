package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/onsi/gomega/ghttp"
	"github.com/openshift/ocm-agent/pkg/handlers"
)

// shared variables for handler testing
var apiServer *ghttp.Server
var responseRecorder *httptest.ResponseRecorder
var internalId = "internal-id"
var ocmOperationId = "ocm-operation-id"

func makeOCMRequest(method string, status int, route string, ocmResponse string) {
	var handler http.HandlerFunc
	if status == http.StatusNoContent {
		handler = ghttp.RespondWith(
			status,
			nil,
			http.Header{
				"Content-Type": []string{
					"application/json",
				},
				handlers.OCM_OPERATION_ID_HEADER: []string{
					ocmOperationId,
				},
			},
		)
	} else {
		handler = ghttp.RespondWith(
			status,
			ocmResponse,
			http.Header{
				"Content-Type": []string{
					"application/json",
				},
				handlers.OCM_OPERATION_ID_HEADER: []string{
					ocmOperationId,
				},
			},
		)

	}
	apiServer.RouteToHandler(method, route, handler)
}
