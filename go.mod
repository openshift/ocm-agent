module github.com/openshift/ocm-agent

go 1.16

require (
	github.com/gorilla/mux v1.8.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/ginkgo/v2 v2.1.1 // indirect
	github.com/onsi/gomega v1.18.1
	github.com/prometheus/alertmanager v0.23.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	k8s.io/kubectl v0.23.3
)

replace k8s.io/kubectl => k8s.io/kubectl v0.22.1
