package serve_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/openshift/ocm-agent/pkg/config"
)

// TestNewServeCmd tests the creation and basic properties of the serve command
func TestNewServeCmd(t *testing.T) {
	cmd := serve.NewServeCmd()

	if cmd == nil {
		t.Fatal("NewServeCmd returned nil")
	} else {
		if cmd.Use != "serve" {
			t.Errorf("Expected command Use to be 'serve', got %s", cmd.Use)
		}

		if cmd.Short != "Starts the OCM Agent server" {
			t.Errorf("Expected command Short to be 'Starts the OCM Agent server', got %s", cmd.Short)
		}

		if !strings.Contains(cmd.Long, "Start the OCM Agent server") {
			t.Errorf("Expected command Long to contain 'Start the OCM Agent server', got %s", cmd.Long)
		}

		if !strings.Contains(cmd.Example, "ocm-agent serve") {
			t.Errorf("Expected command Example to contain 'ocm-agent serve', got %s", cmd.Example)
		}
	}
}

// TestServeCommandFlags tests all the flags defined for the serve command
func TestServeCommandFlags(t *testing.T) {
	cmd := serve.NewServeCmd()

	expectedFlags := []struct {
		name      string
		shorthand string
		usage     string
	}{
		{config.OcmURL, "", "OCM URL (string)"},
		{config.AccessToken, "t", "Access token for OCM (string)"},
		{config.ExternalClusterID, "c", "Cluster ID (string)"},
		{config.OCMClientID, "", "OCM Client ID for testing fleet mode (string)"},
		{config.OCMClientSecret, "", "OCM Client Secret for testing fleet mode (string)"},
		{config.Services, "", "OCM service name (string)"},
		{config.FleetMode, "", "Fleet Mode (bool)"},
		{config.Debug, "d", "Debug mode enable"},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("Flag %s should exist", expected.name)
			continue
		}

		if expected.shorthand != "" && flag.Shorthand != expected.shorthand {
			t.Errorf("Flag %s shorthand should be %s, got %s", expected.name, expected.shorthand, flag.Shorthand)
		}

		if !strings.Contains(flag.Usage, strings.Split(expected.usage, " ")[0]) {
			t.Errorf("Flag %s usage should contain %s, got %s", expected.name, expected.usage, flag.Usage)
		}
	}
}

// TestServeCommandStructure tests the command structure and relationships
func TestServeCommandStructure(t *testing.T) {
	cmd := serve.NewServeCmd()

	if cmd.Run == nil {
		t.Error("Command should have Run function")
	}

	if cmd.RunE != nil {
		t.Error("Command should use Run, not RunE")
	}

	if cmd.PreRun == nil {
		t.Error("Command should have PreRun function for validation")
	}
}

// TestNewServeOptions tests the ServeOptions creation
func TestNewServeOptions(t *testing.T) {
	options := serve.NewServeOptions()
	if options == nil {
		t.Fatal("NewServeOptions returned nil")
	}
}

// TestReadFlagsFromFileString tests reading string flags from files
func TestReadFlagsFromFileString(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test_value")
	content := "test-string-value"
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+testFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	if value != content {
		t.Errorf("Expected value %s, got %s", content, value)
	}
}

// TestReadFlagsFromFileStringSlice tests reading string slice flags from files
func TestReadFlagsFromFileStringSlice(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test_services")
	content := "service_log,clusters,upgrade_policies"
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.Services, "@"+testFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.Services)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	values, err := cmd.Flags().GetStringSlice(config.Services)
	if err != nil {
		t.Fatal(err)
	}

	expectedServices := []string{"service_log", "clusters", "upgrade_policies"}
	if !containsAllElements(values, expectedServices) {
		t.Errorf("Expected services %v, got %v", expectedServices, values)
	}
}

// TestReadFlagsFromFileWhitespace tests whitespace handling in file content
func TestReadFlagsFromFileWhitespace(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file with whitespace
	testFile := filepath.Join(tempDir, "test_whitespace")
	content := "  token-with-whitespace  \n"
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+testFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	expected := "token-with-whitespace"
	if value != expected {
		t.Errorf("Expected whitespace-trimmed value %s, got %s", expected, value)
	}
}

// TestReadFlagsFromFileError tests error handling for non-existent files
func TestReadFlagsFromFileError(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	nonExistentFile := filepath.Join(tempDir, "does_not_exist")

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+nonExistentFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "can't read value of flag") {
		t.Errorf("Expected error to contain 'can't read value of flag', got %s", err.Error())
	}
}

// TestReadFlagsFromFileNoPrefix tests that flags without @ prefix are not modified
func TestReadFlagsFromFileNoPrefix(t *testing.T) {
	cmd := serve.NewServeCmd()
	originalValue := "https://direct.url.com"

	err := cmd.Flags().Set(config.OcmURL, originalValue)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.OcmURL)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.OcmURL)
	if err != nil {
		t.Fatal(err)
	}

	if value != originalValue {
		t.Errorf("Expected unchanged value %s, got %s", originalValue, value)
	}
}

// TestReadFlagsFromFileMultipleFlags tests reading multiple flags in one call
func TestReadFlagsFromFileMultipleFlags(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	urlFile := filepath.Join(tempDir, "url_file")
	tokenFile := filepath.Join(tempDir, "token_file")

	err = os.WriteFile(urlFile, []byte("https://test.com"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tokenFile, []byte("test-token"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.OcmURL, "@"+urlFile)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Flags().Set(config.AccessToken, "@"+tokenFile)
	if err != nil {
		t.Fatal(err)
	}

	// Read both flags in one call
	err = serve.ReadFlagsFromFile(cmd, config.OcmURL, config.AccessToken)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	// Verify both values were read
	urlValue, err := cmd.Flags().GetString(config.OcmURL)
	if err != nil {
		t.Fatal(err)
	}
	if urlValue != "https://test.com" {
		t.Errorf("Expected URL https://test.com, got %s", urlValue)
	}

	tokenValue, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if tokenValue != "test-token" {
		t.Errorf("Expected token test-token, got %s", tokenValue)
	}
}

// TestHelpFunctionality tests the help functionality
func TestHelpFunctionality(t *testing.T) {
	cmd := serve.NewServeCmd()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	err := cmd.Help()
	if err != nil {
		t.Fatalf("Help failed: %v", err)
	}

	helpOutput := output.String()
	expectedStrings := []string{
		"Start the OCM Agent server",
		"Usage:",
		"Flags:",
		"Examples:",
		"ocm-agent serve --access-token",
		"--fleet-mode",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(helpOutput, expected) {
			t.Errorf("Expected help output to contain %s", expected)
		}
	}
}

// TestAdvancedStringSliceHandling tests complex CSV values from files
func TestAdvancedStringSliceHandling(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "complex_services")
	content := "service_log,clusters,upgrade_policies"
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.Services, "@"+testFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.Services)
	if err != nil {
		t.Fatal(err)
	}

	values, err := cmd.Flags().GetStringSlice(config.Services)
	if err != nil {
		t.Fatal(err)
	}

	if len(values) == 0 {
		t.Error("Expected non-empty services slice")
	}
}

// TestFileSystemEdgeCases tests various file system scenarios
func TestFileSystemEdgeCases(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Test empty file
	emptyFile := filepath.Join(tempDir, "empty")
	err = os.WriteFile(emptyFile, []byte(""), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+emptyFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	value, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	if value != "" {
		t.Errorf("Expected empty string for empty file, got %s", value)
	}

	// Test file with only whitespace
	whitespaceFile := filepath.Join(tempDir, "whitespace")
	err = os.WriteFile(whitespaceFile, []byte("   \n\t  \n  "), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd2 := serve.NewServeCmd()
	err = cmd2.Flags().Set(config.ExternalClusterID, "@"+whitespaceFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd2, config.ExternalClusterID)
	if err != nil {
		t.Fatal(err)
	}

	value, err = cmd2.Flags().GetString(config.ExternalClusterID)
	if err != nil {
		t.Fatal(err)
	}

	if value != "" {
		t.Errorf("Expected empty string for whitespace-only file, got %s", value)
	}
}

// TestDifferentLineEndings tests files with different line ending formats
func TestDifferentLineEndings(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Test Unix line endings
	unixFile := filepath.Join(tempDir, "unix_ending")
	err = os.WriteFile(unixFile, []byte("unix\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+unixFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	value, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	if value != "unix" {
		t.Errorf("Expected 'unix', got %s", value)
	}

	// Test Windows line endings
	windowsFile := filepath.Join(tempDir, "windows_ending")
	err = os.WriteFile(windowsFile, []byte("windows\r\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd2 := serve.NewServeCmd()
	err = cmd2.Flags().Set(config.OcmURL, "@"+windowsFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd2, config.OcmURL)
	if err != nil {
		t.Fatal(err)
	}

	value, err = cmd2.Flags().GetString(config.OcmURL)
	if err != nil {
		t.Fatal(err)
	}

	if value != "windows" {
		t.Errorf("Expected 'windows', got %s", value)
	}
}

// Helper functions

// contains checks if a slice contains a specific element
func contains(slice []string, element string) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

// containsAllElements checks if a slice contains all elements from another slice
func containsAllElements(slice []string, elements []string) bool {
	for _, element := range elements {
		if !contains(slice, element) {
			return false
		}
	}
	return true
}

// Benchmark tests
func BenchmarkNewServeCmd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = serve.NewServeCmd()
	}
}

func BenchmarkReadFlagsFromFile(b *testing.B) {
	// Setup
	tempDir, _ := os.MkdirTemp("", "benchmark-*")
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_file")
	_ = os.WriteFile(testFile, []byte("test-value"), 0600)

	cmd := serve.NewServeCmd()
	_ = cmd.Flags().Set(config.OcmURL, "@"+testFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serve.ReadFlagsFromFile(cmd, config.OcmURL)
	}
}

// TestCompleteMethodDirect tests the Complete method directly
func TestCompleteMethodDirect(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	urlFile := filepath.Join(tempDir, "url_file")
	tokenFile := filepath.Join(tempDir, "token_file")
	servicesFile := filepath.Join(tempDir, "services_file")
	clusterFile := filepath.Join(tempDir, "cluster_file")

	err = os.WriteFile(urlFile, []byte("https://test-complete.com"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tokenFile, []byte("test-token-complete"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(servicesFile, []byte("service_log,clusters"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(clusterFile, []byte("test-cluster-complete"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Create command and set flags
	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.OcmURL, "@"+urlFile)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Flags().Set(config.AccessToken, "@"+tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Flags().Set(config.Services, "@"+servicesFile)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Flags().Set(config.ExternalClusterID, "@"+clusterFile)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Flags().Set(config.Debug, "true")
	if err != nil {
		t.Fatal(err)
	}

	// Create serveOptions instance and call Complete method directly
	options := serve.NewServeOptions()
	err = options.Complete(cmd, []string{})
	if err != nil {
		t.Fatalf("Complete method failed: %v", err)
	}

	// Verify that Complete method processed the flags correctly
	urlValue, _ := cmd.Flags().GetString(config.OcmURL)
	if urlValue != "https://test-complete.com" {
		t.Errorf("Complete method: Expected URL https://test-complete.com, got %s", urlValue)
	}

	tokenValue, _ := cmd.Flags().GetString(config.AccessToken)
	if tokenValue != "test-token-complete" {
		t.Errorf("Complete method: Expected token test-token-complete, got %s", tokenValue)
	}

	servicesValue, _ := cmd.Flags().GetStringSlice(config.Services)
	expectedServices := []string{"service_log", "clusters"}
	if !containsAllElements(servicesValue, expectedServices) {
		t.Errorf("Complete method: Expected services %v, got %v", expectedServices, servicesValue)
	}

	clusterValue, _ := cmd.Flags().GetString(config.ExternalClusterID)
	if clusterValue != "test-cluster-complete" {
		t.Errorf("Complete method: Expected cluster test-cluster-complete, got %s", clusterValue)
	}

	debugValue, _ := cmd.Flags().GetBool(config.Debug)
	if !debugValue {
		t.Error("Complete method: Expected debug to be true")
	}
}

// TestCompleteMethodDirectErrorHandling tests Complete method error handling
func TestCompleteMethodDirectErrorHandling(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Reference non-existent file
	nonExistentFile := filepath.Join(tempDir, "does_not_exist")

	// Create command and set flag with non-existent file
	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+nonExistentFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create serveOptions instance and call Complete method directly
	options := serve.NewServeOptions()
	err = options.Complete(cmd, []string{})

	// Should return error for non-existent file
	if err == nil {
		t.Error("Complete method should return error for non-existent file")
	}
	if !strings.Contains(err.Error(), "can't read value of flag") {
		t.Errorf("Complete method: Expected error to contain 'can't read value of flag', got %s", err.Error())
	}
}

// TestCompleteMethodDirectServicesSliceProcessing tests Complete method services slice processing
func TestCompleteMethodDirectServicesSliceProcessing(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "serve-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create services file
	servicesFile := filepath.Join(tempDir, "services_file")
	err = os.WriteFile(servicesFile, []byte("service_log,clusters,upgrade_policies"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Create command and set services flag with file reference
	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.Services, "@"+servicesFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create serveOptions instance and call Complete method directly
	options := serve.NewServeOptions()
	err = options.Complete(cmd, []string{})
	if err != nil {
		t.Fatalf("Complete method failed: %v", err)
	}

	// Verify services were processed correctly
	servicesValue, _ := cmd.Flags().GetStringSlice(config.Services)
	expectedServices := []string{"service_log", "clusters", "upgrade_policies"}
	if !containsAllElements(servicesValue, expectedServices) {
		t.Errorf("Complete method: Expected services %v, got %v", expectedServices, servicesValue)
	}
}
