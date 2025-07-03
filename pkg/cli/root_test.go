package cli_test

import (
	"bytes"
	"testing"

	"github.com/openshift/ocm-agent/pkg/cli"
	"github.com/spf13/pflag"
)

// TestNewCmdRoot tests the creation and basic properties of the root command
func TestNewCmdRoot(t *testing.T) {
	rootCmd := cli.NewCmdRoot()

	if rootCmd == nil {
		t.Fatal("NewCmdRoot returned nil")
	}

	if rootCmd.Use != "ocm-agent" {
		t.Errorf("Expected command Use to be 'ocm-agent', got %s", rootCmd.Use)
	}

	expectedShort := "Command line tool for OCM Agent to talk to OCM services."
	if rootCmd.Short != expectedShort {
		t.Errorf("Expected command Short to be '%s', got %s", expectedShort, rootCmd.Short)
	}

	expectedLong := "Command line tool for OCM Agent to talk to OCM services."
	if rootCmd.Long != expectedLong {
		t.Errorf("Expected command Long to be '%s', got %s", expectedLong, rootCmd.Long)
	}

	if !rootCmd.DisableAutoGenTag {
		t.Error("Expected DisableAutoGenTag to be true")
	}
}

// TestRootCommandSubcommands tests that the root command has the correct subcommands
func TestRootCommandSubcommands(t *testing.T) {
	rootCmd := cli.NewCmdRoot()

	commands := rootCmd.Commands()
	if len(commands) != 1 {
		t.Errorf("Expected exactly 1 subcommand, got %d", len(commands))
	}

	if len(commands) > 0 && commands[0].Use != "serve" {
		t.Errorf("Expected first subcommand to be 'serve', got %s", commands[0].Use)
	}
}

// TestRootCommandFlags tests that the root command has no flags
func TestRootCommandFlags(t *testing.T) {
	rootCmd := cli.NewCmdRoot()

	flags := rootCmd.Flags()
	if flags.NFlag() != 0 {
		t.Errorf("Expected root command to have 0 flags, got %d", flags.NFlag())
	}

	persistentFlags := rootCmd.PersistentFlags()
	if persistentFlags.NFlag() != 0 {
		t.Errorf("Expected root command to have 0 persistent flags, got %d", persistentFlags.NFlag())
	}
}

// TestRootCommandHelp tests the help functionality
func TestRootCommandHelp(t *testing.T) {
	rootCmd := cli.NewCmdRoot()
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)

	err := rootCmd.Help()
	if err != nil {
		t.Fatalf("Help() failed: %v", err)
	}

	helpOutput := output.String()
	expectedStrings := []string{
		"ocm-agent",
		"Command line tool for OCM Agent",
		"Usage:",
		"Available Commands:",
		"serve",
	}

	for _, expected := range expectedStrings {
		if !contains(helpOutput, expected) {
			t.Errorf("Expected help output to contain '%s'", expected)
		}
	}
}

// TestRootCommandInvalidSubcommand tests handling of invalid subcommands
func TestRootCommandInvalidSubcommand(t *testing.T) {
	rootCmd := cli.NewCmdRoot()
	rootCmd.SetArgs([]string{"invalid-command"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid subcommand")
	}
	if !contains(err.Error(), "unknown command") {
		t.Errorf("Expected error to contain 'unknown command', got %s", err.Error())
	}
}

// TestRootCommandStructure tests the command structure and relationships
func TestRootCommandStructure(t *testing.T) {
	rootCmd := cli.NewCmdRoot()

	commands := rootCmd.Commands()
	if len(commands) == 0 {
		t.Fatal("Expected at least one subcommand")
	}

	serveCmd := commands[0]
	if serveCmd.Use != "serve" {
		t.Errorf("Expected serve command, got %s", serveCmd.Use)
	}

	if serveCmd.Short != "Starts the OCM Agent server" {
		t.Errorf("Expected serve command Short to be 'Starts the OCM Agent server', got %s", serveCmd.Short)
	}

	if serveCmd.Parent() != rootCmd {
		t.Error("Expected serve command parent to be root command")
	}
}

// TestRootCommandHelpFlags tests help flag functionality
func TestRootCommandHelpFlags(t *testing.T) {
	// Test --help flag
	rootCmd := cli.NewCmdRoot()
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute with --help failed: %v", err)
	}

	helpOutput := output.String()
	if !contains(helpOutput, "Command line tool for OCM Agent") {
		t.Error("Expected help output to contain description")
	}

	// Test -h flag
	rootCmd2 := cli.NewCmdRoot()
	output2 := &bytes.Buffer{}
	rootCmd2.SetOut(output2)
	rootCmd2.SetErr(output2)
	rootCmd2.SetArgs([]string{"-h"})

	err = rootCmd2.Execute()
	if err != nil {
		t.Fatalf("Execute with -h failed: %v", err)
	}

	helpOutput2 := output2.String()
	if !contains(helpOutput2, "Command line tool for OCM Agent") {
		t.Error("Expected help output to contain description")
	}
}

// TestInitConfig tests the initConfig function indirectly
func TestInitConfig(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	// Test that creating a new root command doesn't panic
	// This indirectly tests the initConfig function which is called via cobra.OnInitialize
	rootCmd := cli.NewCmdRoot()
	if rootCmd == nil {
		t.Fatal("NewCmdRoot returned nil")
	}

	// Check that pflag.CommandLine is properly initialized
	if pflag.CommandLine == nil {
		t.Error("Expected pflag.CommandLine to be initialized")
	}

	// Check that it's a valid flagset by testing it has no flags initially
	if pflag.CommandLine.NFlag() != 0 {
		t.Errorf("Expected pflag.CommandLine to have 0 flags initially, got %d", pflag.CommandLine.NFlag())
	}
}

// TestInitConfigEmptyCommandLine tests initConfig with empty command line
func TestInitConfigEmptyCommandLine(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	// This test ensures the initConfig function doesn't fail with empty args
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("initConfig panicked with empty command line: %v", r)
		}
	}()

	rootCmd := cli.NewCmdRoot()
	if rootCmd == nil {
		t.Fatal("NewCmdRoot returned nil")
	}
}

// TestInitConfigFlagSetProperties tests that initConfig properly sets up the flagset
func TestInitConfigFlagSetProperties(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	rootCmd := cli.NewCmdRoot()
	if rootCmd == nil {
		t.Fatal("NewCmdRoot returned nil")
	}

	// The flagset should be configured properly
	if pflag.CommandLine == nil {
		t.Error("Expected pflag.CommandLine to be initialized")
	}

	// Should have no flags initially
	if pflag.CommandLine.NFlag() != 0 {
		t.Errorf("Expected pflag.CommandLine to have 0 flags, got %d", pflag.CommandLine.NFlag())
	}
}

// TestInitConfigIdempotent tests that initConfig can be called multiple times
func TestInitConfigIdempotent(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	// Create multiple root commands to ensure initConfig can be called multiple times
	rootCmd1 := cli.NewCmdRoot()
	rootCmd2 := cli.NewCmdRoot()

	if rootCmd1 == nil {
		t.Fatal("First NewCmdRoot returned nil")
	}
	if rootCmd2 == nil {
		t.Fatal("Second NewCmdRoot returned nil")
	}

	if pflag.CommandLine == nil {
		t.Error("Expected pflag.CommandLine to be initialized")
	}

	if pflag.CommandLine.NFlag() != 0 {
		t.Errorf("Expected pflag.CommandLine to have 0 flags after multiple calls, got %d", pflag.CommandLine.NFlag())
	}
}

// TestInitConfigErrorHandling tests error handling in initConfig
func TestInitConfigErrorHandling(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	// Test that initConfig handles edge cases gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("initConfig panicked unexpectedly: %v", r)
		}
	}()

	// Create root command multiple times to test stability
	for i := 0; i < 3; i++ {
		rootCmd := cli.NewCmdRoot()
		if rootCmd == nil {
			t.Fatalf("NewCmdRoot returned nil on iteration %d", i)
		}
	}
}

// TestInitConfigFlagSetInitialization tests the specific flag set initialization
func TestInitConfigFlagSetInitialization(t *testing.T) {
	// Save original command line for restoration
	originalCommandLine := pflag.CommandLine
	defer func() {
		pflag.CommandLine = originalCommandLine
	}()

	// Test that the flagset is properly initialized by initConfig
	rootCmd := cli.NewCmdRoot()
	if rootCmd == nil {
		t.Fatal("NewCmdRoot returned nil")
	}

	// Verify that pflag.CommandLine is a valid FlagSet
	if pflag.CommandLine == nil {
		t.Error("Expected pflag.CommandLine to be initialized")
	}

	// Test that we can call basic FlagSet methods without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FlagSet methods panicked: %v", r)
		}
	}()

	// Test basic flagset operations
	flagCount := pflag.CommandLine.NFlag()
	if flagCount < 0 {
		t.Errorf("Expected non-negative flag count, got %d", flagCount)
	}

	// Test that we can parse empty args
	err := pflag.CommandLine.Parse([]string{})
	if err != nil {
		t.Errorf("Expected no error parsing empty args, got %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsAt(s, substr, 0)))
}

func containsAt(s, substr string, offset int) bool {
	if offset+len(substr) > len(s) {
		return false
	}
	for i := 0; i < len(substr); i++ {
		if s[offset+i] != substr[i] {
			if offset+1 <= len(s)-len(substr) {
				return containsAt(s, substr, offset+1)
			}
			return false
		}
	}
	return true
}

// Benchmark tests
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
