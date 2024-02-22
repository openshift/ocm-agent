package handlers

import (
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/ocm-agent/pkg/ocm"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type ClusterHandler struct {
	ocm       ocm.OCMClient
	clusterId string
}

// For /api/clusters_mgmt/v1/clusters/{cluster_id}
// https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id_
func NewClusterHandler(o ocm.OCMClient, clusterId string) *ClusterHandler {
	log.Debug("Creating new cluster object Handler")
	return &ClusterHandler{
		ocm:       o,
		clusterId: clusterId,
	}
}

// Proxies to
// https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id_
func (g *ClusterHandler) ServeClusterGet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cluster, operationIdHeader, err := g.ocm.GetCluster(g.clusterId)

		w.Header().Set(OCM_OPERATION_ID_HEADER, operationIdHeader)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = cmv1.MarshalCluster(cluster, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}
	default:
		invalidRequestVerbResponse(r.Method, w)
	}
}
