package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/ocm-agent/pkg/consts"
	log "github.com/sirupsen/logrus"
)

// UpgradePoliciesHandler represents a request or requests to the upgrade policies endpoint set
// in OCM.
type UpgradePoliciesHandler struct {
	ocm             *sdk.Connection
	clusterID       string
	upgradePolicyID string
}

// Creates a new UpgradePoliciesHandler instance.
func NewUpgradePoliciesHandler(o *sdk.Connection, clusterId string) *UpgradePoliciesHandler {
	log.Debug("Creating new upgrade policies Handler")
	return &UpgradePoliciesHandler{
		ocm:       o,
		clusterID: clusterId,
	}
}

// GetUpgradePolicies gets all of the upgrade policies belonging to a cluster from OCM.
// It does not paginate, and sends the whole list as a single list.
// Proxies to https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies
func (g *UpgradePoliciesHandler) GetUpgradePolicies() ([]*cmv1.UpgradePolicy, string, error) {
	var upgradePolicies []*cmv1.UpgradePolicy
	var operationIdHeader string

	log.Debugf("Sending get all upgrade polices request to OCM API: %s", g.clusterID)
	collection := g.ocm.ClustersMgmt().V1().Clusters().Cluster(g.clusterID).UpgradePolicies()
	page := consts.OCMListRequestStartPage
	size := consts.OCMListRequestMaxPerPage

	for {
		resp, err := collection.List().Send()

		if err != nil {
			return nil, resp.Header().Get(OCM_OPERATION_ID_HEADER), err
		}
		upgradePolicies = append(upgradePolicies, resp.Items().Slice()...)
		operationIdHeader = resp.Header().Get(OCM_OPERATION_ID_HEADER)
		if resp.Size() < size {
			break
		}
		page++
	}

	return upgradePolicies, operationIdHeader, nil
}

// GetUpgradePolicy gets a single upgrade policy from a cluster.
// Proxies to https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id_
func (g *UpgradePoliciesHandler) GetUpgradePolicy() (*cmv1.UpgradePolicy, string, error) {
	log.Debugf("Sending get upgrade policy request to OCM API: %s %s", g.clusterID, g.upgradePolicyID)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(g.clusterID).UpgradePolicies().UpgradePolicy(g.upgradePolicyID)
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OCM_OPERATION_ID_HEADER), err
	}
	return resp.Body(), resp.Header().Get(OCM_OPERATION_ID_HEADER), nil
}

// GetUpgradePolicy gets a single upgrade policy's state from a cluster.
// Proxies to https://api.openshift.com#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id__state
func (g *UpgradePoliciesHandler) GetUpgradePolicyState() (*cmv1.UpgradePolicyState, string, error) {
	log.Debugf("Sending get upgrade policy state request to OCM API: %s", g.clusterID)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(g.clusterID).UpgradePolicies().UpgradePolicy(g.upgradePolicyID).State()
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OCM_OPERATION_ID_HEADER), err
	}
	return resp.Body(), resp.Header().Get(OCM_OPERATION_ID_HEADER), nil
}

// UpdateUpgradePolicyState updates a single upgrade policy's state for a given cluster.
// Proxies to https://api.openshift.com/#/default/patch_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id__state
func (g *UpgradePoliciesHandler) UpdateUpgradePolicyState(policyState *cmv1.UpgradePolicyState) (*cmv1.UpgradePolicyState, string, error) {
	log.Debugf("Sending update upgrade policy state request to OCM API: %s %s", g.clusterID, g.upgradePolicyID)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(g.clusterID).UpgradePolicies().UpgradePolicy(g.upgradePolicyID).State().Update().Body(policyState)
	resp, err := request.Send()
	if err != nil {
		return nil, resp.Header().Get(OCM_OPERATION_ID_HEADER), err
	}
	return resp.Body(), resp.Header().Get(OCM_OPERATION_ID_HEADER), nil
}

// ServeUpgradePolicyList reads and writes raw HTTP requests and proxies them to the 'list' endpoints for upgrade policies
func (g *UpgradePoliciesHandler) ServeUpgradePolicyList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		policies, operationIdHeader, err := g.GetUpgradePolicies()
		w.Header().Set(OCM_OPERATION_ID_HEADER, operationIdHeader)

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
	g.upgradePolicyID = vars[consts.UpgradePolicyIdParam]

	switch r.Method {
	case "GET":
		policy, operationIdHeader, err := g.GetUpgradePolicy()
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
	g.upgradePolicyID = vars[consts.UpgradePolicyIdParam]

	switch r.Method {
	case "GET":
		policyState, operationIdHeader, err := g.GetUpgradePolicyState()

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

		policy, operationIdHeader, err := g.UpdateUpgradePolicyState(updatedPolicyState)
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
