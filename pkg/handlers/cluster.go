package handlers

import (
	"fmt"
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
func (g *ClusterHandler) GetCluster(clusterID string) (*cmv1.Cluster, error) {
	log.Debugf("Sending get cluster object request to OCM API: %s", clusterID)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(clusterID)
	resp, err := request.Get().Send()
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}

// https://api.openshift.com/#/default/patch_api_clusters_mgmt_v1_clusters__cluster_id_
func (g *ClusterHandler) UpdateCluster(clusterID string, cluster *cmv1.Cluster) (*cmv1.Cluster, error) {
	log.Debugf("Sending update cluster object request to OCM API: %s %v", clusterID, *cluster)
	request := g.ocm.ClustersMgmt().V1().Clusters().Cluster(clusterID).Update().Body(cluster)
	resp, err := request.Send()
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}

// Proxies to
// https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id_
// https://api.openshift.com/#/default/patch_api_clusters_mgmt_v1_clusters__cluster_id_
func (g *ClusterHandler) ServeClusterGetUpdate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cluster, err := g.GetCluster(g.clusterId)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		err = cmv1.MarshalCluster(cluster, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
	case "PATCH":
		updatedCluster, err := cmv1.UnmarshalCluster(r.Body)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		cluster, err := g.UpdateCluster(g.clusterId, updatedCluster)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		err = cmv1.MarshalCluster(cluster, w)
		if err != nil {
			errorMessageResponse(err, w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
	default:
		invalidRequestVerbResponse(r.Method, w)
	}
}

func errorMessageResponse(err error, w http.ResponseWriter) {
	log.Error(err)
	http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
}

func invalidRequestVerbResponse(method string, w http.ResponseWriter) {
	log.Errorf("Invalid request verb: %s", method)
	http.Error(w, "Bad request body", http.StatusBadRequest)
}
