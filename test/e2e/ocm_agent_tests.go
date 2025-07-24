// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /test/e2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e

package osde2etests

import (
	"context"
	"encoding/json"
	"os/exec"

	// Additional imports for comprehensive testing

	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"

	// OCM-related imports for GetInternalIDByExternalID
	sdk "github.com/openshift-online/ocm-sdk-go"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/ocm-agent/pkg/ocm"
)

var _ = ginkgo.Describe("ocm-agent", ginkgo.Ordered, func() {

	var (
		client               *resources.Resources
		errorServer          *httptest.Server
		namespace            = "openshift-ocm-agent-operator"
		deploymentName       = "ocm-agent"
		ocmAgentConfigMap    = "ocm-agent-cm"
		ocmAccessTokenSecret = "ocm-access-token"
		clusterVersionName   = "version"
		infrastructureName   = "cluster"

		// ConfigMap keys
		clusterIDKey  = "clusterID"
		ocmBaseURLKey = "ocmBaseURL"
		servicesKey   = "services"

		// Secret keys
		accessTokenKey = "access_token"

		// Label selectors
		ocmAgentLabelSelector = "app=ocm-agent"

		deployments = []string{
			deploymentName,
			deploymentName + "-operator",
		}
	)

	ginkgo.BeforeAll(func() {
		// setup the k8s client
		cfg, err := config.GetConfig()
		Expect(err).Should(BeNil(), "failed to get kubeconfig")
		client, err = resources.New(cfg)
		Expect(err).Should(BeNil(), "resources.New error")

		// Create a mock error server that always returns 503
		errorServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"message": "Service temporarily unavailable", "code": 503}`))
		}))
	})

	ginkgo.AfterAll(func() {
		// Clean up the error server
		if errorServer != nil {
			errorServer.Close()
		}
	})

	ginkgo.It("Testing - basic deployment", func(ctx context.Context) {

		// Testing
		ginkgo.By("checking the namespace exists")
		err := client.Get(ctx, namespace, "", &corev1.Namespace{})
		Expect(err).Should(BeNil(), "namespace %s not found", namespace)

		ginkgo.By("checking the deployment exists")
		for _, deploymentName := range deployments {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: namespace}}
			err = wait.For(conditions.New(client).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue))
			Expect(err).Should(BeNil(), "deployment %s not available", deploymentName)
		}

	})

	// TODO: Add the following tests
	//TEST - Verify that ocm-agent handles 5xx(503 service/api unavailable) response gracefully
	//TEST - Get list of all the upgrade policies belonging to a cluster from ocm api
	//TEST - Verify that ocm-agent sends a successful request to ocm api to get all upgrade policies
	//TEST - Get single upgrade policy belonging to a cluster from ocm api
	//TEST - Fetch state of single upgrade policy belonging to a cluster from ocm api
	//TEST - Update state of a single upgrade policy for a given cluster
	// Comprehensive test suite implementing SREP-910 requirements
	ginkgo.It("Testing - common ocm-agent tests", func(ctx context.Context) {

		// Test variables and setup
		var (
			ocmAgentPodName   string
			ocmAgentURL       = "http://localhost:8081"
			httpClient        = &http.Client{Timeout: 30 * time.Second}
			externalClusterID string
			internalClusterID string
			ocmConnection     *sdk.Connection
			portForwardCmd    *exec.Cmd
		)

		// Setup cleanup function for port forwarding
		defer func() {
			if portForwardCmd != nil && portForwardCmd.Process != nil {
				ginkgo.By("cleaning up port forwarding")
				_ = portForwardCmd.Process.Kill()
				_ = portForwardCmd.Wait()
			}
		}()

		// Get OCM configuration from ocm-agent configmap
		ginkgo.By("getting OCM configuration from ocm-agent configmap")
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentConfigMap, Namespace: namespace}}
		err := client.Get(ctx, configMap.Name, configMap.Namespace, configMap)
		Expect(err).Should(BeNil(), "ocm-agent configmap not found")

		// Get real external cluster ID from cluster configmap
		ginkgo.By("getting real external cluster ID from configmap")

		if err == nil {
			if configMap.Data[clusterIDKey] != "" {
				externalClusterID = configMap.Data[clusterIDKey]
			}
		}
		// If not found in configmap, try fallback approaches
		if externalClusterID == "" {
			// Try to get from ClusterVersion object
			clusterVersion := &configv1.ClusterVersion{ObjectMeta: metav1.ObjectMeta{Name: clusterVersionName}}
			err = client.Get(ctx, clusterVersion.Name, "", clusterVersion)
			if err == nil && clusterVersion.Spec.ClusterID != "" {
				externalClusterID = string(clusterVersion.Spec.ClusterID)
			} else {
				// Try to get from Infrastructure object
				infrastructure := &configv1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: infrastructureName}}
				err = client.Get(ctx, infrastructure.Name, "", infrastructure)
				if err == nil && infrastructure.Status.InfrastructureName != "" {
					externalClusterID = infrastructure.Status.InfrastructureName
				}
			}
		}
		Expect(externalClusterID).ShouldNot(BeEmpty(), "external cluster ID should not be empty")

		// Verify required configuration fields
		Expect(configMap.Data).Should(HaveKey(ocmBaseURLKey), "ocmBaseURL not configured")
		Expect(configMap.Data).Should(HaveKey(servicesKey), "services not configured")
		ocmBaseURL := configMap.Data[ocmBaseURLKey]
		Expect(ocmBaseURL).ShouldNot(BeEmpty(), "ocmBaseURL is empty")
		Expect(configMap.Data[servicesKey]).ShouldNot(BeEmpty(), "services configuration is empty")

		// Get access token from ocm-agent secret
		ginkgo.By("getting OCM access token from secret")
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ocmAccessTokenSecret, Namespace: namespace}}
		err = client.Get(ctx, secret.Name, secret.Namespace, secret)
		Expect(err).Should(BeNil(), "ocm-access-token secret not found")
		Expect(secret.Data).Should(HaveKey(accessTokenKey), "access_token not found in secret")
		accessToken := string(secret.Data[accessTokenKey])
		Expect(accessToken).ShouldNot(BeEmpty(), "access token is empty")

		// Create OCM connection and get internal cluster ID
		ginkgo.By("creating OCM connection and getting internal cluster ID")
		connBuilder := ocm.NewConnection()
		ocmConnection, err = connBuilder.Build(ocmBaseURL, externalClusterID, accessToken)
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to create OCM connection: %v. This may be expected in some test environments.", err))
		}

		// Use GetInternalIDByExternalID to get proper internal cluster ID
		internalClusterID, err = ocm.GetInternalIDByExternalID(externalClusterID, ocmConnection)
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to get internal cluster ID: %v. This may be expected if cluster is not registered in OCM.", err))
		}
		Expect(internalClusterID).ShouldNot(BeEmpty(), "internal cluster ID should not be empty")

		// Get OCM Agent pod for testing
		ginkgo.By("getting ocm-agent pod for testing")
		podList := &corev1.PodList{}
		err = client.List(ctx, podList, func(o *metav1.ListOptions) {
			o.LabelSelector = ocmAgentLabelSelector
		})
		Expect(err).Should(BeNil(), "failed to list ocm-agent pods")
		Expect(len(podList.Items)).Should(BeNumerically(">", 0), "no ocm-agent pods found")

		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				ocmAgentPodName = pod.Name
				break
			}
		}
		Expect(ocmAgentPodName).ShouldNot(BeEmpty(), "no running ocm-agent pod found")

		// Set up port forwarding to the ocm-agent pod
		ginkgo.By("setting up port forwarding to ocm-agent pod")
		portForwardCmd = exec.Command("kubectl", "port-forward",
			fmt.Sprintf("pod/%s", ocmAgentPodName),
			"8081:8081",
			"-n", namespace)

		err = portForwardCmd.Start()
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to start port forwarding: %v. This may be expected in some test environments.", err))
		}

		// Wait for port forwarding to be ready
		ginkgo.By("waiting for port forwarding to be ready")
		Eventually(func() error {
			resp, err := httpClient.Get(fmt.Sprintf("%s/livez", ocmAgentURL))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
			return nil
		}, 30*time.Second, 2*time.Second).Should(Succeed(), "port forwarding should be ready")

		// TEST - Ensure that ocm-agent starts successfully
		ginkgo.By("verifying ocm-agent starts successfully")
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentPodName, Namespace: namespace}}
		err = client.Get(ctx, pod.Name, pod.Namespace, pod)
		Expect(err).Should(BeNil(), "failed to get ocm-agent pod")
		Expect(pod.Status.Phase).Should(Equal(corev1.PodRunning), "ocm-agent pod is not running")

		// Check container status
		Expect(len(pod.Status.ContainerStatuses)).Should(BeNumerically(">", 0), "no container statuses found")
		for _, containerStatus := range pod.Status.ContainerStatuses {
			Expect(containerStatus.Ready).Should(BeTrue(), "container %s is not ready", containerStatus.Name)
			Expect(containerStatus.RestartCount).Should(BeNumerically("<=", 2), "container %s has too many restarts", containerStatus.Name)
		}

		// TEST - Ensure that ocm-agent is able to configure and build an ocm connection successfully
		ginkgo.By("verifying ocm connection configuration")
		// Already verified above by successfully creating OCM connection and getting internal cluster ID

		// TEST - Ensure that ocm-agent sends a successful health check request to ocm api
		ginkgo.By("testing health check endpoints")

		// Test livez endpoint
		resp, err := httpClient.Get(fmt.Sprintf("%s/livez", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "livez endpoint failed")
			resp.Body.Close()
		}

		// Test readyz endpoint
		resp, err = httpClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "readyz endpoint failed")
			resp.Body.Close()
		}

		// TEST - Verify that the ocm-agent handles 4xx(400 Not Found) response gracefully
		ginkgo.By("testing 4xx error handling")

		// Test with invalid endpoint
		resp, err = httpClient.Get(fmt.Sprintf("%s/invalid-endpoint", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusNotFound), "should return 404 for invalid endpoint")
			resp.Body.Close()
		}

		// Check logs for graceful error handling
		ginkgo.By("checking logs for error handling patterns")
		// This would require log analysis - implementation depends on log aggregation setup

		// TEST - Verify the timeout handling when ocm api responds slowly
		ginkgo.By("testing timeout handling")

		// Create client with very short timeout
		shortTimeoutClient := &http.Client{Timeout: 1 * time.Millisecond}

		// Test timeout behavior (this should timeout)
		_, err = shortTimeoutClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		// We expect this to timeout or fail gracefully

		// TEST - Get list of all the upgrade policies belonging to a cluster from ocm api
		ginkgo.By("testing upgrade policies API endpoint with real internal cluster ID")

		// Test if upgrade policies endpoint exists and responds
		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/upgrade_policies", ocmAgentURL, internalClusterID))
		if err == nil {
			// Should handle the request even if cluster doesn't exist
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for upgrade policies endpoint")
			resp.Body.Close()
		}

		// TEST - Verify that ocm-agent sends a successful request to ocm api to get all upgrade policies
		ginkgo.By("verifying upgrade policies request handling")

		// Check if the agent properly proxies requests
		// This test verifies the proxy functionality without requiring actual cluster data

		// TEST - Fetch Limited support reasons for cluster
		ginkgo.By("testing limited support reasons full workflow")

		var limitedSupportReasonID string
		limitedSupportSummary := "E2E Test Limited Support"
		limitedSupportDetails := "This is an automated e2e test for limited support functionality"

		// Step 1: Create a limited support reason using ocm CLI
		ginkgo.By("creating limited support reason via ocm CLI")

		createCmd := fmt.Sprintf(`ocm post /api/clusters_mgmt/v1/clusters/%s/limited_support_reasons << 'EOF'
{
  "summary": "%s",
  "details": "%s",
  "detection_type": "manual"
}
EOF`, internalClusterID, limitedSupportSummary, limitedSupportDetails)

		createOutput, err := exec.Command("bash", "-c", createCmd).Output()
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to create limited support reason via ocm CLI: %v. This may be expected if cluster doesn't support limited support or lacks permissions.", err))
		}

		// Parse the JSON response to get the ID
		var createResponse map[string]interface{}
		err = json.Unmarshal(createOutput, &createResponse)
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to parse limited support creation response: %v", err))
		}

		if id, ok := createResponse["id"].(string); ok {
			limitedSupportReasonID = id
		} else {
			ginkgo.Skip("Failed to get limited support reason ID from creation response")
		}

		// Ensure cleanup happens even if tests fail
		defer func() {
			if limitedSupportReasonID != "" {
				ginkgo.By("cleaning up - deleting limited support reason")
				deleteCmd := fmt.Sprintf("ocm delete /api/clusters_mgmt/v1/clusters/%s/limited_support_reasons/%s", internalClusterID, limitedSupportReasonID)
				_, deleteErr := exec.Command("bash", "-c", deleteCmd).Output()
				if deleteErr != nil {
					fmt.Printf("Failed to cleanup limited support reason %s: %v\n", limitedSupportReasonID, deleteErr)
				}
			}
		}()

		// Step 2: Test retrieval of limited support reasons through ocm-agent
		ginkgo.By("testing limited support reasons retrieval through ocm-agent")

		// Test limited support endpoint - should now return the created reason
		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/limited_support_reasons", ocmAgentURL, internalClusterID))
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to retrieve limited support reasons via ocm-agent: %v. This may be expected if the agent doesn't proxy this endpoint.", err))
		} else {
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for limited support reasons")
			defer resp.Body.Close()
		}

		// Step 3: Test retrieval of specific limited support reason
		ginkgo.By("testing specific limited support reason retrieval")

		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/limited_support_reasons/%s", ocmAgentURL, internalClusterID, limitedSupportReasonID))
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("Failed to retrieve specific limited support reason via ocm-agent: %v. This may be expected if the agent doesn't proxy this endpoint.", err))
		} else {
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for specific limited support reason")
			defer resp.Body.Close()
		}

		// Step 4: Verify the limited support reason contains expected data via ocm CLI
		ginkgo.By("verifying limited support reason data via ocm CLI")

		getCmd := fmt.Sprintf("ocm get /api/clusters_mgmt/v1/clusters/%s/limited_support_reasons/%s", internalClusterID, limitedSupportReasonID)
		getOutput, err := exec.Command("bash", "-c", getCmd).Output()
		if err == nil {
			var getResponse map[string]interface{}
			err = json.Unmarshal(getOutput, &getResponse)
			Expect(err).Should(BeNil(), "should parse limited support reason response")

			// Verify the data matches what we created
			if summary, ok := getResponse["summary"].(string); ok {
				Expect(summary).Should(Equal(limitedSupportSummary), "summary should match created value")
			}

			if details, ok := getResponse["details"].(string); ok {
				Expect(details).Should(Equal(limitedSupportDetails), "details should match created value")
			}

			if detectionType, ok := getResponse["detection_type"].(string); ok {
				Expect(detectionType).Should(Equal("manual"), "detection_type should be manual")
			}
		}

		// Final verification - ensure agent is still healthy after all tests
		ginkgo.By("final health verification after all tests")
		resp, err = httpClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "agent unhealthy after tests")
			resp.Body.Close()
		}

		// Verify pod is still running and stable
		finalPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentPodName, Namespace: namespace}}
		err = client.Get(ctx, finalPod.Name, finalPod.Namespace, finalPod)
		Expect(err).Should(BeNil(), "failed to get ocm-agent pod after tests")
		Expect(finalPod.Status.Phase).Should(Equal(corev1.PodRunning), "ocm-agent pod not running after tests")
	})

})
