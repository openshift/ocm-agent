package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/ocm"
	log "github.com/sirupsen/logrus"
)

// UpgradePoliciesHandler represents a request or requests to the upgrade policies endpoint set
// in OCM.
type UpgradePoliciesHandler struct {
	ocm       ocm.OCMClient
	clusterID string
}

// Creates a new UpgradePoliciesHandler instance.
func NewUpgradePoliciesHandler(o ocm.OCMClient, clusterId string) *UpgradePoliciesHandler {
	log.Debug("Creating new upgrade policies Handler")
	return &UpgradePoliciesHandler{
		ocm:       o,
		clusterID: clusterId,
	}
}

// ServeUpgradePolicyList reads and writes raw HTTP requests and proxies them to the 'list' endpoints for upgrade policies
func (g *UpgradePoliciesHandler) ServeUpgradePolicyList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		policies, operationIdHeader, err := g.ocm.GetUpgradePolicies(g.clusterID)
		w.Header().Set(ocm.OcmOperationIdHeader, operationIdHeader)

		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = cmv1.MarshalUpgradePolicyList(policies, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}
	default:
		invalidRequestVerbResponse(r.Method, w)
	}
}

// ServeUpgradePolicyGet reads and writes raw HTTP requests and proxies them to the 'get', 'update', and 'delete' endpoints for upgrade policies
func (g *UpgradePoliciesHandler) ServeUpgradePolicyGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	upgradePolicyID := vars[consts.UpgradePolicyIdParam]

	switch r.Method {
	case "GET":
		policy, operationIdHeader, err := g.ocm.GetUpgradePolicy(g.clusterID, upgradePolicyID)
		w.Header().Set(OCM_OPERATION_ID_HEADER, operationIdHeader)

		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = cmv1.MarshalUpgradePolicy(policy, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}
	default:
		invalidRequestVerbResponse(r.Method, w)
	}
}

// ServeUpgradePolicyState reads and writes raw HTTP requests and proxies them to the 'get' and 'update' endpoints for upgrade policy states.
func (g *UpgradePoliciesHandler) ServeUpgradePolicyState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	upgradePolicyID := vars[consts.UpgradePolicyIdParam]

	switch r.Method {
	case "GET":
		policyState, operationIdHeader, err := g.ocm.GetUpgradePolicyState(g.clusterID, upgradePolicyID)

		w.Header().Set(OCM_OPERATION_ID_HEADER, operationIdHeader)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = cmv1.MarshalUpgradePolicyState(policyState, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}
	case "PATCH":
		updatedPolicyState, err := cmv1.UnmarshalUpgradePolicyState(r.Body)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		policy, operationIdHeader, err := g.ocm.UpdateUpgradePolicyState(g.clusterID, upgradePolicyID, updatedPolicyState)
		w.Header().Set(OCM_OPERATION_ID_HEADER, operationIdHeader)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = cmv1.MarshalUpgradePolicyState(policy, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}
	default:
		invalidRequestVerbResponse(r.Method, w)
	}
}
