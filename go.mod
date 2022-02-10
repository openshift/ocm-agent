module github.com/openshift/ocm-agent

go 1.16

require (
	github.com/gorilla/mux v1.8.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	k8s.io/kubectl v0.23.3
)

replace k8s.io/kubectl => k8s.io/kubectl v0.22.1
