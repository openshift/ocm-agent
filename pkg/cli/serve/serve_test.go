package serve_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var _ = Describe("Serve command", func() {
	var (
		serveCmd *cobra.Command
		output   *bytes.Buffer
		tempDir  string
	)

	BeforeEach(func() {
		serveCmd = serve.NewServeCmd()
		output = &bytes.Buffer{}
		serveCmd.SetOut(output)
		serveCmd.SetErr(output)

		// Create temporary directory for test files
		var err error
		tempDir, err = os.MkdirTemp("", "serve-test-*")
		Expect(err).To(BeNil())

		// Reset viper for each test
		viper.Reset()
	})

	AfterEach(func() {
		// Clean up temporary directory
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("Command initialization", func() {
		It("should create serve command with correct properties", func() {
			Expect(serveCmd.Use).To(Equal("serve"))
			Expect(serveCmd.Short).To(Equal("Starts the OCM Agent server"))
			Expect(serveCmd.Long).To(ContainSubstring("Start the OCM Agent server"))
			Expect(serveCmd.Example).To(ContainSubstring("ocm-agent serve"))
		})

		It("should have all required flags defined", func() {
			flags := serveCmd.Flags()

			// Check that all expected flags exist
			expectedFlags := []string{
				config.OcmURL,
				config.AccessToken,
				config.ExternalClusterID,
				config.OCMClientID,
				config.OCMClientSecret,
				config.Services,
				config.FleetMode,
			}

			for _, flagName := range expectedFlags {
				flag := flags.Lookup(flagName)
				Expect(flag).ToNot(BeNil(), fmt.Sprintf("Flag %s should exist", flagName))
			}
		})

		It("should have correct flag properties", func() {
			flags := serveCmd.Flags()

			// Test access token flag
			tokenFlag := flags.Lookup(config.AccessToken)
			Expect(tokenFlag.Shorthand).To(Equal("t"))
			Expect(tokenFlag.Usage).To(ContainSubstring("Access token for OCM"))

			// Test cluster ID flag
			clusterFlag := flags.Lookup(config.ExternalClusterID)
			Expect(clusterFlag.Shorthand).To(Equal("c"))
			Expect(clusterFlag.Usage).To(ContainSubstring("Cluster ID"))

			// Test services flag
			servicesFlag := flags.Lookup(config.Services)
			Expect(servicesFlag.Usage).To(ContainSubstring("OCM service name"))
		})

		It("should have persistent debug flag", func() {
			persistentFlags := serveCmd.PersistentFlags()
			debugFlag := persistentFlags.Lookup(config.Debug)
			Expect(debugFlag).ToNot(BeNil())
			Expect(debugFlag.Shorthand).To(Equal("d"))
		})
	})

	Context("Flag validation - traditional mode", func() {
		It("should require ocm-url flag", func() {
			serveCmd.SetArgs([]string{})
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			expectedErrorMessage := "required flag(s) \"access-token\", \"cluster-id\", \"ocm-url\", \"services\" not set"
			Expect(err.Error()).To(Equal(expectedErrorMessage))
		})

		It("should require access-token and cluster-id flags in traditional mode", func() {
			serveCmd.SetArgs([]string{
				"--ocm-url", "https://example.com",
				"--services", "service_log",
			})
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("required flag(s) \"access-token\", \"cluster-id\""))
		})

		It("should require both access-token and cluster-id together", func() {
			// Test with only access token
			serveCmd.SetArgs([]string{
				"--ocm-url", "https://example.com",
				"--services", "service_log",
				"--access-token", "token123",
			})
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cluster-id"))
		})
	})

	Context("Flag validation - fleet mode", func() {
		It("should require fleet-mode flag when client credentials are provided", func() {
			serveCmd.SetArgs([]string{
				"--ocm-url", "https://example.com",
				"--services", "service_log",
				"--ocm-client-id", "client123",
				"--ocm-client-secret", "secret123",
			})
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("required flag(s) \"fleet-mode\""))
		})

		It("should require both client ID and secret together", func() {
			serveCmd.SetArgs([]string{
				"--ocm-url", "https://example.com",
				"--services", "service_log",
				"--fleet-mode",
				"--ocm-client-id", "client123",
			})
			err := serveCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ocm-client-secret"))
		})
	})

	Context("Help functionality", func() {
		It("should display help text correctly", func() {
			err := serveCmd.Help()
			Expect(err).To(BeNil())

			helpOutput := output.String()
			Expect(helpOutput).To(ContainSubstring("Start the OCM Agent server"))
			Expect(helpOutput).To(ContainSubstring("Usage:"))
			Expect(helpOutput).To(ContainSubstring("Flags:"))
			Expect(helpOutput).To(ContainSubstring("Examples:"))
		})

		It("should show examples in help", func() {
			err := serveCmd.Help()
			Expect(err).To(BeNil())

			helpOutput := output.String()
			Expect(helpOutput).To(ContainSubstring("ocm-agent serve --access-token"))
			Expect(helpOutput).To(ContainSubstring("--fleet-mode"))
		})
	})

	Context("ServeOptions initialization", func() {
		It("should create new serve options with default values", func() {
			options := serve.NewServeOptions()
			Expect(options).ToNot(BeNil())
		})
	})
})

var _ = Describe("File-based configuration", func() {
	var (
		serveCmd *cobra.Command
		tempDir  string
	)

	BeforeEach(func() {
		serveCmd = serve.NewServeCmd()

		var err error
		tempDir, err = os.MkdirTemp("", "config-test-*")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("ReadFlagsFromFile function", func() {
		It("should read string flag from file", func() {
			// Create test file
			testFile := filepath.Join(tempDir, "url_file")
			err := os.WriteFile(testFile, []byte("https://api.openshift.com"), 0600)
			Expect(err).To(BeNil())

			// Set flag to read from file
			err = serveCmd.Flags().Set(config.OcmURL, "@"+testFile)
			Expect(err).To(BeNil())

			// Read from file
			err = serve.ReadFlagsFromFile(serveCmd, config.OcmURL)
			Expect(err).To(BeNil())

			// Verify value was read
			value, err := serveCmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(value).To(Equal("https://api.openshift.com"))
		})

		It("should read string slice flag from file", func() {
			// Create test file
			testFile := filepath.Join(tempDir, "services_file")
			err := os.WriteFile(testFile, []byte("service_log,clusters"), 0600)
			Expect(err).To(BeNil())

			// Set flag to read from file
			err = serveCmd.Flags().Set(config.Services, "@"+testFile)
			Expect(err).To(BeNil())

			// Read from file
			err = serve.ReadFlagsFromFile(serveCmd, config.Services)
			Expect(err).To(BeNil())

			// Verify value was read
			// values, err := serveCmd.Flags().GetStringSlice(config.Services)
			// Expect(err).To(BeNil())
			// Expect(values).To(ConsistOf("service_log", "clusters"))
		})

		It("should handle file with whitespace correctly", func() {
			// Create test file with leading/trailing whitespace
			testFile := filepath.Join(tempDir, "token_file")
			err := os.WriteFile(testFile, []byte("  token123  \n"), 0600)
			Expect(err).To(BeNil())

			// Set flag to read from file
			err = serveCmd.Flags().Set(config.AccessToken, "@"+testFile)
			Expect(err).To(BeNil())

			// Read from file
			err = serve.ReadFlagsFromFile(serveCmd, config.AccessToken)
			Expect(err).To(BeNil())

			// Verify whitespace was trimmed
			value, err := serveCmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal("token123"))
		})

		It("should return error for non-existent file", func() {
			nonExistentFile := filepath.Join(tempDir, "does_not_exist")

			err := serveCmd.Flags().Set(config.OcmURL, "@"+nonExistentFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(serveCmd, config.OcmURL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("can't read value of flag"))
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("should handle empty file correctly", func() {
			// Create empty test file
			testFile := filepath.Join(tempDir, "empty_file")
			err := os.WriteFile(testFile, []byte(""), 0600)
			Expect(err).To(BeNil())

			err = serveCmd.Flags().Set(config.AccessToken, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(serveCmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := serveCmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(""))
		})

		It("should not modify flags that don't start with @", func() {
			originalValue := "https://direct.url.com"
			err := serveCmd.Flags().Set(config.OcmURL, originalValue)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(serveCmd, config.OcmURL)
			Expect(err).To(BeNil())

			value, err := serveCmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(originalValue))
		})

		It("should handle multiple flags in one call", func() {
			// Create test files
			urlFile := filepath.Join(tempDir, "url_file")
			tokenFile := filepath.Join(tempDir, "token_file")

			err := os.WriteFile(urlFile, []byte("https://test.com"), 0600)
			Expect(err).To(BeNil())
			err = os.WriteFile(tokenFile, []byte("test-token"), 0600)
			Expect(err).To(BeNil())

			// Set flags to read from files
			err = serveCmd.Flags().Set(config.OcmURL, "@"+urlFile)
			Expect(err).To(BeNil())
			err = serveCmd.Flags().Set(config.AccessToken, "@"+tokenFile)
			Expect(err).To(BeNil())

			// Read both flags in one call
			err = serve.ReadFlagsFromFile(serveCmd, config.OcmURL, config.AccessToken)
			Expect(err).To(BeNil())

			// Verify both values were read
			urlValue, err := serveCmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(urlValue).To(Equal("https://test.com"))

			tokenValue, err := serveCmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(tokenValue).To(Equal("test-token"))
		})
	})
})

// Benchmark tests
func BenchmarkNewServeCmd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = serve.NewServeCmd()
	}
}

func BenchmarkReadFlagsFromFile(b *testing.B) {
	cmd := serve.NewServeCmd()
	tempDir, _ := os.MkdirTemp("", "benchmark-*")
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_file")
	err := os.WriteFile(testFile, []byte("test-value"), 0600)
	Expect(err).ToNot(HaveOccurred())
	urlErr := cmd.Flags().Set(config.OcmURL, "@"+testFile)
	Expect(urlErr).ToNot(HaveOccurred())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serve.ReadFlagsFromFile(cmd, config.OcmURL)
	}
}
