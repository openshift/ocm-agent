package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/handlers"
	"github.com/openshift/ocm-agent/pkg/ocm"
)

var getUpgradePolicies = `{
  "items": [
    {
      "kind": "UpgradePolicy",
      "id": "foobar",
      "href": "string",
      "cluster_id": "string",
      "enable_minor_version_upgrades": true,
      "next_run": "2023-10-30T02:37:32.896Z",
      "schedule": "string",
      "schedule_type": "automatic",
      "upgrade_type": "OSD",
      "version": "string"
    }
  ],
  "page": 0,
  "size": 0,
  "total": 0
}`
var getUpgradePolicy = `{
  "kind": "string",
  "id": "string",
  "href": "string",
  "cluster_id": "string",
  "enable_minor_version_upgrades": true,
  "next_run": "2023-11-01T01:57:42.055Z",
  "schedule": "string",
  "schedule_type": "automatic",
  "upgrade_type": "OSD",
  "version": "string"
}`

var getUpgradePolicyState = `{
  "kind": "string",
  "id": "string",
  "href": "string",
  "description": "string",
  "value": "cancelled"
}`

var updateUpgradePolicyState = `{
  "kind": "string",
  "id": "string",
  "href": "string2",
  "description": "string",
  "value": "cancelled"
}`

var upgradePoliciesHandler *handlers.UpgradePoliciesHandler
var upgradePolicyId = "upgrade-policy-id"

var _ = Describe("UpgradePolicies", func() {
	BeforeEach(func() {
		// Inspired by https://github.com/gdbranco/rosa/blob/67f55df2992b596e810942016833893236ef47f1/cmd/upgrade/cluster/cmd_test.go#L23
		apiServer = MakeTCPServer()

		accessToken := MakeTokenString("Bearer", 15*time.Minute)

		sdkclient, _ := sdk.NewConnectionBuilder().
			Logger(nil).
			Tokens(accessToken).
			URL(apiServer.URL()).
			Build()

		upgradePoliciesHandler = handlers.NewUpgradePoliciesHandler(ocm.NewOcmClient(sdkclient), internalId)
		responseRecorder = httptest.NewRecorder()

	})

	AfterEach(func() {
		// Close the servers:
		apiServer.Close()
	})

	It("should list all upgrade policies", func() {
		makeOCMRequest(
			"GET",
			http.StatusOK,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", internalId),
			getUpgradePolicies,
		)
		req := httptest.NewRequest("GET", "/upgrade_policies", nil)

		upgradePoliciesHandler.ServeUpgradePolicyList(responseRecorder, req)
		var policies []cmv1.UpgradePolicy

		_ = json.NewDecoder(responseRecorder.Result().Body).Decode(&policies)
		var ocmResp map[string][]cmv1.UpgradePolicy

		_ = json.Unmarshal([]byte(getUpgradePolicies), &ocmResp)
		items := ocmResp["items"]

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(reflect.DeepEqual(policies, items)).To(BeTrue())
	})
	It("should get an upgrade policy", func() {
		makeOCMRequest(
			"GET",
			http.StatusOK,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", internalId, upgradePolicyId),
			getUpgradePolicy,
		)
		req := httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)

		var policy cmv1.UpgradePolicy

		// nolint
		_ = json.NewDecoder(responseRecorder.Result().Body).Decode(&policy)

		var ocmResp cmv1.UpgradePolicy

		// nolint
		_ = json.Unmarshal([]byte(getUpgradePolicies), &ocmResp)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(reflect.DeepEqual(policy, ocmResp)).To(BeTrue())
	})
	It("should get an upgrade policy state", func() {
		makeOCMRequest(
			"GET",
			http.StatusOK,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			getUpgradePolicyState,
		)
		req := httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		var policyState cmv1.UpgradePolicyState

		// nolint
		_ = json.NewDecoder(responseRecorder.Result().Body).Decode(&policyState)
		var ocmResp cmv1.UpgradePolicyState

		// nolint
		_ = json.Unmarshal([]byte(getUpgradePolicies), &ocmResp)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(reflect.DeepEqual(policyState, ocmResp)).To(BeTrue())
	})

	It("should update an upgrade policy state", func() {
		makeOCMRequest(
			"PATCH",
			http.StatusOK,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			updateUpgradePolicyState,
		)

		reqBody := `{
			"kind": "string",
			"id": "string",
			"href": "string2",
			"description": "string",
			"value": "cancelled"
		}`

		req := httptest.NewRequest("PATCH", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), strings.NewReader(reqBody))
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		var policyState cmv1.UpgradePolicyState

		// nolint
		_ = json.NewDecoder(responseRecorder.Result().Body).Decode(&policyState)
		var ocmResp cmv1.UpgradePolicyState

		// nolint
		_ = json.Unmarshal([]byte(getUpgradePolicies), &ocmResp)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(reflect.DeepEqual(policyState, ocmResp)).To(BeTrue())
	})

	It("should return an error if ocm returns an error for all methods", func() {
		errorMessage := `{"message": "Cannot connect to ocm api"}`
		makeOCMRequest(
			"GET",
			http.StatusBadRequest,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", internalId),
			errorMessage,
		)
		makeOCMRequest(
			"GET",
			http.StatusBadRequest,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", internalId, upgradePolicyId),
			errorMessage,
		)
		makeOCMRequest(
			"GET",
			http.StatusBadRequest,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			errorMessage,
		)
		makeOCMRequest(
			"PATCH",
			http.StatusBadRequest,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			errorMessage,
		)

		req := httptest.NewRequest("GET", "/upgrade_policies", nil)

		upgradePoliciesHandler.ServeUpgradePolicyList(responseRecorder, req)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("PATCH", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("DELETE", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("PATCH", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("should return an error if ocm returns 3xx response for all methods", func() {
		errorMessage := `{"message": "permanently redirected"}`
		makeOCMRequest(
			"GET",
			http.StatusPermanentRedirect,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", internalId),
			errorMessage,
		)
		makeOCMRequest(
			"GET",
			http.StatusPermanentRedirect,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s", internalId, upgradePolicyId),
			errorMessage,
		)
		makeOCMRequest(
			"GET",
			http.StatusPermanentRedirect,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			errorMessage,
		)
		makeOCMRequest(
			"PATCH",
			http.StatusPermanentRedirect,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state", internalId, upgradePolicyId),
			errorMessage,
		)

		req := httptest.NewRequest("GET", "/upgrade_policies", nil)

		upgradePoliciesHandler.ServeUpgradePolicyList(responseRecorder, req)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("PATCH", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("DELETE", fmt.Sprintf("/upgrade_policies/%s", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyGet(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("GET", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), nil)

		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest("PATCH", fmt.Sprintf("/upgrade_policies/%s/state", upgradePolicyId), nil)
		req = mux.SetURLVars(
			req,
			map[string]string{
				consts.UpgradePolicyIdParam: upgradePolicyId,
			},
		)

		upgradePoliciesHandler.ServeUpgradePolicyState(responseRecorder, req)
		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))
	})
})
