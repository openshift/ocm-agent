package test

import (
	"context"
	"time"

	"github.com/openshift/ocm-agent/pkg/ocm"

	"github.com/prometheus/alertmanager/template"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
)

const (
	// Used to map between alert and notification
	TestNotificationName = "test-notification"
)

var (
	Context          = context.TODO()
	Scheme           = setScheme(runtime.NewScheme())
	TestNotification = ocmagentv1alpha1.Notification{
		Name:         TestNotificationName,
		Summary:      "test-summary",
		ActiveDesc:   "test-active-desc",
		ResolvedDesc: "test-resolved-desc",
		Severity:     "test-severity",
		ResendWait:   1,
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
	TestAlert = template.Alert{
		Status: "firing",
		Labels: map[string]string{
			"managed_notification_template": TestNotificationName,
			"send_managed_notification":     "true",
			"alertname":                     "TestAlertName",
			"alertstate":                    "firing",
			"namespace":                     "openshift-monitoring",
			"openshift_io_alert_source":     "platform",
			"prometheus":                    "openshift-monitoring/k8s",
			"severity":                      "info",
		},
		StartsAt: time.Now(),
		EndsAt:   time.Time{},
	}
	TestManagedNotificationList = &ocmagentv1alpha1.ManagedNotificationList{
		Items: []ocmagentv1alpha1.ManagedNotification{
			TestManagedNotification,
		},
	}
	TestActiveServiceLog = ocm.ServiceLog{
		ServiceName:  "SREManualAction",
		ClusterUUID:  "ddb5e04c-87ea-4fcd-b1f9-640981726cc5",
		Summary:      "Test SL Summary",
		InternalOnly: false,
		Description:  TestNotification.ActiveDesc,
	}
	TestResolvedServiceLog = ocm.ServiceLog{
		ServiceName:  "SREManualAction",
		ClusterUUID:  "ddb5e04c-87ea-4fcd-b1f9-640981726cc5",
		Summary:      "Test SL Summary",
		InternalOnly: false,
		Description:  TestNotification.ResolvedDesc,
	}
)

func setScheme(scheme *runtime.Scheme) *runtime.Scheme {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ocmagentv1alpha1.SchemeBuilder.AddToScheme(scheme))
	return scheme
}
