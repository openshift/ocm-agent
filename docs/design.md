# OCM Agent Design

## Pcakages

### net/http

`net/http` is a built-in framework in golang that handles development of web services.

OCM Agent is built and run using `net/http` package as web-service deployemt managed by OAO.

Reference : [net/http](https://pkg.go.dev/net/http)

### gorrilla/mux

Package `gorilla/mux` implements a request router and dispatcher for matching incoming requests to their respective handler.

OCM Agent also additionally uses `gorilla/mux` package along `net/http` to manage request router & dispatcher.

Reference : [gorilla/mux](https://github.com/gorilla/mux)

### Cobra

Package `Cobra` is a library providing an interface to CLI interfaces around golang applications.

OCM Agent uses `Cobra` package to create the ocm-agent CLI.

Reference : [Cobra](https://github.com/spf13/cobra)

### pflag

Package `pflag` is a drop-in replacement for Go's flag package, implementing POSIX/GNU-style --flags.

OCM Agent uses `pflag` package along with `Cobra` to create and implement flags around ocm-agent CLI.

Reference : [pflag](https://github.com/spf13/pflag)

### Viper

Package `Viper` is a complete configuration solution for Go applications and it is designed to work within an application, and can handle all types of configuration needs and formats.

Reference : [Viper](https://github.com/spf13/viper)

## Controllers

Controllers are important part of OCM Agent Operator.

### OCMAgent Controller

The [OCMAgent Controller](https://github.com/openshift/ocm-agent-operator/tree/master/pkg/controller/ocmagent/ocmagent_controller.go) is responsible for ensuring the deployment or removal of an OCM Agent based upon the presence of an `OCMAgent` Custom Resource.

More info on OCMAget Controller in OAO docs :
[OCMAget Controller](https://github.com/openshift/ocm-agent-operator/blob/master/docs/design.md#ocmagent-controller)

## OCM Agent

### handlers

Handlers or a collection of handlers (aka "HTTP Middleware") are used to handle web-service requests within a golang application implemented using net/http package.

### routers

A router or request router and dispatcher match incoming requests towards any respective handler within a web service application implemented using gorilla/mux package.

### Services

Services deployed and managed by OCM Agent handling their respective requests for defined use-cases.
e.g. A service log to be sent to a cluster with appropraite managed-notifications template against a particular alert.

## OCM Agent CLI

OCM Agent has built in CLI to have the ability to read and process instructions and configuration items via the command-line in a standardized way.

Reference : [CLI-usage.md](CLI-usage.md)
