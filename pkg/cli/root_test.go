package cli_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/ocm-agent/pkg/cli"
	"github.com/spf13/cobra"
)

var _ = Describe("ocm-agent cli", func() {
	var (
		rootCmd *cobra.Command
	)

	BeforeEach(func() {
		rootCmd = cli.NewCmdRoot()
	})

	Context("Test root cmd", func() {
		It("Check root command help ", func() {
			err := rootCmd.Help()
			Expect(err).To(BeNil())
		})

		It("Should has one subcommand ", func() {
			commands := rootCmd.Commands()
			Expect(commands).NotTo(BeNil())
			// Only has serve as first layer subcommand
			Expect(len(commands)).To(Equal(1))
		})
	})
})
