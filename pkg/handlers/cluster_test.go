package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	"github.com/openshift/ocm-agent/pkg/handlers"
	"github.com/openshift/ocm-agent/pkg/ocm"
)

var getCluster = `{
	"kind": "Cluster",
	"id": "string",
	"href": "string",
	"name": "string",
	"external_id": "string",
	"infra_id": "string",
	"display_name": "",
	"creation_timestamp": "2023-11-19T07:52:20.413713Z",
	"activity_timestamp": "2023-11-20T10:17:18Z",
	"expiration_timestamp": "2023-11-20T19:52:18.077225Z",
	"cloud_provider": {
	  "kind": "CloudProviderLink",
	  "id": "aws",
	  "href": "/api/clusters_mgmt/v1/cloud_providers/aws"
	},
	"openshift_version": "4.14.1",
	"subscription": {
	  "kind": "SubscriptionLink",
	  "id": "subid",
	  "href": "/api/accounts_mgmt/v1/subscriptions/subid"
	},
	"region": {
	  "kind": "CloudRegionLink",
	  "id": "us-west-2",
	  "href": "/api/clusters_mgmt/v1/cloud_providers/aws/regions/us-west-2"
	},
	"console": {
	  "url": "string"
	},
	"api": {
	  "url": "string",
	  "listening": "external"
	},
	"nodes": {
	  "master": 3,
	  "infra": 2,
	  "compute": 2,
	  "availability_zones": [
		 "us-west-2a"
	  ],
	  "compute_machine_type": {
		"kind": "MachineTypeLink",
		"id": "m5.xlarge",
		"href": "/api/clusters_mgmt/v1/machine_types/m5.xlarge"
	  },
	  "infra_machine_type": {
		"kind": "MachineTypeLink",
		"id": "r5.xlarge",
		"href": "/api/clusters_mgmt/v1/machine_types/r5.xlarge"
	  }
	},
	"state": "ready",
	"flavour": {
	  "kind": "FlavourLink",
	  "id": "osd-4",
	  "href": "/api/clusters_mgmt/v1/flavours/osd-4"
	},
	"groups": {
	  "kind": "GroupListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/groups"
	},
	"aws": {
	  "private_link": false,
	  "private_link_configuration": {
		"kind": "PrivateLinkConfigurationLink",
		"href": "/api/clusters_mgmt/v1/clusters/clusterid/aws/private_link_configuration"
	  },
	  "tags": {
		"red-hat-clustertype": "osd",
		"red-hat-managed": "true"
	  },
	  "audit_log": {
		"role_arn": ""
	  },
	  "ec2_metadata_http_tokens": "optional"
	},
	"dns": {
	  "base_domain": "string"
	},
	"network": {
	  "type": "OVNKubernetes",
	  "machine_cidr": "10.0.0.0/16",
	  "service_cidr": "172.30.0.0/16",
	  "pod_cidr": "10.128.0.0/16",
	  "host_prefix": 23
	},
	"external_configuration": {
	  "kind": "ExternalConfiguration",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/external_configuration",
	  "syncsets": {
		"kind": "SyncsetListLink",
		"href": "/api/clusters_mgmt/v1/clusters/clusterid/external_configuration/syncsets"
	  },
	  "labels": {
		"kind": "LabelListLink",
		"href": "/api/clusters_mgmt/v1/clusters/clusterid/external_configuration/labels"
	  },
	  "manifests": {
		"kind": "ManifestListLink",
		"href": "/api/clusters_mgmt/v1/clusters/clusterid/external_configuration/manifests"
	  }
	},
	"multi_az": false,
	"managed": true,
	"ccs": {
	  "enabled": true,
	  "disable_scp_checks": false
	},
	"version": {
	  "kind": "Version",
	  "id": "openshift-v4.14.1",
	  "href": "/api/clusters_mgmt/v1/versions/openshift-v4.14.1",
	  "raw_id": "4.14.1",
	  "channel_group": "stable",
	  "end_of_life_timestamp": "2025-02-28T00:00:00Z"
	},
	"identity_providers": {
	  "kind": "IdentityProviderListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/identity_providers"
	},
	"aws_infrastructure_access_role_grants": {
	  "kind": "AWSInfrastructureAccessRoleGrantLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/aws_infrastructure_access_role_grants"
	},
	"addons": {
	  "kind": "AddOnInstallationListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/addons"
	},
	"ingresses": {
	  "kind": "IngressListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/ingresses"
	},
	"machine_pools": {
	  "kind": "MachinePoolListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/machine_pools"
	},
	"inflight_checks": {
	  "kind": "InflightCheckListLink",
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/inflight_checks"
	},
	"product": {
	  "kind": "ProductLink",
	  "id": "osd",
	  "href": "/api/clusters_mgmt/v1/products/osd"
	},
	"status": {
	  "state": "ready",
	  "dns_ready": true,
	  "oidc_ready": false,
	  "provision_error_message": "",
	  "provision_error_code": "",
	  "configuration_mode": "full",
	  "limited_support_reason_count": 0
	},
	"node_drain_grace_period": {
	  "value": 60,
	  "unit": "minutes"
	},
	"etcd_encryption": false,
	"billing_model": "standard",
	"disable_user_workload_monitoring": true,
	"managed_service": {
	  "enabled": false,
	  "managed": false
	},
	"hypershift": {
	  "enabled": false
	},
	"byo_oidc": {
	  "enabled": false
	},
	"delete_protection": {
	  "href": "/api/clusters_mgmt/v1/clusters/clusterid/delete_protection",
	  "enabled": false
	}
  }`

var clusterHandler *handlers.ClusterHandler

var _ = Describe("ClusterHandler", func() {
	BeforeEach(func() {
		// Inspired by https://github.com/gdbranco/rosa/blob/67f55df2992b596e810942016833893236ef47f1/cmd/upgrade/cluster/cmd_test.go#L23
		apiServer = MakeTCPServer()

		accessToken := MakeTokenString("Bearer", 15*time.Minute)

		sdkclient, _ := sdk.NewConnectionBuilder().
			Logger(nil).
			Tokens(accessToken).
			URL(apiServer.URL()).
			Build()

		clusterHandler = handlers.NewClusterHandler(ocm.NewOcmClient(sdkclient), internalId)
		responseRecorder = httptest.NewRecorder()

	})

	AfterEach(func() {
		// Close the servers:
		apiServer.Close()
	})

	It("should get a cluster object", func() {
		makeOCMRequest(
			"GET",
			http.StatusOK,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", internalId),
			getCluster,
		)
		req := httptest.NewRequest("GET", "/cluster", nil)

		req = mux.SetURLVars(
			req,
			map[string]string{},
		)

		clusterHandler.ServeClusterGet(responseRecorder, req)

		var cluster cmv1.Cluster

		// nolint
		_ = json.NewDecoder(responseRecorder.Result().Body).Decode(&cluster)

		var ocmResp cmv1.Cluster

		// nolint
		_ = json.Unmarshal([]byte(getCluster), &ocmResp)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(reflect.DeepEqual(cluster, ocmResp)).To(BeTrue())
	})

	It("should return an error if ocm returns an error", func() {
		errorMessage := `{"message": "Cannot connect to ocm api"}`
		makeOCMRequest(
			"GET",
			http.StatusBadRequest,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", internalId),
			errorMessage,
		)

		req := httptest.NewRequest("GET", "/cluster", nil)

		clusterHandler.ServeClusterGet(responseRecorder, req)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("should return an error if ocm returns a 3xx response", func() {
		errorMessage := `{"message": "permanently redirected"}`
		makeOCMRequest(
			"GET",
			http.StatusPermanentRedirect,
			fmt.Sprintf("/api/clusters_mgmt/v1/clusters/%s", internalId),
			errorMessage,
		)

		req := httptest.NewRequest("GET", "/cluster", nil)

		clusterHandler.ServeClusterGet(responseRecorder, req)

		Expect(reflect.DeepEqual(ocmOperationId, responseRecorder.Header().Get(handlers.OCM_OPERATION_ID_HEADER))).To(BeTrue())
		Expect(responseRecorder.Result().StatusCode).To(Equal(http.StatusBadRequest))
	})
})
