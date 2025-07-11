package serve_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/ocm-agent/pkg/cli/serve"
	"github.com/openshift/ocm-agent/pkg/config"
)

// TestReadFlagsFromFileSimpleString tests reading simple string values from files
func TestReadFlagsFromFileSimpleString(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "simple_string")
	content := "simple-value"
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

// TestReadFlagsFromFileMultilineContent tests multiline content handling
func TestReadFlagsFromFileMultilineContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "multiline_string")
	content := "line1\nline2\nline3"
	err = os.WriteFile(testFile, []byte("  "+content+"  \n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.OcmURL, "@"+testFile)
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
	if value != content {
		t.Errorf("Expected value %s, got %s", content, value)
	}
}

// TestReadFlagsFromFileSpecialCharacters tests special characters in file content
func TestReadFlagsFromFileSpecialCharacters(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "special_chars")
	content := "!@#$%^&*()_+-={}[]|\\:;\"'<>?,./"
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

// TestReadFlagsFromFileUnicodeContent tests Unicode content handling
func TestReadFlagsFromFileUnicodeContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "unicode_content")
	content := "测试内容 🚀 émoji"
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.ExternalClusterID, "@"+testFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.ExternalClusterID)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.ExternalClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if value != content {
		t.Errorf("Expected value %s, got %s", content, value)
	}
}

// TestReadFlagsFromFileStringSliceCSV tests comma-separated values from files
func TestReadFlagsFromFileStringSliceCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "services_list")
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

// TestReadFlagsFromFileStringSliceSingle tests single value in string slice
func TestReadFlagsFromFileStringSliceSingle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "single_service")
	content := "service_log"
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

	if !contains(values, "service_log") {
		t.Errorf("Expected services to contain service_log, got %v", values)
	}
}

// TestReadFlagsFromFileStringSliceSpaces tests values with spaces
func TestReadFlagsFromFileStringSliceSpaces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "services_with_spaces")
	content := "service log,cluster management"
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

	expectedServices := []string{"service log", "cluster management"}
	if !containsAllElements(values, expectedServices) {
		t.Errorf("Expected services %v, got %v", expectedServices, values)
	}
}

// TestReadFlagsFromFileNonExistentFile tests error handling for non-existent files
func TestReadFlagsFromFileNonExistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
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
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "can't read value of flag 'access-token'") {
		t.Errorf("Expected error about access-token flag, got %s", err.Error())
	}
	if !strings.Contains(err.Error(), "from file") {
		t.Errorf("Expected error to mention 'from file', got %s", err.Error())
	}
}

// TestReadFlagsFromFileDirectory tests handling directory instead of file
func TestReadFlagsFromFileDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dirPath := filepath.Join(tempDir, "directory")
	err = os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+dirPath)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.AccessToken)
	if err == nil {
		t.Error("Expected error for directory instead of file")
	}
	if !strings.Contains(err.Error(), "can't read value of flag") {
		t.Errorf("Expected error about flag reading, got %s", err.Error())
	}
}

// TestReadFlagsFromFileEmptyFile tests handling empty files
func TestReadFlagsFromFileEmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

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
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Errorf("Expected empty string, got %s", value)
	}
}

// TestReadFlagsFromFileWhitespaceOnly tests files with only whitespace
func TestReadFlagsFromFileWhitespaceOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	whitespaceFile := filepath.Join(tempDir, "whitespace")
	err = os.WriteFile(whitespaceFile, []byte("   \n\t  \n  "), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.ExternalClusterID, "@"+whitespaceFile)
	if err != nil {
		t.Fatal(err)
	}

	err = serve.ReadFlagsFromFile(cmd, config.ExternalClusterID)
	if err != nil {
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err := cmd.Flags().GetString(config.ExternalClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Errorf("Expected empty string, got %s", value)
	}
}

// TestReadFlagsFromFileNullBytes tests files with null bytes
func TestReadFlagsFromFileNullBytes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	nullFile := filepath.Join(tempDir, "null_bytes")
	content := []byte("content\x00with\x00nulls")
	err = os.WriteFile(nullFile, content, 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+nullFile)
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
	if value != "content\x00with\x00nulls" {
		t.Errorf("Expected value with null bytes, got %s", value)
	}
}

// TestReadFlagsFromFileLargeFiles tests handling very large files
func TestReadFlagsFromFileLargeFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	largeFile := filepath.Join(tempDir, "large")
	content := strings.Repeat("a", 10000) // 10KB content
	err = os.WriteFile(largeFile, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+largeFile)
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
		t.Errorf("Expected large content, got different value")
	}
}

// TestReadFlagsFromFileNoAtPrefix tests flags without @ prefix
func TestReadFlagsFromFileNoAtPrefix(t *testing.T) {
	cmd := serve.NewServeCmd()
	regularValue := "regular-value-not-from-file"
	err := cmd.Flags().Set(config.AccessToken, regularValue)
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
	if value != regularValue {
		t.Errorf("Expected unchanged value %s, got %s", regularValue, value)
	}
}

// TestReadFlagsFromFileAtInValue tests @ symbol in the middle of value
func TestReadFlagsFromFileAtInValue(t *testing.T) {
	cmd := serve.NewServeCmd()
	valueWithAt := "user@domain.com"
	err := cmd.Flags().Set(config.AccessToken, valueWithAt)
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
	if value != valueWithAt {
		t.Errorf("Expected unchanged value %s, got %s", valueWithAt, value)
	}
}

// TestReadFlagsFromFileLineEndings tests different line endings
func TestReadFlagsFromFileLineEndings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
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
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
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
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	value, err = cmd2.Flags().GetString(config.OcmURL)
	if err != nil {
		t.Fatal(err)
	}
	if value != "windows" {
		t.Errorf("Expected 'windows', got %s", value)
	}
}

// TestReadFlagsFromFileSymlinks tests symbolic links
func TestReadFlagsFromFileSymlinks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("symlink-content"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Create symbolic link
	linkFile := filepath.Join(tempDir, "link")
	err = os.Symlink(targetFile, linkFile)
	if err != nil {
		t.Skip("Symlinks not supported on this system")
	}

	cmd := serve.NewServeCmd()
	err = cmd.Flags().Set(config.AccessToken, "@"+linkFile)
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
	if value != "symlink-content" {
		t.Errorf("Expected symlink-content, got %s", value)
	}
}

// TestReadFlagsFromFileBinaryContent tests binary-like content
func TestReadFlagsFromFileBinaryContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "binary_content")
	content := []byte{0x01, 0x02, 0x03, 0x41, 0x42, 0x43, 0x00, 0xFF}
	err = os.WriteFile(testFile, content, 0600)
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
	if len(value) != 8 {
		t.Errorf("Expected 8 bytes, got %d", len(value))
	}
}

// TestReadFlagsFromFileComplexCSV tests complex CSV values
func TestReadFlagsFromFileComplexCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
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
		t.Fatalf("ReadFlagsFromFile failed: %v", err)
	}

	values, err := cmd.Flags().GetStringSlice(config.Services)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) == 0 {
		t.Error("Expected non-empty services slice")
	}
}

// TestReadFlagsFromFileDataValidation tests data validation
func TestReadFlagsFromFileDataValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "exact_content")
	content := "  prefix content suffix  "
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
	if value != "prefix content suffix" {
		t.Errorf("Expected 'prefix content suffix', got %s", value)
	}
}

// TestReadFlagsFromFileOnlyNewlines tests files with only newlines
func TestReadFlagsFromFileOnlyNewlines(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "only_newlines")
	content := "\n\n\n\n"
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
	if value != "" {
		t.Errorf("Expected empty string, got %s", value)
	}
}
