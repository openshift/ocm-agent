package handlers

import (
	"net/http"

	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	log "github.com/sirupsen/logrus"
)

type ClusterHandler struct {
	ocm       *sdk.Connection
	clusterId string
}

// For /api/clusters_mgmt/v1/clusters/{cluster_id}
// https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id_
func NewClusterHandler(o *sdk.Connection, clusterId string) *ClusterHandler {
	log.Debug("Creating new cluster object Handler")
	return &ClusterHandler{
		ocm:       o,
		clusterId: clusterId,
	}
}

// https://pkg.go.dev/github.com/openshift-online/ocm-sdk-go@v0.1.382/clustersmgmt/v1#Cluster
func (g *ClusterHandler) GetCluster() (*cmv1.Cluster, string, error) {
	log.Debugf("Sending get cluster object request to OCM API: %s", g.clusterId)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(g.clusterId)
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OCM_OPERATION_ID_HEADER), err
	}
	return resp.Body(), resp.Header().Get(OCM_OPERATION_ID_HEADER), nil
}

// Proxies to
// https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id_
func (g *ClusterHandler) ServeClusterGet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cluster, operationIdHeader, err := g.GetCluster()

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
