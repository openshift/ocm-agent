package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewCmdRoot initialises the root command 'ocm-agent'
func NewCmdRoot() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "ocm-agent",
		Short:             "Command line tool for OCM Agent to talk to OCM services.",
		Long:              "Command line tool for OCM Agent to talk to OCM services.",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	// Add subcommands
	rootCmd.AddCommand(serve.NewServeCmd())

	return rootCmd
}

// initConfig initialise a new flagset for root command and parses the command line arguments
func initConfig() {

	flags := pflag.NewFlagSet("ocm-agent", pflag.ExitOnError)
	err := flag.CommandLine.Parse([]string{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse empty command line: %v", err)
		os.Exit(1)
	}

	pflag.CommandLine = flags
}

func init() {
	cobra.OnInitialize(initConfig)
}
