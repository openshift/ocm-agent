module github.com/openshift/ocm-agent

go 1.16

require (
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/onsi/ginkgo/v2 v2.1.1
	github.com/onsi/gomega v1.18.1
	github.com/prometheus/alertmanager v0.23.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	k8s.io/apimachinery v0.22.1
	k8s.io/kubectl v0.23.3
	sigs.k8s.io/controller-runtime v0.10.0
)

replace k8s.io/kubectl => k8s.io/kubectl v0.22.1
