<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [OCM Agent CLI Usage](#ocm-agent-cli-usage)
  - [OCM Agent CLI](#ocm-agent-cli)
  - [CLI Usage](#cli-usage)
    - [Command "completion" - To generate auto-completion script for different shells](#command-completion---to-generate-auto-completion-script-for-different-shells)
    - [Command "serve" - To start the OCM Agent server](#command-serve---to-start-the-ocm-agent-server)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# OCM Agent CLI Usage

## OCM Agent CLI

OCM Agent has built in CLI to have the ability to read and process instructions and configuration items via the command-line in a standardized way.

`ocm-agent` CLI is available to use within the ocm-agent pod running on Openshift Dedicated V4 clusters.

To use `ocm-agent` CLI, log in to your OSD cluster and rsh into the ocm-agent pod under the openshift-ocm-agent-operator project.

## CLI Usage

```shell
$ ocm-agent 
Command line tool for OCM Agent to talk to OCM services.

Usage:
  ocm-agent [flags]
  ocm-agent [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  serve       Starts the OCM Agent server

Flags:
  -h, --help   help for ocm-agent

Use "ocm-agent [command] --help" for more information about a command
```

### Command "completion" - To generate auto-completion script for different shells

```shell
$ ocm-agent completion
Generate the autocompletion script for ocm-agent for the specified shell.
See each sub-command's help for details on how to use the generated script.

Usage:
  ocm-agent completion [command]

Available Commands:
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh

Flags:
  -h, --help   help for completion

Use "ocm-agent completion [command] --help" for more information about a command.
```

### Command "serve" - To start the OCM Agent server

```shell
$ ocm-agent serve
Usage:
  ocm-agent serve [flags]

Examples:
  # Start the OCM agent server in traditional OSD/ROSA mode
  ocm-agent serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com" --cluster-id abcd-1234
  
  # Start the OCM agent server in traditional OSD/ROSA mode by accepting token from a file (value starting with '@' is considered a file)
  ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile
  
  # Start the OCM agent server in traditional OSD/ROSA in debug mode
  ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile --debug

  # Start OCM agent server in fleet mode on production clusters
  ocm-agent serve --fleet-mode --services "$SERVICE" --ocm-url @urlfile
 
  # Start OCM agent server in fleet mode on staging clusters (in development/testing mode)
  ocm-agent serve --services $SERVICE --ocm-url $URL --fleet-mode --ocm-client-id $CLIENT_ID --ocm-client-secret $CLIENT_SECRET

Flags:
  -t, --access-token string        Access token for OCM (string)
  -c, --cluster-id string          Cluster ID (string)
  -d, --debug                      Debug mode enable
      --fleet-mode                 Fleet Mode (bool)
  -h, --help                       help for serve
      --ocm-client-id string       OCM Client ID for testing fleet mode (string)
      --ocm-client-secret string   OCM Client Secret for testing fleet mode (string)
      --ocm-url string             OCM URL (string)
      --services string            OCM service name (string)
```
