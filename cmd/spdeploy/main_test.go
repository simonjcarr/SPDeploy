package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has correct properties
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "spdeploy" {
		t.Errorf("Expected Use to be 'spdeploy', got '%s'", rootCmd.Use)
	}

	// Check that version information is set
	if !strings.Contains(rootCmd.Version, Version) {
		t.Errorf("Version not set correctly in rootCmd")
	}
}

func TestCommands(t *testing.T) {
	// Test that all expected commands are registered
	expectedCommands := []string{
		"add",
		"remove",
		"list",
		"run",
		"stop",
		"log",
	}

	commands := rootCmd.Commands()
	commandMap := make(map[string]*cobra.Command)
	for _, cmd := range commands {
		commandMap[cmd.Use] = cmd
	}

	// Handle commands that might have arguments in their Use field
	for _, cmd := range commands {
		// Extract the base command name (before any space)
		baseName := strings.Fields(cmd.Use)[0]
		commandMap[baseName] = cmd
	}

	for _, expectedCmd := range expectedCommands {
		if _, exists := commandMap[expectedCmd]; !exists {
			t.Errorf("Expected command '%s' not found", expectedCmd)
		}
	}
}

func TestAddCommandFlags(t *testing.T) {
	// Find the add command
	var addCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if strings.HasPrefix(cmd.Use, "add") {
			addCommand = cmd
			break
		}
	}

	if addCommand == nil {
		t.Fatal("add command not found")
	}

	// Check that expected flags exist
	branchFlag := addCommand.Flags().Lookup("branch")
	if branchFlag == nil {
		t.Error("branch flag not found on add command")
	} else if branchFlag.DefValue != "main" {
		t.Errorf("Expected branch default to be 'main', got '%s'", branchFlag.DefValue)
	}

	scriptFlag := addCommand.Flags().Lookup("script")
	if scriptFlag == nil {
		t.Error("script flag not found on add command")
	}
}

func TestRunCommandFlags(t *testing.T) {
	// Find the run command
	var runCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "run" {
			runCommand = cmd
			break
		}
	}

	if runCommand == nil {
		t.Fatal("run command not found")
	}

	// Check daemon flag
	daemonFlag := runCommand.Flags().Lookup("daemon")
	if daemonFlag == nil {
		t.Error("daemon flag not found on run command")
	}
	if daemonFlag.Shorthand != "d" {
		t.Errorf("Expected daemon flag shorthand to be 'd', got '%s'", daemonFlag.Shorthand)
	}
}

func TestLogCommandFlags(t *testing.T) {
	// Find the log command
	var logCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "log" {
			logCommand = cmd
			break
		}
	}

	if logCommand == nil {
		t.Fatal("log command not found")
	}

	// Check expected flags
	expectedFlags := []struct {
		name      string
		shorthand string
	}{
		{"follow", "f"},
		{"global", "g"},
		{"repo", "r"},
		{"all", "a"},
		{"user", "u"},
	}

	for _, expected := range expectedFlags {
		flag := logCommand.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("Flag '%s' not found on log command", expected.name)
			continue
		}
		if flag.Shorthand != expected.shorthand {
			t.Errorf("Expected flag '%s' shorthand to be '%s', got '%s'",
				expected.name, expected.shorthand, flag.Shorthand)
		}
	}
}

func TestListCommandExecution(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "spdeploy-cmd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Find the list command
	var listCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "list" {
			listCommand = cmd
			break
		}
	}

	if listCommand == nil {
		t.Fatal("list command not found")
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute list command (should show "No repositories configured")
	listCommand.Run(listCommand, []string{})

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No repositories configured") {
		t.Errorf("Expected output to contain 'No repositories configured', got: %s", output)
	}
}

func TestVersionOutput(t *testing.T) {
	// Test version template
	expectedVersion := Version
	if !strings.Contains(rootCmd.Version, expectedVersion) {
		t.Errorf("Version string doesn't contain expected version %s", expectedVersion)
	}
}

// Test helper functions
func TestConfigPathGeneration(t *testing.T) {
	// Test that config paths are generated correctly
	tmpDir, err := os.MkdirTemp("", "spdeploy-path-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Test paths that would be created
	expectedConfigPath := filepath.Join(tmpDir, ".spdeploy", "config.json")
	expectedLogPath := filepath.Join(tmpDir, ".spdeploy", "logs")

	// These paths should be consistent
	if !strings.Contains(expectedConfigPath, ".spdeploy") {
		t.Error("Config path doesn't contain .spdeploy directory")
	}
	if !strings.Contains(expectedLogPath, ".spdeploy") {
		t.Error("Log path doesn't contain .spdeploy directory")
	}
}