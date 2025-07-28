// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /test/e2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e

package osde2etests

import (
	"context"
	"os"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

var _ = ginkgo.Describe("ocm-agent", ginkgo.Ordered, func() {

	var (
		client         *resources.Resources
		namespace      = "openshift-ocm-agent-operator"
		deploymentName = "ocm-agent"
		deployments    = []string{
			deploymentName,
			deploymentName + "-operator",
		}
		managedNotificationName = "sre-managed-notifications"
		yamlFilePath            = "../../manifests/sre-managed-notifications.yaml"
	)

	ginkgo.BeforeAll(func() {
		// setup the k8s client
		cfg, err := config.GetConfig()
		Expect(err).Should(BeNil(), "failed to get kubeconfig")
		client, err = resources.New(cfg)
		Expect(err).Should(BeNil(), "resources.New error")

	})

	ginkgo.It("Testing - SREP-909", func(ctx context.Context) {

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

		// TODO(SREP-910): Add the following tests
		//TEST - Ensure that ocm-agent starts successfully
		//TEST - Ensure that ocm-agent is able to configure and build an ocm connection successfully
		//TEST - Ensure that ocm-agent sends a successful health check request to ocm api
		//TEST - Verify that the ocm-agent handles 4xx(400 Not Found) response gracefully
		//TEST - Verify that ocm-agent handles 5xx(503 service/api unavailable) response gracefully
		//TEST - Verify the timeout handling when ocm api responds slowly
		//TEST - Get list of all the upgrade policies belonging to a cluster from ocm api
		//TEST - Verify that ocm-agent sends a successful request to ocm api to get all upgrade policies
		//TEST - Get single upgrade policy belonging to a cluster from ocm api
		//TEST - Fetch state of single upgrade policy belonging to a cluster from ocm api
		//TEST - Update state of a single upgrade policy for a given cluster
		//TEST - Fetch Limited support reasons for cluster

	})

	ginkgo.It("ROSA Classic e2e tests", func(ctx context.Context) {

		// replicate the tests here http://github.com/openshift/ocm-agent/blob/master/test/test-alerts.sh
		// TEST - Verify and recreate the default ManagedNotification template
		ginkgo.By("Verify and recreate the test ManagedNotification template")
		var managedNotification ocmagentv1alpha1.ManagedNotification
		err := client.Get(ctx, managedNotificationName, namespace, &managedNotification)
		if err == nil && managedNotification.Name != "" {
			//Delete existing ManagedNotificaiton CR
			client.Delete(ctx, &managedNotification)
		}

		// Read YAML file and parse to ManagedNotification object
		yamlContent, err := os.ReadFile(yamlFilePath)
		Expect(err).Should(BeNil(), "failed to read YAML file: %s", yamlFilePath)

		var testManagedNotificationObj ocmagentv1alpha1.ManagedNotification
		err = yaml.Unmarshal(yamlContent, &testManagedNotificationObj)
		Expect(err).Should(BeNil(), "failed to unmarshal YAML content from file: %s", yamlFilePath)

		//Create new ManagedNotification CR
		err = client.Create(ctx, &testManagedNotificationObj)
		Expect(err).Should(BeNil(), "failed to create ManagedNotification")

		// TEST - Validate that the request method is allowed
		// TEST - Verify that the request is valid before processing the alert
		// TEST - Validate that supplied alert is one that warrants being processed for a notification
		// TEST - Send service log for firing alert in case of no NotificationRecords
		// TEST - Resend service log for firing alert iff NotificationRecords exists and resendWait interval exceeded
		// TEST - Ensure that firing and resolved alerts processed successfully
		// TEST - Verify actual firing notification count with expected
		// TEST - Verify actual resolved notification count with expected
		// TEST - Verify actual service log count with expected
		// TEST - Check for parallel execution of the alerts
	})
})
