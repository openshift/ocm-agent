package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/onsi/gomega/ghttp"
	. "github.com/openshift-online/ocm-sdk-go/testing"
)

// shared variables for handler testing
var apiServer *ghttp.Server
var responseRecorder *httptest.ResponseRecorder
var internalId = "internal-id"

func makeOCMRequest(method string, status int, route string, ocmResponse string) {
	handler := RespondWithJSON(status, ocmResponse)
	if status == http.StatusNoContent {
		handler = ghttp.RespondWith(
			status,
			nil,
			http.Header{
				"Content-Type": []string{
					"application/json",
				},
			},
		)
	}
	apiServer.RouteToHandler(method, route, handler)
}
