module github.com/openshift/ocm-agent

go 1.16

require (
	github.com/golang/mock v1.6.0
	github.com/gorilla/mux v1.8.0
	github.com/onsi/ginkgo/v2 v2.1.1
	github.com/onsi/gomega v1.18.1
	github.com/openshift-online/ocm-cli v0.1.62
	github.com/openshift-online/ocm-sdk-go v0.1.242
	github.com/openshift/ocm-agent-operator v0.0.0-20220316071606-afd26cc5ca68
	github.com/prometheus/alertmanager v0.23.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kubectl v0.23.3
	sigs.k8s.io/controller-runtime v0.10.0
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20210910062324-a41d3573a3ba
	k8s.io/api => k8s.io/api v0.21.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.1
	k8s.io/client-go => k8s.io/client-go v0.21.1
	k8s.io/kubectl => k8s.io/kubectl v0.21.1
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.10.0
)
