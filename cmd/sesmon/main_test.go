package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: newRootCmd should create root command with monitor, check, and test subcommands.
func Test_newRootCmd_SubcommandsAdded_Success(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	rootCmd := newRootCmd(ctx)

	require.NotNil(t, rootCmd)
	require.Equal(t, "sesmon", rootCmd.Use)
	require.Equal(t, "SES monitoring and alerting daemon", rootCmd.Short)
	require.Equal(t, Version, rootCmd.Version)
	require.True(t, rootCmd.SilenceUsage)
	require.True(t, rootCmd.CompletionOptions.DisableDefaultCmd)

	commands := rootCmd.Commands()
	require.Len(t, commands, 3)

	commandNames := make([]string, len(commands))
	for i, cmd := range commands {
		commandNames[i] = cmd.Name()
	}
	require.Contains(t, commandNames, "monitor")
	require.Contains(t, commandNames, "check")
	require.Contains(t, commandNames, "test")
}

// Expectation: newMonitorCmd should return error when config file does not exist.
func Test_newMonitorCmd_ConfigFileNotFound_Error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	monitorCmd := newMonitorCmd(ctx)

	monitorCmd.SetOut(io.Discard)
	monitorCmd.SetErr(io.Discard)

	monitorCmd.SetArgs([]string{"nonexistent.yaml"})
	err := monitorCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure reading configuration file")
}

// Expectation: newMonitorCmd should return error when no arguments provided.
func Test_newMonitorCmd_NoArgs_Error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	monitorCmd := newMonitorCmd(ctx)

	monitorCmd.SetOut(io.Discard)
	monitorCmd.SetErr(io.Discard)

	monitorCmd.SetArgs([]string{})
	err := monitorCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
}

// Expectation: newMonitorCmd should return error when invalid YAML provided.
func Test_newMonitorCmd_InvalidYAML_Error(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0o600)
	require.NoError(t, err)

	ctx := t.Context()
	monitorCmd := newMonitorCmd(ctx)

	monitorCmd.SetOut(io.Discard)
	monitorCmd.SetErr(io.Discard)

	monitorCmd.SetArgs([]string{configPath})
	err = monitorCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure establishing program")
}

// Expectation: newCheckCmd should return error when config file does not exist.
func Test_newCheckCmd_ConfigFileNotFound_Error(t *testing.T) {
	t.Parallel()

	checkCmd := newCheckCmd()

	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)

	checkCmd.SetArgs([]string{"nonexistent.yaml"})
	err := checkCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure reading configuration file")
}

// Expectation: newCheckCmd should return error when no arguments provided.
func Test_newCheckCmd_NoArgs_Error(t *testing.T) {
	t.Parallel()

	checkCmd := newCheckCmd()

	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)

	checkCmd.SetArgs([]string{})
	err := checkCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
}

// Expectation: newCheckCmd should return error when YAML is invalid.
func Test_newCheckCmd_InvalidYAML_Error(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0o600)
	require.NoError(t, err)

	checkCmd := newCheckCmd()

	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)

	checkCmd.SetArgs([]string{configPath})
	err = checkCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure parsing YAML")
}

// Expectation: newCheckCmd should succeed when YAML is valid.
func Test_newCheckCmd_ValidYAML_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid.yaml")
	validYAML := `---
devices:
  - device: /dev/sg0
    address: "5000c500a1b2c3d4"
    enabled: false
`
	err := os.WriteFile(configPath, []byte(validYAML), 0o600)
	require.NoError(t, err)

	checkCmd := newCheckCmd()

	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)

	checkCmd.SetArgs([]string{configPath})
	err = checkCmd.Execute()

	require.NoError(t, err)
}

// Expectation: newCheckCmd should return error when YAML has unknown fields.
func Test_newCheckCmd_UnknownFields_Error(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown.yaml")
	invalidYAML := `---
unknown_field: "value"
devices:
  - device: /dev/sg0
    enabled: false
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0o600)
	require.NoError(t, err)

	checkCmd := newCheckCmd()

	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)

	checkCmd.SetArgs([]string{configPath})
	err = checkCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure parsing YAML")
}

// Expectation: newTestCmd should return error when config file does not exist.
func Test_newTestCmd_ConfigFileNotFound_Error(t *testing.T) {
	t.Parallel()

	testCmd := newTestCmd()

	testCmd.SetOut(io.Discard)
	testCmd.SetErr(io.Discard)

	testCmd.SetArgs([]string{"nonexistent.yaml"})
	err := testCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure reading configuration file")
}

// Expectation: newTestCmd should return error when no arguments provided.
func Test_newTestCmd_NoArgs_Error(t *testing.T) {
	t.Parallel()

	testCmd := newTestCmd()

	testCmd.SetOut(io.Discard)
	testCmd.SetErr(io.Discard)

	testCmd.SetArgs([]string{})
	err := testCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
}

// Expectation: newTestCmd should return error when YAML is invalid.
func Test_newTestCmd_InvalidYAML_Error(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0o600)
	require.NoError(t, err)

	testCmd := newTestCmd()

	testCmd.SetOut(io.Discard)
	testCmd.SetErr(io.Discard)

	testCmd.SetArgs([]string{configPath})
	err = testCmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure establishing program")
}

// Expectation: newTestCmd should succeed when program can be established.
func Test_newTestCmd_ValidConfig_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid.yaml")

	devicePath := filepath.Join(tmpDir, "device.json")
	err := os.WriteFile(devicePath, []byte(`{}`), 0o600)
	require.NoError(t, err)

	validYAML := `---
devices:
  - device: ` + devicePath + `
    address: "5000c500a1b2c3d4"
    type: 1
    enabled: true
`
	err = os.WriteFile(configPath, []byte(validYAML), 0o600)
	require.NoError(t, err)

	testCmd := newTestCmd()

	testCmd.SetOut(io.Discard)
	testCmd.SetErr(io.Discard)

	testCmd.SetArgs([]string{configPath})
	err = testCmd.Execute()

	require.NoError(t, err)
}
