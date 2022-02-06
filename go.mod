module github.com/openshift/ocm-agent

go 1.16

require (
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/onsi/gomega v1.18.1 // indirect
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	k8s.io/kubectl v0.23.3
)

replace (
	k8s.io/component-base => k8s.io/component-base v0.22.1
	k8s.io/kubectl => k8s.io/kubectl v0.22.1
)
