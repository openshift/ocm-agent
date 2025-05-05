package serve_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/spf13/cobra"
)

var _ = Describe("Configuration file handling", func() {
	var (
		cmd     *cobra.Command
		tempDir string
		stderr  *bytes.Buffer
	)

	BeforeEach(func() {
		cmd = serve.NewServeCmd()

		var err error
		tempDir, err = os.MkdirTemp("", "config-test-*")
		Expect(err).To(BeNil())

		// Capture stderr for testing os.Exit scenarios
		stderr = &bytes.Buffer{}
		cmd.SetErr(stderr)
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("String flag file reading", func() {
		It("should read simple string value from file", func() {
			testFile := filepath.Join(tempDir, "simple_string")
			content := "simple-value"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(content))
		})

		It("should handle multiline content by trimming whitespace", func() {
			testFile := filepath.Join(tempDir, "multiline_string")
			content := "line1\nline2\nline3"
			err := os.WriteFile(testFile, []byte("  "+content+"  \n"), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.OcmURL, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.OcmURL)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(content))
		})

		It("should handle special characters in file content", func() {
			testFile := filepath.Join(tempDir, "special_chars")
			content := "!@#$%^&*()_+-={}[]|\\:;\"'<>?,./"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(content))
		})

		It("should handle Unicode content", func() {
			testFile := filepath.Join(tempDir, "unicode_content")
			content := "测试内容 🚀 émoji"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.ExternalClusterID, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.ExternalClusterID)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.ExternalClusterID)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(content))
		})
	})

	Context("String slice flag file reading", func() {
		It("should read comma-separated values from file", func() {
			testFile := filepath.Join(tempDir, "services_list")
			content := "service_log,clusters,upgrade_policies"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.Services, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.Services)
			Expect(err).To(BeNil())

			values, err := cmd.Flags().GetStringSlice(config.Services)
			Expect(err).To(BeNil())
			Expect(values).To(ContainElements("service_log", "clusters", "upgrade_policies"))

		})

		It("should handle single value in string slice", func() {
			testFile := filepath.Join(tempDir, "single_service")
			content := "service_log"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.Services, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.Services)
			Expect(err).To(BeNil())

			values, err := cmd.Flags().GetStringSlice(config.Services)
			Expect(err).To(BeNil())
			Expect(values).To(ContainElements("service_log"))
		})

		It("should handle values with spaces", func() {
			testFile := filepath.Join(tempDir, "services_with_spaces")
			content := "service log,cluster management"
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.Services, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.Services)
			Expect(err).To(BeNil())

			values, err := cmd.Flags().GetStringSlice(config.Services)
			Expect(err).To(BeNil())
			Expect(values).To(ContainElements("service log", "cluster management"))
		})

		It("should trim whitespace from string slice content", func() {
			testFile := filepath.Join(tempDir, "services_with_whitespace")
			content := "  service_log , clusters  , upgrade_policies  "
			err := os.WriteFile(testFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.Services, "@"+testFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.Services)
			Expect(err).To(BeNil())

			values, err := cmd.Flags().GetStringSlice(config.Services)
			Expect(err).To(BeNil())
			Expect(values).To(ContainElements("service_log ", " clusters  ", " upgrade_policies"))
		})
	})

	Context("Error handling", func() {
		It("should return descriptive error for non-existent file (string)", func() {
			nonExistentFile := filepath.Join(tempDir, "does_not_exist")

			err := cmd.Flags().Set(config.AccessToken, "@"+nonExistentFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("can't read value of flag 'access-token'"))
			Expect(err.Error()).To(ContainSubstring("from file"))
			Expect(err.Error()).To(ContainSubstring("does_not_exist"))
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("should return descriptive error for non-existent file (string slice)", func() {
			nonExistentFile := filepath.Join(tempDir, "missing_services")

			err := cmd.Flags().Set(config.Services, "@"+nonExistentFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.Services)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("can't read value of flag 'services'"))
			Expect(err.Error()).To(ContainSubstring("from file"))
		})

		It("should handle directory instead of file", func() {
			dirPath := filepath.Join(tempDir, "directory")
			err := os.Mkdir(dirPath, 0755)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+dirPath)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("can't read value of flag"))
		})
	})

	Context("Edge cases", func() {
		It("should handle empty file", func() {
			emptyFile := filepath.Join(tempDir, "empty")
			err := os.WriteFile(emptyFile, []byte(""), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+emptyFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(""))
		})

		It("should handle file with only whitespace", func() {
			whitespaceFile := filepath.Join(tempDir, "whitespace")
			err := os.WriteFile(whitespaceFile, []byte("   \n\t  \n  "), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.ExternalClusterID, "@"+whitespaceFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.ExternalClusterID)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.ExternalClusterID)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(""))
		})

		It("should handle file with null bytes", func() {
			nullFile := filepath.Join(tempDir, "null_bytes")
			content := []byte("content\x00with\x00nulls")
			err := os.WriteFile(nullFile, content, 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+nullFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal("content\x00with\x00nulls"))
		})

		It("should handle very large files", func() {
			largeFile := filepath.Join(tempDir, "large")
			content := strings.Repeat("a", 10000) // 10KB content
			err := os.WriteFile(largeFile, []byte(content), 0600)
			Expect(err).To(BeNil())

			err = cmd.Flags().Set(config.AccessToken, "@"+largeFile)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(content))
		})

		It("should not process flags that don't start with @", func() {
			regularValue := "regular-value-not-from-file"
			err := cmd.Flags().Set(config.AccessToken, regularValue)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(regularValue))
		})

		It("should handle @ symbol in the middle of value", func() {
			valueWithAt := "user@domain.com"
			err := cmd.Flags().Set(config.AccessToken, valueWithAt)
			Expect(err).To(BeNil())

			err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
			Expect(err).To(BeNil())

			value, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(valueWithAt))
		})
	})

	Context("Multiple flag processing", func() {
		It("should process multiple flags with mix of file and direct values", func() {
			// Create test file
			urlFile := filepath.Join(tempDir, "url")
			err := os.WriteFile(urlFile, []byte("https://from-file.com"), 0600)
			Expect(err).To(BeNil())

			// Set flags: one from file, one direct
			err = cmd.Flags().Set(config.OcmURL, "@"+urlFile)
			Expect(err).To(BeNil())
			err = cmd.Flags().Set(config.AccessToken, "direct-token")
			Expect(err).To(BeNil())

			// Process both flags
			err = serve.ReadFlagsFromFile(cmd, config.OcmURL, config.AccessToken)
			Expect(err).To(BeNil())

			// Verify values
			urlValue, err := cmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(urlValue).To(Equal("https://from-file.com"))

			tokenValue, err := cmd.Flags().GetString(config.AccessToken)
			Expect(err).To(BeNil())
			Expect(tokenValue).To(Equal("direct-token"))
		})

		It("should stop processing on first error", func() {
			// Create one valid file
			validFile := filepath.Join(tempDir, "valid")
			err := os.WriteFile(validFile, []byte("valid-content"), 0600)
			Expect(err).To(BeNil())

			// Set flags: one valid file, one invalid
			err = cmd.Flags().Set(config.OcmURL, "@"+validFile)
			Expect(err).To(BeNil())
			err = cmd.Flags().Set(config.AccessToken, "@invalid-file")
			Expect(err).To(BeNil())

			// Process both flags - should fail on the invalid one
			err = serve.ReadFlagsFromFile(cmd, config.OcmURL, config.AccessToken)
			Expect(err).To(HaveOccurred())

			// The valid flag should still be processed
			urlValue, err := cmd.Flags().GetString(config.OcmURL)
			Expect(err).To(BeNil())
			Expect(urlValue).To(Equal("valid-content"))
		})
	})
})
