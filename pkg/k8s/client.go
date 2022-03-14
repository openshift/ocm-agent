package k8s

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClient builds and returns a k8s client or error if the client can't be configured
func NewClient() (*client.Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	c, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	return &c, err
}
