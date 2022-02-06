package main

import (
	"fmt"
	"os"

	"github.com/openshift/ocm-agent/pkg/cli"
)

func main() {

	// Execute the root command 'ocm-agent'
	command := cli.NewCmdRoot()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute command 'ocm-agent': %v\n", err)
		os.Exit(1)
	}

}
