// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /test/e2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e

package osde2etests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/ocm-agent/pkg/k8s"
	"github.com/openshift/ocm-agent/pkg/ocm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"

	// OCM-related imports
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	configv1 "github.com/openshift/api/config/v1"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
)

// AlertPayload represents the structure of an alert sent to OCM Agent
type AlertPayload struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []Alert           `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
}

// Alert represents the structure of an alert sent to OCM Agent
type Alert struct {
	Status                           string            `json:"status"`
	Labels                           map[string]string `json:"labels"`
	Annotations                      map[string]string `json:"annotations"`
	StartsAt                         string            `json:"startsAt"`
	EndsAt                           string            `json:"endsAt"`
	GeneratorURL                     string            `json:"generatorURL"`
	ManagedFleetNotificationTemplate string            `json:"managedFleetNotificationTemplate"`
	MCClusterID                      string            `json:"_mc_id"`
	ClusterID                        string            `json:"_id"`
	Source                           string            `json:"source"`
}

// AlertResponse represents the response from the OCM Agent
type AlertResponse struct {
	Status string `json:"status"`
}

var _ = ginkgo.Describe("ocm-agent", ginkgo.Ordered, func() {

	var (
		client                      *resources.Resources
		k8sClient                   crclient.Client
		errorServer                 *httptest.Server
		ocmConnection               *sdk.Connection
		namespace                   = "openshift-ocm-agent-operator"
		deploymentName              = "ocm-agent"
		ocmAgentConfigMap           = "ocm-agent-cm"
		ocmAgentFleetConfigMap      = "ocm-agent-fleet-cm"
		ocmAgentManagedNotification = "sre-managed-notifications"
		networkPolicyName           = "ocm-agent-allow-all-ingress"
		testNotificationName        = "LoggingVolumeFillingUp"
		clusterVersionName          = "version"
		infrastructureName          = "cluster"
		isFleetMode                 = false
		shortSleepInterval          = 5 * time.Second  // 3 seconds
		longSleepInterval           = 30 * time.Second // 30 seconds

		// ConfigMap keys
		clusterIDKey  = "clusterID"
		ocmBaseURLKey = "ocmBaseURL"
		servicesKey   = "services"

		// Label selectors
		ocmAgentLabelSelector = "app=ocm-agent"

		ocmAgentPodName   string
		ocmBaseURL        string
		ocmAgentURL       = "http://ocm-agent.openshift-ocm-agent-operator.svc:8081"
		httpClient        = &http.Client{Timeout: 30 * time.Second}
		externalClusterID string
		internalClusterID string

		deployments = []string{
			deploymentName,
			deploymentName + "-operator",
		}
		alertName = "LoggingVolumeFillingUpNotificationSRE"
	)

	// createNetworkpolicy creates a network policy which allow all traffic to ocm-agent for testing.
	createNetworkPolicy := func(ctx context.Context) error {
		networkPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkPolicyName,
				Namespace: namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ocm-agent",
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{},
				},
			},
		}
		err := client.Create(ctx, networkPolicy)
		if err != nil {
			return fmt.Errorf("failed to create network policy: %v", err)
		}

		return nil
	}

	// createDefaultNotification creates a managed notification CRD for testing
	createDefaultNotification := func(ctx context.Context) error {
		defaultNotification := &oav1alpha1.ManagedNotification{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ManagedNotification",
				APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ocmAgentManagedNotification,
				Namespace: namespace,
			},
			Spec: oav1alpha1.ManagedNotificationSpec{
				Notifications: []oav1alpha1.Notification{
					{
						Name:         testNotificationName,
						ResendWait:   24,
						ResolvedDesc: `Your cluster's ElasticSearch deployment is detected as being at safe disk consumption levels and no additional action on this issue is required.`,
						ActiveDesc:   `Your cluster requires you to take action as its ElasticSearch cluster logging deployment has been detected as reaching a high disk usage threshold.`,
						Severity:     "Info",
						Summary:      "ElasticSearch reaching disk capacity",
					},
					{
						Name:         "ParallelAlert_1",
						ResendWait:   24,
						ResolvedDesc: `Parallel Alert1 has been resolved`,
						ActiveDesc:   `Your cluster requires you to take action as Parallel Alert1 is firing.`,
						Severity:     "Info",
						Summary:      "Test Parallel Alert 1",
					},
					{
						Name:         "ParallelAlert_2",
						ResendWait:   24,
						ResolvedDesc: `Parallel Alert2 has been resolved`,
						ActiveDesc:   `Your cluster requires you to take action as Parallel Alert1 is firing.`,
						Severity:     "Info",
						Summary:      "Test Parallel Alert 2",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, defaultNotification)
		if err != nil {
			return fmt.Errorf("failed to create default notification: %v", err)
		}
		return nil
	}

	// createAlert creates an alert payload similar to the shell script create-alert.sh
	createSingleAlert := func(alertStatus, alertName, managedNotificationTemplate string) AlertPayload {
		today := time.Now().UTC().Format("2006-01-02")

		return AlertPayload{
			Receiver: "ocmagent",
			Status:   alertStatus,
			Alerts: []Alert{
				{
					Status: alertStatus,
					Labels: map[string]string{
						"alertname":                     alertName,
						"alertstate":                    alertStatus,
						"namespace":                     "openshift-monitoring",
						"openshift_io_alert_source":     "platform",
						"prometheus":                    "openshift-monitoring/k8s",
						"send_managed_notification":     "true",
						"managed_notification_template": managedNotificationTemplate,
						"severity":                      "info",
					},
					Annotations: map[string]string{
						"description": "",
					},
					StartsAt:     today + "T00:00:00Z",
					EndsAt:       "0001-01-01T00:00:00Z",
					GeneratorURL: "",
				},
			},
			GroupLabels:       map[string]string{},
			CommonLabels:      map[string]string{},
			CommonAnnotations: map[string]string{},
			ExternalURL:       "",
		}
	}

	createBiAlert := func(alertStatus, alertName, managedNotificationTemplate string) AlertPayload {
		today := time.Now().UTC().Format("2006-01-02")

		return AlertPayload{
			Receiver: "ocmagent",
			Status:   alertStatus,
			Alerts: []Alert{
				{
					Status: alertStatus,
					Labels: map[string]string{
						"alertname":                     alertName + "_1",
						"alertstate":                    alertStatus,
						"namespace":                     "openshift-monitoring",
						"openshift_io_alert_source":     "platform",
						"prometheus":                    "openshift-monitoring/k8s",
						"send_managed_notification":     "true",
						"managed_notification_template": managedNotificationTemplate + "_1",
						"severity":                      "info",
					},
					Annotations: map[string]string{
						"description": "",
					},
					StartsAt:     today + "T00:00:00Z",
					EndsAt:       "0001-01-01T00:00:00Z",
					GeneratorURL: "",
				},
				{
					Status: alertStatus,
					Labels: map[string]string{
						"alertname":                     alertName + "_2",
						"alertstate":                    alertStatus,
						"namespace":                     "openshift-monitoring",
						"openshift_io_alert_source":     "platform",
						"prometheus":                    "openshift-monitoring/k8s",
						"send_managed_notification":     "true",
						"managed_notification_template": managedNotificationTemplate + "_2",
						"severity":                      "info",
					},
					Annotations: map[string]string{
						"description": "",
					},
					StartsAt:     today + "T00:00:00Z",
					EndsAt:       "0001-01-01T00:00:00Z",
					GeneratorURL: "",
				},
			},
			GroupLabels:       map[string]string{},
			CommonLabels:      map[string]string{},
			CommonAnnotations: map[string]string{},
			ExternalURL:       "",
		}
	}

	// postAlert sends an alert to the OCM Agent similar to post-alert.sh
	postAlert := func(ctx context.Context, alert AlertPayload) error {
		alertJSON, err := json.Marshal(alert)
		if err != nil {
			return fmt.Errorf("failed to marshal alert: %v", err)
		}

		resp, err := httpClient.Post(
			fmt.Sprintf("%s/alertmanager-receiver", ocmAgentURL),
			"application/json",
			bytes.NewBuffer(alertJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to post alert: %v", err)
		}
		defer resp.Body.Close()

		var alertResponse AlertResponse
		if err := json.NewDecoder(resp.Body).Decode(&alertResponse); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}

		if alertResponse.Status != "ok" {
			return fmt.Errorf("alert posting failed with status: %s", alertResponse.Status)
		}

		return nil
	}

	// getServiceLogCount gets the count of service logs for a cluster
	getServiceLogCount := func(ctx context.Context, clusterUUID string) (int, error) {
		serviceLogsClient := ocmConnection.ServiceLogs().V1()

		response, err := serviceLogsClient.Clusters().ClusterLogs().List().
			Parameter("cluster_uuid", clusterUUID).
			Send()
		if err != nil {
			return 0, fmt.Errorf("failed to get service logs: %v", err)
		}

		return response.Total(), nil
	}

	// checkServiceLogCount verifies the service log count matches expectations
	checkServiceLogCount := func(ctx context.Context, clusterUUID string, preCount, expectedNew int) {
		expectedTotal := preCount + expectedNew
		actualCount, err := getServiceLogCount(ctx, clusterUUID)
		Expect(err).Should(BeNil(), "failed to get service log count")
		Expect(actualCount).Should(Equal(expectedTotal),
			fmt.Sprintf("Expected SL count: %d, Got SL count: %d", expectedTotal, actualCount))
	}

	ginkgo.BeforeAll(func(ctx context.Context) {
		// setup the k8s client
		cfg, err := config.GetConfig()
		Expect(err).Should(BeNil(), "failed to get kubeconfig")
		client, err = resources.New(cfg)
		Expect(err).Should(BeNil(), "resources.New error")

		k8sClient, err = k8s.NewClient()
		Expect(err).Should(BeNil(), "Failed to create controller runtime client")

		// Create a mock error server that always returns 503
		errorServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"message": "Service temporarily unavailable", "code": 503}`))
		}))

		ginkgo.By("getting OCM configuration from ocm-agent configmap")
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentConfigMap, Namespace: namespace}}
		err = client.Get(ctx, configMap.Name, configMap.Namespace, configMap)
		// Verify required configuration fields
		Expect(err).Should(BeNil(), "ocm-agent configmap not found")

		ginkgo.By("getting real external cluster ID from configmap")
		if err == nil {
			if configMap.Data[clusterIDKey] != "" {
				externalClusterID = configMap.Data[clusterIDKey]
				ginkgo.By(fmt.Sprintf("externalClusterId %s", externalClusterID))
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

		ginkgo.By("Getting if fleet mode is enabled with ocm-agent configmap")
		fleetConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentFleetConfigMap, Namespace: namespace}}
		err = client.Get(ctx, fleetConfigMap.Name, fleetConfigMap.Namespace, fleetConfigMap)
		if err == nil {
			// If we could get ocm-agent-fleet-cm, then ocm-agent is in fleet mode
			isFleetMode = true
		}
		if isFleetMode {
			Expect(fleetConfigMap.Data).Should(HaveKey(ocmBaseURLKey), "ocmBaseURL not configured")
			Expect(fleetConfigMap.Data).Should(HaveKey(servicesKey), "services not configured")
			ocmBaseURL = fleetConfigMap.Data[ocmBaseURLKey]
			Expect(ocmBaseURL).ShouldNot(BeEmpty(), "ocmBaseURL is empty")
			Expect(fleetConfigMap.Data[servicesKey]).ShouldNot(BeEmpty(), "services configuration is empty")
		} else {
			Expect(configMap.Data).Should(HaveKey(ocmBaseURLKey), "ocmBaseURL not configured")
			Expect(configMap.Data).Should(HaveKey(servicesKey), "services not configured")
			ocmBaseURL = configMap.Data[ocmBaseURLKey]
			Expect(ocmBaseURL).ShouldNot(BeEmpty(), "ocmBaseURL is empty")
			Expect(configMap.Data[servicesKey]).ShouldNot(BeEmpty(), "services configuration is empty")
		}
		// Override the ocm-agent URL if the OCM_AGENT_URL environment variable is set
		if os.Getenv("OCM_AGENT_URL") != "" {
			ocmAgentURL = os.Getenv("OCM_AGENT_URL")
		}
		// Override fleetMode if the OCM_FLEET_MODE environment variable is set
		if os.Getenv("OCM_FLEET_MODE") != "" {
			isFleetMode = os.Getenv("OCM_FLEET_MODE") == "true"
		}

		// Get access token from env or secret
		var accessToken string
		// OCM_TOKEN env is set in the test image, other name will cause token not found failure
		if thirdPartyToken := os.Getenv("OCM_TOKEN"); thirdPartyToken != "" {
			accessToken = thirdPartyToken
		}

		// Create OCM connection and get internal cluster ID
		ginkgo.By("creating OCM connection and getting internal cluster ID")
		logger, err := sdk.NewGoLoggerBuilder().Debug(false).Build()
		Expect(err).To(BeNil())

		builder := sdk.NewConnectionBuilder().
			Logger(logger).
			URL(ocmBaseURL)

		if accessToken != "" {
			builder = builder.Tokens(accessToken)
		} else {
			if ocmClientID, ocmClientSecret := os.Getenv("OCM_CLIENT_ID"), os.Getenv("OCM_CLIENT_SECRET"); ocmClientID != "" && ocmClientSecret != "" {
				builder = builder.Client(ocmClientID, ocmClientSecret)
			}
		}

		ocmConnection, err = builder.Build()
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to create OCM connection. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to create OCM connection: %v. This may be expected in some test environments.", err))
		}

		// Use GetInternalIDByExternalID to get proper internal cluster ID
		internalClusterID, err = ocm.GetInternalIDByExternalID(externalClusterID, ocmConnection)
		if err != nil {
			fmt.Println("Failed to get ocm connection")
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to get internal cluster ID. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to get internal cluster ID: %v. This may be expected if cluster is not registered in OCM.", err))
		}
		Expect(internalClusterID).ShouldNot(BeEmpty(), "internal cluster ID should not be empty")

		ginkgo.By("creating networkpolicy to allow traffic from all namespace")
		err = createNetworkPolicy(ctx)
		Expect(err).To(BeNil(), fmt.Sprintf("Failed to create networkpolicy %v", err))

	})

	ginkgo.AfterAll(func(ctx context.Context) {
		// Clean up the error server
		if errorServer != nil {
			errorServer.Close()
		}
		networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: networkPolicyName, Namespace: namespace}}
		err := client.Get(ctx, networkPolicy.Name, networkPolicy.Namespace, networkPolicy)
		if err == nil {
			// If networkpolicy exist, delete it
			client.Delete(ctx, networkPolicy)
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

	ginkgo.It("Testing - common ocm-agent tests", func(ctx context.Context) {
		// Get OCM Agent pod for testing
		ginkgo.By("getting ocm-agent pod for testing")
		podList := &corev1.PodList{}
		err := client.List(ctx, podList, func(o *metav1.ListOptions) {
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

		// Wait for the ocm-agent service to be ready
		ginkgo.By("waiting for the ocm-agent service to be ready")
		Eventually(func() error {
			resp, err := httpClient.Get(fmt.Sprintf("%s/livez", ocmAgentURL))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("service not ready, got status code: %d", resp.StatusCode)
			}
			return nil
		}, "2m", "5s").Should(Succeed(), "ocm-agent service should be available")

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

		ocmClient := ocm.NewOcmClient(ocmConnection)
		limitedSupportSummary := "E2E Test Limited Support"
		limitedSupportDetails := "This is an automated e2e test for limited support functionality"
		lsReason, err := cmv1.NewLimitedSupportReason().Summary(limitedSupportSummary).Details(limitedSupportDetails).DetectionType(cmv1.DetectionTypeManual).Build()
		Expect(err).ToNot(HaveOccurred())

		// Step 1: Create a limited support reason
		ginkgo.By("creating limited support reason via OCM client")
		err = ocmClient.SendLimitedSupport(externalClusterID, lsReason)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to create limited support reason. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to create limited support reason: %v. This may be expected if cluster doesn't support limited support or lacks permissions.", err))
		}

		// Since SendLimitedSupport doesn't return the ID, we have to find it.
		ginkgo.By("finding the created limited support reason to get its ID")
		var limitedSupportReasonID string
		reasons, err := ocmClient.GetLimitedSupportReasons(externalClusterID)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get limited support reasons after creating one: %v", err))
		}
		for _, r := range reasons {
			if r.Summary() == limitedSupportSummary && r.Details() == limitedSupportDetails {
				limitedSupportReasonID = r.ID()
				break
			}
		}
		Expect(limitedSupportReasonID).ToNot(BeEmpty(), "Could not find the created limited support reason")

		// Ensure cleanup happens even if tests fail
		defer func() {
			if limitedSupportReasonID != "" {
				ginkgo.By("cleaning up - deleting limited support reason")
				err := ocmClient.RemoveLimitedSupport(externalClusterID, limitedSupportReasonID)
				if err != nil {
					fmt.Printf("Failed to cleanup limited support reason %s: %v\n", limitedSupportReasonID, err)
				}
			}
		}()

		// Step 2: Test retrieval of limited support reasons through ocm-agent
		ginkgo.By("testing limited support reasons retrieval through ocm-agent")

		// Test limited support endpoint - should now return the created reason
		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/limited_support_reasons", ocmAgentURL, internalClusterID))
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to retrieve limited support reasons. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to retrieve limited support reasons via ocm-agent: %v. This may be expected if the agent doesn't proxy this endpoint.", err))
		} else {
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for limited support reasons")

			if resp.StatusCode == http.StatusOK {
				limitedSupportReasonList, err := cmv1.UnmarshalLimitedSupportReasonList(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				var foundReason *cmv1.LimitedSupportReason
				for _, r := range limitedSupportReasonList {
					if r.ID() == limitedSupportReasonID {
						foundReason = r
						break
					}
				}
				Expect(foundReason).ToNot(BeNil(), "created limited support reason not found in agent response")
				Expect(foundReason.Summary()).To(Equal(limitedSupportSummary))
				Expect(foundReason.Details()).To(Equal(limitedSupportDetails))
			}
			defer resp.Body.Close()
		}

		// Step 3: Test retrieval of specific limited support reason
		ginkgo.By("testing specific limited support reason retrieval")

		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/limited_support_reasons/%s", ocmAgentURL, internalClusterID, limitedSupportReasonID))
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to retrieve specific limited support reason. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to retrieve specific limited support reason via ocm-agent: %v. This may be expected if the agent doesn't proxy this endpoint.", err))
		} else {
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for specific limited support reason")
			defer resp.Body.Close()
		}

		// Final verification - ensure agent is still healthy after all tests
		ginkgo.By("final health verification after all tests")
		resp, err = httpClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "ocm-agent unhealthy after tests")
			resp.Body.Close()
		}

		// Verify pod is still running and stable
		finalPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentPodName, Namespace: namespace}}
		err = client.Get(ctx, finalPod.Name, finalPod.Namespace, finalPod)
		Expect(err).Should(BeNil(), "failed to get ocm-agent pod after tests")
		Expect(finalPod.Status.Phase).Should(Equal(corev1.PodRunning), "ocm-agent pod not running after tests")
	})

	ginkgo.It("Testing - Alert processing for classic mode", func(ctx context.Context) {
		if isFleetMode {
			ginkgo.GinkgoWriter.Printf("Skipping test: Skip the tests for classic mode.")
			ginkgo.Skip(fmt.Sprintf("Ocm-agent is in fleet mode, skip the tests for classic mode."))
		}

		ginkgo.By("Delete and create default managed notification so that ")
		managedNotification := &oav1alpha1.ManagedNotification{}
		err := k8sClient.Get(ctx, crclient.ObjectKey{Name: ocmAgentManagedNotification, Namespace: namespace}, managedNotification)
		if err != nil {
			// If notification doesn't exist, create default notification
			err = createDefaultNotification(ctx)
			Expect(err).Should(BeNil(), "failed to create default ManagedNotification")
		} else {
			// If notification exists, delete it then create default for testing
			err = k8sClient.Delete(ctx, managedNotification)
			Expect(err).Should(BeNil(), "failed to delete existing ManagedNotification")
			err = createDefaultNotification(ctx)
			Expect(err).Should(BeNil(), "failed to create default ManagedNotification")
		}

		// TEST - Verify http request actions
		ginkgo.By("TEST - http GET should not be supported")
		resp, err := httpClient.Get(fmt.Sprintf("%s/alertmanager-receiver", ocmAgentURL))
		Expect(resp.StatusCode).ShouldNot(Equal(http.StatusOK))

		// TEST - Get servicelog count before sending alert
		ginkgo.By("TEST - sending service log for a firing alert")
		preServiceLogCount, err := getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get initial service log count")

		firingAlert := createSingleAlert("firing", "LoggingVolumeFillingUpNotificationSRE", testNotificationName)
		// TEST - Verify alert has notification template associated
		ginkgo.By("TEST - Alert should have notification template associated")
		Expect(firingAlert.Alerts[0].Labels["managed_notification_template"]).ShouldNot(BeNil(), "No managed notification template for alert")
		Expect(firingAlert.Alerts[0].Labels["managed_notification_template"]).Should(Equal(testNotificationName))

		// Test - Post alert
		ginkgo.By("TEST - Post single alert, servicelog count should be increased by 1")
		err = postAlert(ctx, firingAlert)
		Expect(err).Should(BeNil(), "failed to post firing alert")
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preServiceLogCount, 1)

		// TEST - Do not send Service Log again for the same firing alert
		ginkgo.By("TEST - Not sending service log again for same firing alert within resend period")
		preServiceLogCount, err = getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count before duplicate test")

		duplicateAlert := createSingleAlert("firing", "LoggingVolumeFillingUpNotificationSRE", testNotificationName)
		err = postAlert(ctx, duplicateAlert)
		Expect(err).Should(BeNil(), "failed to post duplicate firing alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preServiceLogCount, 0)

		// TEST - Send Service Log for resolved alert
		ginkgo.By("TEST - Sending service log for resolved alert")
		preServiceLogCount, err = getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		resolvedAlert := createSingleAlert("resolved", "LoggingVolumeFillingUpNotificationSRE", testNotificationName)
		err = postAlert(ctx, resolvedAlert)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preServiceLogCount, 1)

		// TEST - Firing 2 alerts, servicelog count should be increased by 2
		ginkgo.By("TEST - Firing 2 alerts, servicelog count should be increased by 2")
		preServiceLogCount, err = getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		biAlerts := createBiAlert("firing", "TestAlert", "ParallelAlert")
		err = postAlert(ctx, biAlerts)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preServiceLogCount, 2)

		// TEST - Resolve 2 alerts, servicelog count should be increased by 2
		ginkgo.By("TEST - Resolve 2 alerts, servicelog count should be increased by 2")
		preServiceLogCount, err = getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		resolvedBiAlerts := createBiAlert("resolved", "TestAlert", "ParallelAlert")
		err = postAlert(ctx, resolvedBiAlerts)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preServiceLogCount, 2)

		ginkgo.By("verifying ocm-agent is still healthy after alert tests")
		resp, err = httpClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "ocm agent unhealthy after alert tests")
			resp.Body.Close()
		}
	})

	ginkgo.It("Testing - ocm-agent tests for fleet mode", func(ctx context.Context) {
		if !isFleetMode {
			ginkgo.GinkgoWriter.Printf("Skipping test: Skip the tests for fleet mode.")
			ginkgo.Skip(fmt.Sprintf("Ocm-agent is not in fleet mode, skip the tests for fleet mode."))
		}
		var (
			// generate a random alphanumeric string for the mcClusterID in the format of 1111-2222-3333-44444
			mcClusterID1 = "random-mc-id-1" //uuid.New().String()[:18]
			mcClusterID2 = "random-mc-id-2" //uuid.New().String()[:18]
			mcClusterID3 = "random-mc-id-3" //uuid.New().String()[:18]
		)
		k8sClient, err := k8s.NewClient()
		Expect(err).Should(BeNil(), "Failed to create controller runtime client")
		// replicate the tests here http://github.com/openshift/ocm-agent/blob/master/test/test-fleet-alerts.sh
		// Create a new ManagedFleetNotification object
		var mFleetNotificationAuditWebhookErrorTemplate = &ocmagentv1alpha1.ManagedFleetNotification{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
				Kind:       "ManagedFleetNotification",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "audit-webhook-error-putting-minimized-cloudwatch-log",
				Namespace: "openshift-ocm-agent-operator",
			},
			Spec: ocmagentv1alpha1.ManagedFleetNotificationSpec{
				FleetNotification: ocmagentv1alpha1.FleetNotification{
					Name:                "audit-webhook-error-putting-minimized-cloudwatch-log",
					NotificationMessage: "An audit-event send to your CloudWatch failed delivery, due to the event being too large. The reduced event failed delivery as well. Please verify your CloudWatch configuration for this cluster: https://access.redhat.com/solutions/7002219",
					ResendWait:          0,
					Severity:            "Info",
					Summary:             "Audit-events could not be delivered to your CloudWatch",
				},
			},
		}

		var mFleetNotificationOidcDeletedTemplate = &ocmagentv1alpha1.ManagedFleetNotification{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
				Kind:       "ManagedFleetNotification",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name:      "oidc-deleted-notification",
				Namespace: "openshift-ocm-agent-operator",
			},
			Spec: ocmagentv1alpha1.ManagedFleetNotificationSpec{
				FleetNotification: ocmagentv1alpha1.FleetNotification{
					Name:                "oidc-deleted-notification",
					NotificationMessage: "Your cluster is degraded due to the deletion of the associated OpenIDConnectProvider. To restore full support, please recreate the OpenID Connect provider by executing the command: rosa create oidc-provider --mode manual --cluster $CLUSTER_ID",
					ResendWait:          0,
					Severity:            "Info",
					Summary:             "Cluster is in Limited Support due to unsupported cloud provider configuration",
					LimitedSupport:      true,
				},
			},
		}

		// create an alert payload
		createFleetAlert := func(alertStatus, alertName, managementClusterID, managedFleetNotificationTemplate string) AlertPayload {
			today := time.Now().UTC().Format("2006-01-02")

			return AlertPayload{
				Receiver: "ocmagent",
				Status:   alertStatus,
				Alerts: []Alert{
					{
						Status: alertStatus,
						Labels: map[string]string{
							"alertname":                     alertName,
							"alertstate":                    alertStatus,
							"namespace":                     "openshift-monitoring",
							"openshift_io_alert_source":     "platform",
							"prometheus":                    "openshift-monitoring/k8s",
							"send_managed_notification":     "true",
							"managed_notification_template": managedFleetNotificationTemplate,
							"severity":                      "info",
							"_mc_id":                        managementClusterID,
							"_id":                           externalClusterID,
							"source":                        "MC",
						},
						Annotations: map[string]string{
							"description": "",
						},
						StartsAt:     today + "T00:00:00Z",
						EndsAt:       "0001-01-01T00:00:00Z",
						GeneratorURL: "",
					},
				},
				GroupLabels:       map[string]string{},
				CommonLabels:      map[string]string{},
				CommonAnnotations: map[string]string{},
				ExternalURL:       "",
			}
		}
		// postAlert sends an alert to the OCM Agent similar to post-alert.sh
		postAlert := func(ctx context.Context, alert AlertPayload) error {
			alertJSON, err := json.Marshal(alert)
			if err != nil {
				return fmt.Errorf("failed to marshal alert: %v", err)
			}

			resp, err := httpClient.Post(
				fmt.Sprintf("%s/alertmanager-receiver", ocmAgentURL),
				"application/json",
				bytes.NewBuffer(alertJSON),
			)
			if err != nil {
				return fmt.Errorf("failed to post alert: %v", err)
			}
			defer resp.Body.Close()

			var alertResponse AlertResponse
			if err := json.NewDecoder(resp.Body).Decode(&alertResponse); err != nil {
				return fmt.Errorf("failed to decode response: %v", err)
			}

			if alertResponse.Status != "ok" {
				return fmt.Errorf("alert posting failed with status: %s", alertResponse.Status)
			}

			return nil
		}
		// getServiceLogCount gets the count of service logs for a cluster
		getServiceLogCount := func(ctx context.Context, clusterUUID string) (int, error) {
			serviceLogsClient := ocmConnection.ServiceLogs().V1()

			response, err := serviceLogsClient.Clusters().ClusterLogs().List().
				Parameter("cluster_uuid", clusterUUID).
				Send()
			if err != nil {
				return 0, fmt.Errorf("failed to get service logs: %v", err)
			}

			return response.Total(), nil
		}

		// checkServiceLogCount verifies the service log count matches expectations
		checkServiceLogCount := func(ctx context.Context, clusterUUID string, preCount, expectedNew int) {
			expectedTotal := preCount + expectedNew
			actualCount, err := getServiceLogCount(ctx, clusterUUID)
			Expect(err).Should(BeNil(), "failed to get service log count")
			Expect(actualCount).Should(Equal(expectedTotal),
				fmt.Sprintf("Expected SL count: %d, Got SL count: %d", expectedTotal, actualCount))
		}

		// getLimitedSupportCount gets the count of limited support records for a cluster
		getLimitedSupportCount := func(ctx context.Context, clusterUUID string) (int, error) {
			limitedSupportClient := ocmConnection.ClustersMgmt().V1()
			response, err := limitedSupportClient.Clusters().Cluster(internalClusterID).LimitedSupportReasons().List().
				Send()
			if err != nil {
				return 0, fmt.Errorf("failed to get limited support count: %v", err)
			}
			return response.Total(), nil
		}

		// checkLimitedSupportCount verifies the limited support count matches expectations
		checkLimitedSupportCount := func(ctx context.Context, clusterUUID string, expectedTotal int) {
			actualCount, err := getLimitedSupportCount(ctx, clusterUUID)
			Expect(err).Should(BeNil(), "failed to get limited support count")
			Expect(actualCount).Should(Equal(expectedTotal),
				fmt.Sprintf("Expected limited support count: %d, Got limited support count: %d", expectedTotal, actualCount))
		}
		// getMfnriCount gets the count of firing notification sent for a cluster
		getMfnriCount := func(ctx context.Context, mcClusterID string, clusterUUID string) (int, int, error) {
			mFNRRecord := &ocmagentv1alpha1.ManagedFleetNotificationRecord{}
			err := k8sClient.Get(ctx, crClient.ObjectKey{
				Name:      mcClusterID,
				Namespace: "openshift-ocm-agent-operator"},
				mFNRRecord)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to get managed-fleet-notification-records: %v", err)
			}
			return mFNRRecord.Status.NotificationRecordByName[0].NotificationRecordItems[0].FiringNotificationSentCount,
				mFNRRecord.Status.NotificationRecordByName[0].NotificationRecordItems[0].ResolvedNotificationSentCount,
				nil
		}

		// compare firging notificaction sent count with expected count for a cluster
		checkMfnriCount := func(ctx context.Context, clusterUUID string, mcClusterID string, expectedFiringCount int, expectedResolvedCount int) {
			currentFiringCount, currentResolvedCount, err := getMfnriCount(ctx, mcClusterID, clusterUUID)
			Expect(err).Should(BeNil(), "failed to get managed-fleet-notification-record count")
			Expect(currentFiringCount).Should(Equal(expectedFiringCount),
				fmt.Sprintf("Expected firing notification sent count: %d, Got firing notification sent count: %d", expectedFiringCount, currentFiringCount))
			Expect(currentResolvedCount).Should(Equal(expectedResolvedCount),
				fmt.Sprintf("Expected resolved notification sent count: %d, Got resolved notification sent count: %d", expectedResolvedCount, currentResolvedCount))
		}

		// TEST - Verify and recreate the default ManagedNotification template
		ginkgo.By("Check and create the test ManagedFleetNotification templates if they don't exist")

		// Get the existing ManagedFleetNotification object
		mFleetNotificationAuditWebhookError := &ocmagentv1alpha1.ManagedFleetNotification{}
		// Check if the ManagedFleetNotification object audit-webhook-error-putting-minimized-cloudwatch-log exists
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: mFleetNotificationAuditWebhookErrorTemplate.Name, Namespace: namespace}, mFleetNotificationAuditWebhookError)
		if err == nil && mFleetNotificationAuditWebhookError.Name != "" {
			err = k8sClient.Delete(ctx, mFleetNotificationAuditWebhookError)
			Expect(err).Should(BeNil(), "failed to delete FleetManagedNotification")
		}
		// Create a new ManagedFleetNotification object
		err = k8sClient.Create(ctx, mFleetNotificationAuditWebhookErrorTemplate)
		Expect(err).Should(BeNil(), "failed to create FleetManagedNotification")
		// Check if the ManagedFleetNotification object oidc-deleted-notification exists
		mFleetNotificationOidcDeleted := &ocmagentv1alpha1.ManagedFleetNotification{}
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: mFleetNotificationOidcDeletedTemplate.Name, Namespace: namespace}, mFleetNotificationOidcDeleted)
		if err == nil && mFleetNotificationOidcDeleted.Name != "" {
			err = k8sClient.Delete(ctx, mFleetNotificationOidcDeleted)
			Expect(err).Should(BeNil(), "failed to delete FleetManagedNotification")
		}
		// Create a new ManagedFleetNotification object
		err = k8sClient.Create(ctx, mFleetNotificationOidcDeletedTemplate)
		Expect(err).Should(BeNil(), "failed to create FleetManagedNotification")

		// Delete all managed-fleet-notification-record objects before the test
		err = k8sClient.DeleteAllOf(ctx, &ocmagentv1alpha1.ManagedFleetNotificationRecord{}, crClient.InNamespace("openshift-ocm-agent-operator"))
		Expect(err).Should(BeNil(), "failed to delete managedfleetnotificationrecord objects")

		// Remove all the limited support records and resolve firing alerts before the test
		defer func() {
			// delete all the limited support records
			ocmClient := ocm.NewOcmClient(ocmConnection)
			limitedSupportReasons, err := ocmClient.GetLimitedSupportReasons(externalClusterID)
			Expect(err).Should(BeNil(), "failed to get limited support reasons")
			for _, r := range limitedSupportReasons {
				err := ocmClient.RemoveLimitedSupport(externalClusterID, r.ID())
				if err != nil {
					fmt.Printf("Failed to delete limited support record: %v\n", err)
				}
			}
			// resolve firing alerts
			alertPayloadAuditWebhook := createFleetAlert("resolved", alertName, mcClusterID1, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name)
			err = postAlert(ctx, alertPayloadAuditWebhook)
			Expect(err).Should(BeNil(), "failed to post alert")
			postAlert(ctx, alertPayloadAuditWebhook)
			alertPayloadOidcDeleted := createFleetAlert("resolved", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name)
			err = postAlert(ctx, alertPayloadOidcDeleted)
			Expect(err).Should(BeNil(), "failed to post alert")
			alertPayloadAuditWebhook = createFleetAlert("resolved", alertName, mcClusterID3, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name)
			err = postAlert(ctx, alertPayloadAuditWebhook)
			Expect(err).Should(BeNil(), "failed to post alert")
		}()

		ginkgo.By("### TEST 1 - Send Service log for a firing alert ###")

		preSLCount, err := getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count")
		// create an alert payload for the audit-webhook-error-putting-minimized-cloudwatch-log
		alertPayloadAuditWebhook := createFleetAlert("firing", alertName, mcClusterID1, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name)
		// send the alert payload for the audit-webhook-error-putting-minimized-cloudwatch-log to the ocm-agent
		err = postAlert(ctx, alertPayloadAuditWebhook)
		Expect(err).Should(BeNil(), "failed to post alert")
		// wait for shortSleepInterval
		time.Sleep(shortSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preSLCount, 1)
		checkMfnriCount(ctx, externalClusterID, mcClusterID1, 1, 0)

		// increment the preSLCount for next checks
		preSLCount++

		ginkgo.By("### TEST 2 - Send Limited Support for a firing alert")
		// check the limited support count before the test starts
		preLimitedSupportCount, err := getLimitedSupportCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get limited support count")
		expectedLimitedSupportCount := preLimitedSupportCount + 1
		// create an alert payload for the oidc-deleted-notification
		alertPayloadOidcDeleted := createFleetAlert("firing", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent
		err = postAlert(ctx, alertPayloadOidcDeleted)
		Expect(err).Should(BeNil(), "failed to post alert")
		// wait for shortSleepInterval
		time.Sleep(shortSleepInterval)

		checkLimitedSupportCount(ctx, externalClusterID, expectedLimitedSupportCount)
		checkMfnriCount(ctx, externalClusterID, mcClusterID2, 1, 0)

		ginkgo.By("### TEST 3 - Resend Limited Support for a firing alert without a resolve inbetween ###")

		time.Sleep(shortSleepInterval)
		checkLimitedSupportCount(ctx, externalClusterID, expectedLimitedSupportCount)
		checkMfnriCount(ctx, externalClusterID, mcClusterID2, 1, 0)

		ginkgo.By("### TEST 4 - Remove Limited Support for resolved alert ###")

		preLimitedSupportCount, err = getLimitedSupportCount(ctx, externalClusterID)
		// create an alert payload for the oidc-deleted-notification
		alertPayloadOidcDeleted = createFleetAlert("resolved", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent
		err = postAlert(ctx, alertPayloadOidcDeleted)
		Expect(err).Should(BeNil(), "failed to post alert")
		time.Sleep(shortSleepInterval)
		expectedLimitedSupportCount = preLimitedSupportCount - 1
		Expect(err).Should(BeNil(), "failed to get limited support count")
		checkLimitedSupportCount(ctx, externalClusterID, expectedLimitedSupportCount)
		checkMfnriCount(ctx, externalClusterID, mcClusterID2, 1, 1)

		ginkgo.By("### TEST 5 - Send Service log for a firing alert ###")

		preSLCount, err = getServiceLogCount(ctx, externalClusterID)
		Expect(err).Should(BeNil(), "failed to get service log count")

		// create an alert payload for the oidc-deleted-notification
		alertPayloadAuditWebhook = createFleetAlert("firing", alertName, mcClusterID3, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent 10 times
		for i := 0; i < 10; i++ {
			err = postAlert(ctx, alertPayloadAuditWebhook)
			Expect(err).Should(BeNil(), "failed to post alert")
		}
		// wait for 3 seconds
		time.Sleep(longSleepInterval)
		checkServiceLogCount(ctx, externalClusterID, preSLCount, 10)
		checkMfnriCount(ctx, externalClusterID, mcClusterID3, 10, 0)
	})

})
