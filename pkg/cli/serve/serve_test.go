package serve_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/spf13/cobra"
)

var _ = Describe("Serve subcommands", func() {

	var (
		serveCmd *cobra.Command
	)

	BeforeEach(func() {
		serveCmd = serve.NewServeCmd()
	})

	Context("Test serve cmd", func() {
		It("Check root command help ", func() {
			err := serveCmd.Help()
			Expect(err).To(BeNil())
		})

		It("Execute without required parameters will return error ", func() {
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			expectedErrorMessage := "required flag(s) \"access-token\", \"cluster-id\", \"ocm-url\", \"services\" not set"
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})

		It("Execute fleetmode without required parameters will return error ", func() {
			flagErr := serveCmd.Flags().Set("ocm-client-id", "BD845DE4-5C16-4067-A868-15B02D55CCEF")
			Expect(flagErr).To(BeNil())
			flagErr = serveCmd.Flags().Set("ocm-url", "https://sample.example.com")
			Expect(flagErr).To(BeNil())
			flagErr = serveCmd.Flags().Set("services", "service_log")
			Expect(flagErr).To(BeNil())
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			expectedErrorMessage := "required flag(s) \"fleet-mode\" not set"
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})
	})

	Context("Config read from file", func() {
		It("Check string type ", func() {
			flagErr := serveCmd.Flag("ocm-url").Value.Set("@url_file")
			Expect(flagErr).To(BeNil())
			err := serve.ReadFlagsFromFile(serveCmd, config.OcmURL)
			Expect(err).To(BeNil())
		})

		It("Return error when failed to read file ", func() {
			flagErr := serveCmd.Flag("ocm-url").Value.Set("@urlfile")
			Expect(flagErr).To(BeNil())
			err := serve.ReadFlagsFromFile(serveCmd, config.OcmURL)
			Expect(err).To(HaveOccurred())
			expectedErrorMessage := "can't read value of flag 'ocm-url' from file 'urlfile': open urlfile: no such file or directory"
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})
	})
})
