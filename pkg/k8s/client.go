package k8s

import (
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const GroupName = "ocmagent.managed.openshift.io"
const GroupVersion = "v1alpha1"

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

// NewClient builds and returns a k8s client or error if the client can't be configured
func NewClient() (client.Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	_ = addKnownTypes(scheme)
	c, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	return c, err
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&oav1alpha1.ManagedNotification{},
		&oav1alpha1.ManagedNotificationList{},
		&oav1alpha1.ManagedFleetNotification{},
		&oav1alpha1.ManagedFleetNotificationList{},
		&oav1alpha1.ManagedFleetNotificationRecord{},
		&oav1alpha1.ManagedFleetNotificationRecordList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
