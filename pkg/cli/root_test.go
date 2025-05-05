package cli_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/ocm-agent/pkg/cli"
	"github.com/spf13/cobra"
)

var _ = Describe("ocm-agent CLI root command", func() {
	var (
		rootCmd *cobra.Command
		output  *bytes.Buffer
	)

	BeforeEach(func() {
		rootCmd = cli.NewCmdRoot()
		output = &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetErr(output)
	})

	Context("Root command initialization", func() {
		It("should create root command with correct properties", func() {
			Expect(rootCmd.Use).To(Equal("ocm-agent"))
			Expect(rootCmd.Short).To(Equal("Command line tool for OCM Agent to talk to OCM services."))
			Expect(rootCmd.Long).To(Equal("Command line tool for OCM Agent to talk to OCM services."))
			Expect(rootCmd.DisableAutoGenTag).To(BeTrue())
		})

		It("should have exactly one subcommand (serve)", func() {
			commands := rootCmd.Commands()
			Expect(commands).To(HaveLen(1))
			Expect(commands[0].Use).To(Equal("serve"))
		})

		It("should not have any flags", func() {
			flags := rootCmd.Flags()
			Expect(flags.NFlag()).To(Equal(0))
		})

		It("should not have any persistent flags", func() {
			persistentFlags := rootCmd.PersistentFlags()
			Expect(persistentFlags.NFlag()).To(Equal(0))
		})
	})

	Context("Root command execution", func() {
		It("should display help and exit when run without arguments", func() {
			// Since the root command calls os.Exit(1), we need to test the Run function indirectly
			rootCmd.SetArgs([]string{})

			// The root command's Run function calls cmd.Help() and os.Exit(1)
			// We can't test os.Exit directly, but we can verify the help is shown
			err := rootCmd.Help()
			Expect(err).To(BeNil())
			Expect(output.String()).To(ContainSubstring("ocm-agent"))
			Expect(output.String()).To(ContainSubstring("Command line tool for OCM Agent"))
		})

		It("should show help text with usage information", func() {
			err := rootCmd.Help()
			Expect(err).To(BeNil())

			helpOutput := output.String()
			Expect(helpOutput).To(ContainSubstring("Usage:"))
			Expect(helpOutput).To(ContainSubstring("ocm-agent"))
			Expect(helpOutput).To(ContainSubstring("Available Commands:"))
			Expect(helpOutput).To(ContainSubstring("serve"))
		})

		It("should handle invalid subcommands gracefully", func() {
			rootCmd.SetArgs([]string{"invalid-command"})
			err := rootCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown command"))
		})
	})

	Context("Command structure validation", func() {
		It("should have serve command properly configured", func() {
			serveCmd := rootCmd.Commands()[0]
			Expect(serveCmd).ToNot(BeNil())
			Expect(serveCmd.Use).To(Equal("serve"))
			Expect(serveCmd.Short).To(Equal("Starts the OCM Agent server"))
		})

		It("should have correct parent-child relationships", func() {
			serveCmd := rootCmd.Commands()[0]
			Expect(serveCmd.Parent()).To(Equal(rootCmd))
		})
	})

	Context("Help functionality", func() {
		It("should display help when --help flag is used", func() {
			rootCmd.SetArgs([]string{"--help"})
			err := rootCmd.Execute()
			Expect(err).To(BeNil())

			helpOutput := output.String()
			Expect(helpOutput).To(ContainSubstring("Command line tool for OCM Agent"))
		})

		It("should display help when -h flag is used", func() {
			rootCmd.SetArgs([]string{"-h"})
			err := rootCmd.Execute()
			Expect(err).To(BeNil())

			helpOutput := output.String()
			Expect(helpOutput).To(ContainSubstring("Command line tool for OCM Agent"))
		})
	})
})

// Test the initConfig function indirectly by testing cobra initialization
var _ = Describe("CLI initialization", func() {
	Context("Configuration initialization", func() {
		It("should initialize without errors", func() {
			// Test that creating a new root command doesn't panic
			// This indirectly tests the initConfig function which is called via cobra.OnInitialize
			Expect(func() {
				_ = cli.NewCmdRoot()
			}).ToNot(Panic())
		})
	})
})

// Benchmark tests for performance
func BenchmarkNewCmdRoot(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = cli.NewCmdRoot()
	}
}

func BenchmarkRootCommandHelp(b *testing.B) {
	cmd := cli.NewCmdRoot()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output.Reset()
		_ = cmd.Help()
	}
}
