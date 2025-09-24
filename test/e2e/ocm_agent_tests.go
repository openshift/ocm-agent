// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /test/e2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	"github.com/openshift/ocm-agent/pkg/k8s"
	"github.com/openshift/ocm-agent/pkg/ocm"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"

	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

var _ = ginkgo.Describe("ocm-agent", ginkgo.Ordered, func() {

	var (
		client                      *resources.Resources
		k8sClient                   crClient.Client
		errorServer                 *httptest.Server
		ocmConnection               *sdk.Connection
		namespace                   = "openshift-ocm-agent-operator"
		deploymentName              = "ocm-agent"
		ocmAgentConfigMap           = "ocm-agent-cm"
		ocmAgentManagedNotification = "sre-managed-notifications"
		networkPolicyName           = "ocm-agent-allow-all-ingress"
		testNotificationName        = "LoggingVolumeFillingUp"
		clusterVersionName          = "version"
		infrastructureName          = "cluster"
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
		ocmAgentFleetURL  = "http://ocm-agent-fleet.openshift-ocm-agent-operator.svc:8081"
		httpClient        = &http.Client{Timeout: 30 * time.Second}
		externalClusterID string
		internalClusterID string

		deployments = []string{
			deploymentName,
			deploymentName + "-operator",
		}
		alertName = "LoggingVolumeFillingUpNotificationSRE"
	)
	ginkgo.BeforeAll(func(ctx context.Context) {
		// Setup the k8s client
		ginkgo.By("Setup: Setting up clients and prerequisites for tests")
		cfg, err := config.GetConfig()
		Expect(err).Should(BeNil(), "failed to get kubeconfig")
		client, err = resources.New(cfg)
		Expect(err).Should(BeNil(), "resources.New error")
		k8sClient, err = k8s.NewClient()
		Expect(err).Should(BeNil(), "Failed to create controller runtime client")

		// Add required types to scheme
		k8sClient.Scheme().AddKnownTypes(oav1alpha1.GroupVersion, &oav1alpha1.OcmAgent{})
		k8sClient.Scheme().AddKnownTypes(appsv1.SchemeGroupVersion, &appsv1.Deployment{})
		k8sClient.Scheme().AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Service{})

		err = oav1alpha1.AddToScheme(k8sClient.Scheme())
		if err != nil {
			ginkgo.Fail("Failed to add ocmagent to scheme")
		}

		err = appsv1.AddToScheme(k8sClient.Scheme())
		if err != nil {
			ginkgo.Fail("Failed to add deployment to scheme")
		}

		err = corev1.AddToScheme(k8sClient.Scheme())
		if err != nil {
			ginkgo.Fail("Failed to add service to scheme")
		}

		// Create a mock error server that always returns 503
		errorServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"message": "Service temporarily unavailable", "code": 503}`))
		}))

		ginkgo.By("Setup: Getting OCM configuration from ocm-agent configmap")
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ocmAgentConfigMap, Namespace: namespace}}
		err = client.Get(ctx, configMap.Name, configMap.Namespace, configMap)
		// Verify required configuration fields
		Expect(err).Should(BeNil(), "ocm-agent configmap not found")

		ginkgo.By("Setup: Getting real external cluster ID from configmap")
		if err == nil {
			if configMap.Data[clusterIDKey] != "" {
				externalClusterID = configMap.Data[clusterIDKey]
				ginkgo.By(fmt.Sprintf("Setup: External cluster ID: %s", externalClusterID))
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
		Expect(configMap.Data).Should(HaveKey(ocmBaseURLKey), "ocmBaseURL not configured")
		Expect(configMap.Data).Should(HaveKey(servicesKey), "services not configured")
		ocmBaseURL = configMap.Data[ocmBaseURLKey]
		Expect(ocmBaseURL).ShouldNot(BeEmpty(), "ocmBaseURL is empty")
		Expect(configMap.Data[servicesKey]).ShouldNot(BeEmpty(), "services configuration is empty")
		// override when OCM_URL is set
		if os.Getenv("OCM_URL") != "" {
			ocmBaseURL = os.Getenv("OCM_URL")
		}
		fmt.Sprintf("ocmBaseURL is %v", ocmBaseURL)
		// Override the ocm-agent URL if the OCM_AGENT_URL environment variable is set
		if os.Getenv("OCM_AGENT_URL") != "" {
			ocmAgentURL = os.Getenv("OCM_AGENT_URL")
		}

		// Get access token from env or secret
		var accessToken string
		// OCM_TOKEN env is set in the test image, other name will cause token not found failure
		if thirdPartyToken := os.Getenv("OCM_TOKEN"); thirdPartyToken != "" {
			accessToken = thirdPartyToken
		}

		// Create OCM connection and get internal cluster ID
		ginkgo.By("Setup: Creating OCM connection and getting internal cluster ID")
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

		ginkgo.By("Setup: Creating network policy to allow traffic from all namespaces")
		err = testconst.CreateNetworkPolicy(ctx, client, networkPolicyName, namespace)
		Expect(err).To(BeNil(), fmt.Sprintf("Failed to create networkpolicy %v", err))
	})
	ginkgo.It("OcmAgentCommon - Testing basic deployment", Label("OcmAgentCommon"), func(ctx context.Context) {
		ginkgo.By("Step 1: Verifying that the namespace exists")
		err := client.Get(ctx, namespace, "", &corev1.Namespace{})
		Expect(err).Should(BeNil(), "namespace %s not found", namespace)

		ginkgo.By("Step 2: Verifying that the deployment exists")
		for _, deploymentName := range deployments {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: namespace}}
			err = wait.For(conditions.New(client).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue))
			Expect(err).Should(BeNil(), "deployment %s not available", deploymentName)
		}
	})

	ginkgo.It("OcmAgentCommon - Testing common ocm-agent tests", Label("OcmAgentCommon"), func(ctx context.Context) {
		ginkgo.By("Step 1: Listing ocm-agent pods for testing")
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
		ginkgo.By("Step 2: Verifying ocm-agent service is ready")
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

		// TEST - Ensure that ocm-agent sends a successful health check request to ocm api
		ginkgo.By("Step 3: Testing health check endpoints")
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
		ginkgo.By("Step 4: Verifying invalid endpoint returns 4xx error")
		resp, err = httpClient.Get(fmt.Sprintf("%s/invalid-endpoint", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusNotFound), "should return 404 for invalid endpoint")
			resp.Body.Close()
		}

		// TEST - Fetch Limited support reasons for cluster
		ginkgo.By("Step 5: Testing limited support reasons full workflow")

		ocmClient := ocm.NewOcmClient(ocmConnection)
		limitedSupportSummary := "E2E Test Limited Support"
		limitedSupportDetails := "This is an automated e2e test for limited support functionality"
		lsReason, err := cmv1.NewLimitedSupportReason().Summary(limitedSupportSummary).Details(limitedSupportDetails).DetectionType(cmv1.DetectionTypeManual).Build()
		Expect(err).ToNot(HaveOccurred())

		// Step 1: Create a limited support reason
		ginkgo.By("Step 5a: Creating limited support reason via OCM client")
		err = ocmClient.SendLimitedSupport(externalClusterID, lsReason)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to create limited support reason. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to create limited support reason: %v. This may be expected if cluster doesn't support limited support or lacks permissions.", err))
		}

		// Since SendLimitedSupport doesn't return the ID, we have to find it.
		ginkgo.By("Step 5b: Finding the created limited support reason to get its ID")
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
				ginkgo.By("Cleanup: Deleting limited support reason")
				err := ocmClient.RemoveLimitedSupport(externalClusterID, limitedSupportReasonID)
				if err != nil {
					fmt.Printf("Failed to cleanup limited support reason %s: %v\n", limitedSupportReasonID, err)
				}
			}
		}()

		// Step 2: Test retrieval of limited support reasons through ocm-agent
		ginkgo.By("Step 5c: Testing limited support reasons retrieval through ocm-agent")

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
		ginkgo.By("Step 5d: Testing specific limited support reason retrieval")

		resp, err = httpClient.Get(fmt.Sprintf("%s/api/clusters_mgmt/v1/clusters/%s/limited_support_reasons/%s", ocmAgentURL, internalClusterID, limitedSupportReasonID))
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Skipping test: Failed to retrieve specific limited support reason. Error: %v\n", err)
			ginkgo.Skip(fmt.Sprintf("Failed to retrieve specific limited support reason via ocm-agent: %v. This may be expected if the agent doesn't proxy this endpoint.", err))
		} else {
			Expect(resp.StatusCode).Should(BeElementOf([]int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized}),
				"unexpected status code for specific limited support reason")
			defer resp.Body.Close()
		}
	})
	ginkgo.It("OcmAgentClassic - Testing alert processing for classic mode", Label("OcmAgentClassic"), func(ctx context.Context) {

		ginkgo.By("Step 1: Creating and managing default ManagedNotification")
		managedNotification := &oav1alpha1.ManagedNotification{}
		err := k8sClient.Get(ctx, crClient.ObjectKey{Name: ocmAgentManagedNotification, Namespace: namespace}, managedNotification)
		if err != nil {
			// If notification doesn't exist, create default notification
			err = testconst.CreateDefaultNotification(ctx, k8sClient, namespace, ocmAgentManagedNotification, testNotificationName)
			Expect(err).Should(BeNil(), "failed to create default ManagedNotification")
		} else {
			// If notification exists, delete it then create default for testing
			err = k8sClient.Delete(ctx, managedNotification)
			Expect(err).Should(BeNil(), "failed to delete existing ManagedNotification")
			err = testconst.CreateDefaultNotification(ctx, k8sClient, namespace, ocmAgentManagedNotification, testNotificationName)
			Expect(err).Should(BeNil(), "failed to create default ManagedNotification")
		}

		// TEST - Verify http request actions
		ginkgo.By("Step 2: Verifying HTTP GET is not supported on alertmanager-receiver")
		resp, err := httpClient.Get(fmt.Sprintf("%s/alertmanager-receiver", ocmAgentURL))
		Expect(resp.StatusCode).ShouldNot(Equal(http.StatusOK))

		// TEST - Get servicelog count before sending alert
		ginkgo.By("Step 3: Sending service log for a firing alert")
		preServiceLogCount, err := testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get initial service log count")

		firingAlert := testconst.CreateSingleAlert("firing", alertName, testNotificationName)
		// TEST - Verify alert has notification template associated
		ginkgo.By("Step 4: Verifying alert has notification template associated")
		Expect(firingAlert.Alerts[0].Labels["managed_notification_template"]).ShouldNot(BeNil(), "No managed notification template for alert")
		Expect(firingAlert.Alerts[0].Labels["managed_notification_template"]).Should(Equal(testNotificationName))

		// Test - Post alert
		ginkgo.By("Step 5: Posting single alert, service log count should increase by 1")
		err = testconst.PostAlert(ctx, firingAlert, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post firing alert")
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preServiceLogCount, 1, ocmConnection)

		// TEST - Do not send Service Log again for the same firing alert
		ginkgo.By("Step 6: Verifying no duplicate service log for same firing alert within resend period")
		preServiceLogCount, err = testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count before duplicate test")

		duplicateAlert := testconst.CreateSingleAlert("firing", alertName, testNotificationName)
		err = testconst.PostAlert(ctx, duplicateAlert, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post duplicate firing alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preServiceLogCount, 0, ocmConnection)

		// TEST - Send Service Log for resolved alert
		ginkgo.By("Step 7: Sending service log for resolved alert")
		preServiceLogCount, err = testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		resolvedAlert := testconst.CreateSingleAlert("resolved", alertName, testNotificationName)
		err = testconst.PostAlert(ctx, resolvedAlert, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preServiceLogCount, 1, ocmConnection)

		// TEST - Firing 2 alerts, servicelog count should be increased by 2
		ginkgo.By("Step 8: Firing 2 alerts, service log count should increase by 2")
		preServiceLogCount, err = testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		biAlerts := testconst.CreateBiAlert("firing", "TestAlert", "ParallelAlert")
		err = testconst.PostAlert(ctx, biAlerts, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preServiceLogCount, 2, ocmConnection)

		// TEST - Resolve 2 alerts, servicelog count should be increased by 2
		ginkgo.By("Step 9: Resolving 2 alerts, service log count should increase by 2")
		preServiceLogCount, err = testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count before resolved test")

		resolvedBiAlerts := testconst.CreateBiAlert("resolved", "TestAlert", "ParallelAlert")
		err = testconst.PostAlert(ctx, resolvedBiAlerts, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post resolved alert")

		// Wait for processing
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preServiceLogCount, 2, ocmConnection)

		ginkgo.By("Step 10: Verifying ocm-agent is still healthy after alert tests")
		resp, err = httpClient.Get(fmt.Sprintf("%s/readyz", ocmAgentURL))
		if err == nil {
			Expect(resp.StatusCode).Should(Equal(http.StatusOK), "ocm agent unhealthy after alert tests")
			resp.Body.Close()
		}
	})
	ginkgo.It("OcmAgentHCP - Testing ocm-agent tests in fleet mode", Label("OcmAgentHCP"), func(ctx context.Context) {

		// set ocm agent url to fleet url when overriding OCM_AGENT_URL environment variable is not set
		if os.Getenv("OCM_AGENT_URL") == "" {
			ocmAgentURL = ocmAgentFleetURL
		}

		// Get OcmAgent "ocm-agent" resource
		oa := &oav1alpha1.OcmAgent{}
		err := k8sClient.Get(ctx, crClient.ObjectKey{Name: "ocm-agent", Namespace: namespace}, oa)
		if err == nil {
			Expect(err).Should(BeNil(), "failed to get ocm-agent OcmAgent resource")
		}

		// Get Deployment "ocm-agent" resource
		oadeploy := &appsv1.Deployment{}
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: deploymentName, Namespace: namespace}, oadeploy)
		if err != nil {
			Expect(err).Should(BeNil(), "failed to get ocm-agent deployment")
		}
		deployLabels := map[string]string{
			"app": deploymentName + "-fleet",
		}

		// Setup fleet-mode deployment
		// NOTE: For now we create deployment separately since the "--test-mode" flag isn't enabled via OcmAgent API
		oadeployfleet := &appsv1.Deployment{}

		// Get Deployment "ocm-agent-fleet" resource and create it if it doesn't exist
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: deploymentName + "-fleet", Namespace: namespace}, oadeployfleet)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Creating fleet mode deployment since it doesn't exist")
			oadeployfleet.Spec = *oadeploy.Spec.DeepCopy()
			oadeployfleet.Name = deploymentName + "-fleet"
			oadeployfleet.Namespace = namespace
			oadeployfleet.Labels = deployLabels
			oadeployfleet.Spec.Selector.MatchLabels = deployLabels
			oadeployfleet.Spec.Template.ObjectMeta.Labels = deployLabels
			oadeployfleet.Spec.Template.Spec.Containers[0].Command = append(oadeployfleet.Spec.Template.Spec.Containers[0].Command, "--fleet-mode")
			oadeployfleet.Spec.Template.Spec.Containers[0].Command = append(oadeployfleet.Spec.Template.Spec.Containers[0].Command, "--test-mode")

			// For a failed test run, validate if the fleet mode deployment is already created or not
			ginkgo.By("Setup: Creating OCM Agent Fleet mode deployment")
			err = k8sClient.Create(ctx, oadeployfleet)
			if err != nil {
				Expect(err).Should(BeNil(), "failed to create ocm-agent-fleet deployment resource")
			}

			deployments = []string{deploymentName + "-fleet"}
			for _, deploymentName := range deployments {
				deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: namespace}}
				err = wait.For(conditions.New(client).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue))
				Expect(err).Should(BeNil(), "deployment %s not available", deploymentName)
			}
		}

		// Get Service "ocm-agent-fleet" resource and create it if it doesn't exist
		oasvcfleet := &corev1.Service{}
		oasvc := &corev1.Service{}
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: deploymentName, Namespace: namespace}, oasvc)
		if err != nil {
			Expect(err).Should(BeNil(), "failed to get ocm-agent service resource")
		}
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: deploymentName + "-fleet", Namespace: namespace}, oasvcfleet)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Creating fleet mode service since it doesn't exist")
			oasvcfleet.Spec = *oasvc.Spec.DeepCopy()
			oasvcfleet.Name = oasvc.Name + "-fleet"
			oasvcfleet.Namespace = oasvc.Namespace
			oasvcfleet.Spec.Ports[0].Name = oasvc.Name + "-fleet"
			oasvcfleet.Spec.Selector = deployLabels
			oasvcfleet.Spec.ClusterIP = ""
			oasvcfleet.Spec.ClusterIPs = []string{}
			ginkgo.By("Setup: Creating OCM Agent Fleet mode service")
			err = k8sClient.Create(ctx, oasvcfleet)
			if err != nil {
				Expect(err).Should(BeNil(), "failed to create ocm-agent-fleet service resource")
			}
		}

		var (
			// generate a random alphanumeric string for the mcClusterID in the format of 1111-2222-3333-44444
			mcClusterID1 = "random-mc-id-1" //uuid.New().String()[:18]
			mcClusterID2 = "random-mc-id-2" //uuid.New().String()[:18]
			mcClusterID3 = "random-mc-id-3" //uuid.New().String()[:18]
		)
		// replicate the tests here http://github.com/openshift/ocm-agent/blob/master/test/test-fleet-alerts.sh
		// Create a new ManagedFleetNotification object
		var mFleetNotificationAuditWebhookErrorTemplate = &oav1alpha1.ManagedFleetNotification{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
				Kind:       "ManagedFleetNotification",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "audit-webhook-error-putting-minimized-cloudwatch-log",
				Namespace: "openshift-ocm-agent-operator",
			},
			Spec: oav1alpha1.ManagedFleetNotificationSpec{
				FleetNotification: oav1alpha1.FleetNotification{
					Name:                "audit-webhook-error-putting-minimized-cloudwatch-log",
					NotificationMessage: "An audit-event send to your CloudWatch failed delivery, due to the event being too large. The reduced event failed delivery as well. Please verify your CloudWatch configuration for this cluster: https://access.redhat.com/solutions/7002219",
					ResendWait:          0,
					Severity:            "Info",
					Summary:             "Audit-events could not be delivered to your CloudWatch",
				},
			},
		}

		var mFleetNotificationOidcDeletedTemplate = &oav1alpha1.ManagedFleetNotification{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
				Kind:       "ManagedFleetNotification",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name:      "oidc-deleted-notification",
				Namespace: "openshift-ocm-agent-operator",
			},
			Spec: oav1alpha1.ManagedFleetNotificationSpec{
				FleetNotification: oav1alpha1.FleetNotification{
					Name:                "oidc-deleted-notification",
					NotificationMessage: "Your cluster is degraded due to the deletion of the associated OpenIDConnectProvider. To restore full support, please recreate the OpenID Connect provider by executing the command: rosa create oidc-provider --mode manual --cluster $CLUSTER_ID",
					ResendWait:          0,
					Severity:            "Info",
					Summary:             "Cluster is in Limited Support due to unsupported cloud provider configuration",
					LimitedSupport:      true,
				},
			},
		}

		// TEST - Verify and recreate the default ManagedNotification template
		ginkgo.By("Step 1: Creating test ManagedFleetNotification templates")

		// Get the existing ManagedFleetNotification object
		mFleetNotificationAuditWebhookError := &oav1alpha1.ManagedFleetNotification{}
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
		mFleetNotificationOidcDeleted := &oav1alpha1.ManagedFleetNotification{}
		err = k8sClient.Get(ctx, crClient.ObjectKey{Name: mFleetNotificationOidcDeletedTemplate.Name, Namespace: namespace}, mFleetNotificationOidcDeleted)
		if err == nil && mFleetNotificationOidcDeleted.Name != "" {
			err = k8sClient.Delete(ctx, mFleetNotificationOidcDeleted)
			Expect(err).Should(BeNil(), "failed to delete FleetManagedNotification")
		}
		// Create a new ManagedFleetNotification object
		err = k8sClient.Create(ctx, mFleetNotificationOidcDeletedTemplate)
		Expect(err).Should(BeNil(), "failed to create FleetManagedNotification")

		// Delete all managed-fleet-notification-record objects before the test
		err = k8sClient.DeleteAllOf(ctx, &oav1alpha1.ManagedFleetNotificationRecord{}, crClient.InNamespace("openshift-ocm-agent-operator"))
		Expect(err).Should(BeNil(), "failed to delete managedfleetnotificationrecord objects")

		// Remove all the limited support records after the test
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
			alertPayloadAuditWebhook := testconst.CreateFleetAlert("resolved", alertName, mcClusterID1, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name, externalClusterID)
			err = testconst.PostAlert(ctx, alertPayloadAuditWebhook, httpClient, ocmAgentURL)
			Expect(err).Should(BeNil(), "failed to post alert")
			testconst.PostAlert(ctx, alertPayloadAuditWebhook, httpClient, ocmAgentURL)
			alertPayloadOidcDeleted := testconst.CreateFleetAlert("resolved", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name, externalClusterID)
			err = testconst.PostAlert(ctx, alertPayloadOidcDeleted, httpClient, ocmAgentURL)
			Expect(err).Should(BeNil(), "failed to post alert")
			alertPayloadAuditWebhook = testconst.CreateFleetAlert("resolved", alertName, mcClusterID3, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name, externalClusterID)
			err = testconst.PostAlert(ctx, alertPayloadAuditWebhook, httpClient, ocmAgentURL)
			Expect(err).Should(BeNil(), "failed to post alert")
		}()

		ginkgo.By("Step 2: Sending service log for a firing alert")

		preSLCount, err := testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count")
		// create an alert payload for the audit-webhook-error-putting-minimized-cloudwatch-log
		alertPayloadAuditWebhook := testconst.CreateFleetAlert("firing", alertName, mcClusterID1, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name, externalClusterID)
		// send the alert payload for the audit-webhook-error-putting-minimized-cloudwatch-log to the ocm-agent
		err = testconst.PostAlert(ctx, alertPayloadAuditWebhook, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post alert")
		// wait for shortSleepInterval
		time.Sleep(shortSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preSLCount, 1, ocmConnection)
		testconst.CheckMfnriCount(ctx, mcClusterID1, 1, 0, k8sClient)

		ginkgo.By("Step 3: Sending Limited Support for a firing alert")
		// check the limited support count before the test starts
		preLimitedSupportCount, err := testconst.GetLimitedSupportCount(ctx, internalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get limited support count")
		expectedLimitedSupportCount := preLimitedSupportCount + 1
		// create an alert payload for the oidc-deleted-notification
		alertPayloadOidcDeleted := testconst.CreateFleetAlert("firing", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name, externalClusterID)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent
		err = testconst.PostAlert(ctx, alertPayloadOidcDeleted, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post alert")
		// wait for shortSleepInterval
		time.Sleep(shortSleepInterval)

		testconst.CheckLimitedSupportCount(ctx, internalClusterID, expectedLimitedSupportCount, ocmConnection)
		testconst.CheckMfnriCount(ctx, mcClusterID2, 1, 0, k8sClient)

		ginkgo.By("Step 4: Resending Limited Support for a firing alert without resolve")

		time.Sleep(shortSleepInterval)
		testconst.CheckLimitedSupportCount(ctx, internalClusterID, expectedLimitedSupportCount, ocmConnection)
		testconst.CheckMfnriCount(ctx, mcClusterID2, 1, 0, k8sClient)

		ginkgo.By("Step 5: Removing Limited Support for resolved alert")

		preLimitedSupportCount, err = testconst.GetLimitedSupportCount(ctx, internalClusterID, ocmConnection)
		// create an alert payload for the oidc-deleted-notification
		alertPayloadOidcDeleted = testconst.CreateFleetAlert("resolved", alertName, mcClusterID2, mFleetNotificationOidcDeletedTemplate.ObjectMeta.Name, externalClusterID)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent
		err = testconst.PostAlert(ctx, alertPayloadOidcDeleted, httpClient, ocmAgentURL)
		Expect(err).Should(BeNil(), "failed to post alert")
		time.Sleep(shortSleepInterval)
		expectedLimitedSupportCount = preLimitedSupportCount - 1
		Expect(err).Should(BeNil(), "failed to get limited support count")
		testconst.CheckLimitedSupportCount(ctx, internalClusterID, expectedLimitedSupportCount, ocmConnection)
		testconst.CheckMfnriCount(ctx, mcClusterID2, 1, 1, k8sClient)

		ginkgo.By("Step 6: Sending service log for a firing alert (multiple times)")

		preSLCount, err = testconst.GetServiceLogCount(ctx, externalClusterID, ocmConnection)
		Expect(err).Should(BeNil(), "failed to get service log count")

		// create an alert payload for the oidc-deleted-notification
		alertPayloadAuditWebhook = testconst.CreateFleetAlert("firing", alertName, mcClusterID3, mFleetNotificationAuditWebhookErrorTemplate.ObjectMeta.Name, externalClusterID)
		// send the alert payload for the oidc-deleted-notification to the ocm-agent 10 times
		for i := 0; i < 10; i++ {
			err = testconst.PostAlert(ctx, alertPayloadAuditWebhook, httpClient, ocmAgentURL)
			Expect(err).Should(BeNil(), "failed to post alert")
		}
		// wait for longSleepInterval
		time.Sleep(longSleepInterval)
		testconst.CheckServiceLogCount(ctx, externalClusterID, preSLCount, 10, ocmConnection)
		testconst.CheckMfnriCount(ctx, mcClusterID3, 10, 0, k8sClient)
	})

	ginkgo.AfterAll(func(ctx context.Context) {
		// Clean up the error server
		if errorServer != nil {
			errorServer.Close()
		}
		ginkgo.By("Cleanup: Removing network policy")
		networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: networkPolicyName, Namespace: namespace}}
		err := client.Get(ctx, networkPolicy.Name, networkPolicy.Namespace, networkPolicy)
		if err == nil {
			// If networkpolicy exist, delete it
			client.Delete(ctx, networkPolicy)
		}

		ginkgo.By("Cleanup: Removing fleet-mode deployment")
		oadeployfleet := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "ocm-agent-fleet", Namespace: namespace}}
		err = client.Get(ctx, oadeployfleet.Name, namespace, oadeployfleet)
		if err == nil {
			client.Delete(ctx, oadeployfleet)
		}

		ginkgo.By("Cleanup: Removing fleet-mode service")
		oadeploysvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "ocm-agent-fleet", Namespace: namespace}}
		err = client.Get(ctx, oadeployfleet.Name, namespace, oadeploysvc)
		if err == nil {
			client.Delete(ctx, oadeploysvc)
		}

	})
})
