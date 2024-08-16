package test

import (
	"context"
	"time"

	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"
	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/prometheus/alertmanager/template"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

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
