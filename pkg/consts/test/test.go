package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"
	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/prometheus/alertmanager/template"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"

	"github.com/openshift/ocm-agent/pkg/consts"
)

const (
	// Used to map between alert and notification
	TestNotificationName = "test-notification"
)

var (
	Context              = context.TODO()
	Scheme               = setScheme(runtime.NewScheme())
	TestManagedClusterID = "test-managed-cluster-id"
	TestHostedClusterID  = "test-hosted-cluster-id"
	TestNotification     = ocmagentv1alpha1.Notification{
		Name:         TestNotificationName,
		Summary:      "test-summary [namespace: '${namespace}']",
		ActiveDesc:   "test-active-desc [description: '${description}', overriden-key: '${overriden-key}', recursive-key: '${recursive-key}']",
		ResolvedDesc: "test-resolved-desc [description: '${description}', overriden-key: '${overriden-key}']",
		Severity:     "test-severity",
		ResendWait:   1,
		LogType:      "test-type",
		References: []ocmagentv1alpha1.NotificationReferenceType{
			"http://some.awesome.com/reference",
			"https://another.great.com/resource",
		},
	}
	ServiceLogSummary               = "test-summary [namespace: 'openshift-monitoring']"
	ServiceLogActiveDesc            = "test-active-desc [description: 'alert-desc', overriden-key: 'label-value', recursive-key: '_${recursive-key}_']"
	ServiceLogResolvedDesc          = "test-resolved-desc [description: 'alert-desc', overriden-key: 'label-value']"
	ServiceLogFleetDesc             = "test-notification [description: 'alert-desc', overriden-key: 'label-value']"
	NotificationWithoutResolvedBody = ocmagentv1alpha1.Notification{
		Name:       TestNotificationName,
		Summary:    "test-summary",
		ActiveDesc: "test-active-desc",
		Severity:   "test-severity",
		ResendWait: 1,
	}
	TestNotificationRecord = ocmagentv1alpha1.NotificationRecord{
		Name:                TestNotificationName,
		ServiceLogSentCount: 0,
		Conditions: []ocmagentv1alpha1.NotificationCondition{
			{
				Type:   ocmagentv1alpha1.ConditionAlertFiring,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   ocmagentv1alpha1.ConditionAlertResolved,
				Status: corev1.ConditionFalse,
			},
			{
				Type:   ocmagentv1alpha1.ConditionServiceLogSent,
				Status: corev1.ConditionTrue,
			},
		},
	}
	TestManagedNotification = ocmagentv1alpha1.ManagedNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mn",
			Namespace: "openshift-ocm-agent-operator",
		},
		Spec: ocmagentv1alpha1.ManagedNotificationSpec{
			Notifications: []ocmagentv1alpha1.Notification{TestNotification},
		},
		Status: ocmagentv1alpha1.ManagedNotificationStatus{
			NotificationRecords: ocmagentv1alpha1.NotificationRecords{
				TestNotificationRecord,
			},
		},
	}
	TestManagedNotificationWithoutStatus = ocmagentv1alpha1.ManagedNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mn",
			Namespace: "openshift-ocm-agent-operator",
		},
		Spec: ocmagentv1alpha1.ManagedNotificationSpec{
			Notifications: []ocmagentv1alpha1.Notification{TestNotification},
		},
	}
	TestManagedNotificationList = &ocmagentv1alpha1.ManagedNotificationList{
		Items: []ocmagentv1alpha1.ManagedNotification{
			TestManagedNotification,
		},
	}
)

// E2E Test Structs
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

// AlertResponse represents the response from the OCM Agent
type AlertResponse struct {
	Status string `json:"status"`
}

func NewFleetNotification() ocmagentv1alpha1.FleetNotification {
	return ocmagentv1alpha1.FleetNotification{
		Name:                TestNotificationName,
		Summary:             "test-summary [namespace: '${namespace}']",
		NotificationMessage: "test-notification [description: '${description}', overriden-key: '${overriden-key}']",
		References: []ocmagentv1alpha1.NotificationReferenceType{
			"http://some.awesome.com/reference",
			"https://another.great.com/resource",
		},
		Severity:   "test-severity",
		ResendWait: 1,
	}
}

func NewManagedFleetNotification(limited_support bool) ocmagentv1alpha1.ManagedFleetNotification {
	mfn := ocmagentv1alpha1.ManagedFleetNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestNotificationName,
			Namespace: "openshift-ocm-agent-operator",
		},
		Spec: ocmagentv1alpha1.ManagedFleetNotificationSpec{
			FleetNotification: NewFleetNotification(),
		},
	}

	if limited_support {
		mfn.Spec.FleetNotification.LimitedSupport = true
	}

	return mfn
}

func NewManagedFleetNotificationRecord() ocmagentv1alpha1.ManagedFleetNotificationRecord {
	return ocmagentv1alpha1.ManagedFleetNotificationRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestManagedClusterID,
			Namespace: "openshift-ocm-agent-operator",
		},
		Status: ocmagentv1alpha1.ManagedFleetNotificationRecordStatus{
			ManagementCluster:        TestManagedClusterID,
			NotificationRecordByName: nil,
		},
	}
}

func NewManagedFleetNotificationRecordWithStatus() ocmagentv1alpha1.ManagedFleetNotificationRecord {
	return ocmagentv1alpha1.ManagedFleetNotificationRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestManagedClusterID,
			Namespace: "openshift-ocm-agent-operator",
		},
		Status: ocmagentv1alpha1.ManagedFleetNotificationRecordStatus{
			ManagementCluster: TestManagedClusterID,
			NotificationRecordByName: []ocmagentv1alpha1.NotificationRecordByName{
				{
					NotificationName: TestNotificationName,
					ResendWait:       0,
					NotificationRecordItems: []ocmagentv1alpha1.NotificationRecordItem{
						{HostedClusterID: TestHostedClusterID},
					},
				},
			},
		},
	}
}

func NewTestAlert(resolved bool, fleet bool) template.Alert {
	alert := template.Alert{
		Labels: map[string]string{
			"managed_notification_template": TestNotificationName,
			"send_managed_notification":     "true",
			"alertname":                     "TestAlertName",
			"alertstate":                    "firing",
			"namespace":                     "openshift-monitoring",
			"openshift_io_alert_source":     "platform",
			"prometheus":                    "openshift-monitoring/k8s",
			"severity":                      "info",
			"overriden-key":                 "label-value",
		},
		Annotations: map[string]string{
			"description":   "alert-desc",
			"overriden-key": "annotation-value",
			"recursive-key": "_${recursive-key}_",
		},
		StartsAt: time.Now(),
		EndsAt:   time.Time{},
	}
	if resolved {
		alert.Status = "resolved"
		alert.Labels["alertstate"] = "resolved"
	} else {
		alert.Status = "firing"
		alert.Labels["alertstate"] = "firing"
	}

	if fleet {
		alert.Labels["_mc_id"] = TestManagedClusterID
		alert.Labels["_id"] = TestHostedClusterID
	}
	return alert
}

func setScheme(scheme *runtime.Scheme) *runtime.Scheme {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ocmagentv1alpha1.SchemeBuilder.AddToScheme(scheme))
	return scheme
}

func NewTestServiceLog(summary, desc, clusterUUID string, severity ocmagentv1alpha1.NotificationSeverity, logType string, references []ocmagentv1alpha1.NotificationReferenceType) *slv1.LogEntry {
	// Convert references to string slice for docReferences
	var docReferences []string
	for _, ref := range references {
		docReferences = append(docReferences, string(ref))
	}

	// Construct the ServiceLog using the LogEntryBuilder
	slBuilder := slv1.NewLogEntry().
		Summary(summary).
		Description(desc).
		DocReferences(docReferences...).
		ServiceName(consts.ServiceLogServiceName).
		ClusterUUID(clusterUUID).
		InternalOnly(false).
		Severity(slv1.Severity(severity)).
		LogType(slv1.LogType(logType))

	// Build the ServiceLog object
	sl, _ := slBuilder.Build()

	return sl
}

// E2E Test Helper Functions

// CreateNetworkPolicy creates a network policy which allow all traffic to ocm-agent for testing.
func CreateNetworkPolicy(ctx context.Context, client *resources.Resources, networkPolicyName, namespace string) error {
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
	err := client.Get(ctx, networkPolicyName, namespace, &networkingv1.NetworkPolicy{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = client.Create(ctx, networkPolicy)
			if err != nil {
				return fmt.Errorf("failed to create network policy: %v", err)
			}
		}
	}
	return nil
}

// CreateDefaultNotification creates a managed notification CRD for testing
func CreateDefaultNotification(ctx context.Context, k8sClient crClient.Client, namespace, ocmAgentManagedNotification, testNotificationName string) error {
	defaultNotification := &ocmagentv1alpha1.ManagedNotification{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ManagedNotification",
			APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ocmAgentManagedNotification,
			Namespace: namespace,
		},
		Spec: ocmagentv1alpha1.ManagedNotificationSpec{
			Notifications: []ocmagentv1alpha1.Notification{
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

// CreateSingleAlert creates an alert payload similar to the shell script create-alert.sh
func CreateSingleAlert(alertStatus, alertName, managedNotificationTemplate string) AlertPayload {
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

// CreateBiAlert creates an alert payload with two alerts
func CreateBiAlert(alertStatus, alertName, managedNotificationTemplate string) AlertPayload {
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

// PostAlert sends an alert to the OCM Agent similar to post-alert.sh
func PostAlert(ctx context.Context, alert AlertPayload, httpClient *http.Client, ocmAgentURL string) error {
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

// GetServiceLogCount gets the count of service logs for a cluster
func GetServiceLogCount(ctx context.Context, clusterUUID string, ocmConnection *sdk.Connection) (int, error) {
	serviceLogsClient := ocmConnection.ServiceLogs().V1()

	response, err := serviceLogsClient.Clusters().ClusterLogs().List().
		Parameter("cluster_uuid", clusterUUID).
		Send()
	if err != nil {
		return 0, fmt.Errorf("failed to get service logs: %v", err)
	}

	return response.Total(), nil
}

// CheckServiceLogCount verifies the service log count matches expectations
func CheckServiceLogCount(ctx context.Context, clusterUUID string, preCount, expectedNew int, ocmConnection *sdk.Connection) {
	expectedTotal := preCount + expectedNew
	actualCount, err := GetServiceLogCount(ctx, clusterUUID, ocmConnection)
	Expect(err).Should(BeNil(), "failed to get service log count")
	Expect(actualCount).Should(Equal(expectedTotal),
		fmt.Sprintf("Expected SL count: %d, Got SL count: %d", expectedTotal, actualCount))
}

// CreateFleetAlert creates an alert payload for fleet mode
func CreateFleetAlert(alertStatus, alertName, managementClusterID, managedFleetNotificationTemplate, externalClusterID string) AlertPayload {
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

// GetLimitedSupportCount gets the count of limited support records for a cluster
func GetLimitedSupportCount(ctx context.Context, internalClusterID string, ocmConnection *sdk.Connection) (int, error) {
	limitedSupportClient := ocmConnection.ClustersMgmt().V1()
	response, err := limitedSupportClient.Clusters().Cluster(internalClusterID).LimitedSupportReasons().List().
		Send()
	if err != nil {
		return 0, fmt.Errorf("failed to get limited support count: %v", err)
	}
	return response.Total(), nil
}

// CheckLimitedSupportCount verifies the limited support count matches expectations
func CheckLimitedSupportCount(ctx context.Context, internalClusterID string, expectedTotal int, ocmConnection *sdk.Connection) {
	actualCount, err := GetLimitedSupportCount(ctx, internalClusterID, ocmConnection)
	Expect(err).Should(BeNil(), "failed to get limited support count")
	Expect(actualCount).Should(Equal(expectedTotal),
		fmt.Sprintf("Expected limited support count: %d, Got limited support count: %d", expectedTotal, actualCount))
}

// GetMfnriCount gets the count of firing notification sent for a cluster
func GetMfnriCount(ctx context.Context, mcClusterID string, k8sClient crClient.Client) (int, int, error) {
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

// CheckMfnriCount compares firing notification sent count with expected count for a cluster
func CheckMfnriCount(ctx context.Context, mcClusterID string, expectedFiringCount int, expectedResolvedCount int, k8sClient crClient.Client) {
	currentFiringCount, currentResolvedCount, err := GetMfnriCount(ctx, mcClusterID, k8sClient)
	Expect(err).Should(BeNil(), "failed to get managed-fleet-notification-record count")
	Expect(currentFiringCount).Should(Equal(expectedFiringCount),
		fmt.Sprintf("Expected firing notification sent count: %d, Got firing notification sent count: %d", expectedFiringCount, currentFiringCount))
	Expect(currentResolvedCount).Should(Equal(expectedResolvedCount),
		fmt.Sprintf("Expected resolved notification sent count: %d, Got resolved notification sent count: %d", expectedResolvedCount, currentResolvedCount))
}
