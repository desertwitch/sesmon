package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

var _ Notifier = (*mockNotifier)(nil)

type mockNotifier struct {
	calls    []string
	err      error
	notified chan struct{}

	mu sync.Mutex
}

func newMockNotifier() *mockNotifier {
	return &mockNotifier{
		calls:    make([]string, 0),
		notified: make(chan struct{}, 10),
	}
}

func (m *mockNotifier) Name() string {
	return "mock_notifier"
}

func (m *mockNotifier) Config() string {
	return "-"
}

func (m *mockNotifier) Notify(ctx context.Context, device Device, msg string, extra any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, msg)
	select {
	case m.notified <- struct{}{}:
	default:
	}

	return m.err
}

func (m *mockNotifier) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.calls)
}

func (m *mockNotifier) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]string, len(m.calls))
	copy(result, m.calls)

	return result
}

func (m *mockNotifier) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.err = err
}

func (m *mockNotifier) waitForNotification(timeout time.Duration) bool {
	select {
	case <-m.notified:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Expectation: ScriptNotifierConfig MarshalJSON should correctly serialize durations as strings.
func Test_ScriptNotifierConfig_MarshalJSON_Success(t *testing.T) {
	t.Parallel()

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(3),
		NotifyAttemptTimeout:  ptr(15 * time.Second),
		NotifyAttemptInterval: ptr(10 * time.Second),
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.InDelta(t, 3, result["notify_attempts"], 0.001)
	require.Equal(t, "10s", result["notify_attempt_interval"])
	require.Equal(t, "15s", result["notify_attempt_timeout"])
}

// Expectation: ScriptNotifierConfig MarshalJSON should handle nil duration pointers.
func Test_ScriptNotifierConfig_MarshalJSON_NilDurations_Success(t *testing.T) {
	t.Parallel()

	cfg := &ScriptNotifierConfig{
		NotifyAttempts: ptr(5),
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.InDelta(t, 5, result["notify_attempts"], 0.001)
	require.Nil(t, result["notify_attempt_interval"])
	require.Nil(t, result["notify_attempt_timeout"])
}

// Expectation: ScriptNotifierConfig MarshalJSON should handle zero values.
func Test_ScriptNotifierConfig_MarshalJSON_ZeroValues_Success(t *testing.T) {
	t.Parallel()

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(0),
		NotifyAttemptTimeout:  ptr(0 * time.Second),
		NotifyAttemptInterval: ptr(0 * time.Second),
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.InDelta(t, 0, result["notify_attempts"], 0.001)
	require.Equal(t, "0s", result["notify_attempt_interval"])
	require.Equal(t, "0s", result["notify_attempt_timeout"])
}

// Expectation: NewScriptNotifier should successfully create notifier with a valid executable script.
func Test_NewScriptNotifier_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.NotNil(t, notifier)
	require.Equal(t, scriptPath, notifier.script)
}

// Expectation: NewScriptNotifier should return an error when no script is provided.
func Test_NewScriptNotifier_NoScriptGiven_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier("", cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.Error(t, err)
	require.Nil(t, notifier)
	require.Contains(t, err.Error(), "no script")
}

// Expectation: NewScriptNotifier should return an error when script doesn't exist.
func Test_NewScriptNotifier_ScriptNotExist_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier("/tmp/nonexistent.sh", cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.Error(t, err)
	require.Nil(t, notifier)
	require.Contains(t, err.Error(), "stat script failure")
}

// Expectation: NewScriptNotifier should return an error when no dependencies are provided.
func Test_NewScriptNotifier_NoDependenciesGiven_Error(t *testing.T) {
	t.Parallel()

	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier("/tmp/nonexistent.sh", cfg, nil, runner, log.New(io.Discard, "", 0))
	require.Error(t, err)
	require.Nil(t, notifier)
	require.Contains(t, err.Error(), "dependency is nil")
}

// Expectation: NewScriptNotifier should return an error when script is not executable.
func Test_NewScriptNotifier_NotExecutable_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o644)
	require.NoError(t, err)

	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.Error(t, err)
	require.Nil(t, notifier)
	require.ErrorIs(t, err, errNotExecutable)
}

// Expectation: NewScriptNotifier should use default config when nil is passed.
func Test_NewScriptNotifier_NilConfig_UsesDefault_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, nil, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.NotNil(t, notifier)
	require.Equal(t, DefaultScriptNotifierConfig(), notifier.cfg)
}

// Expectation: ScriptNotifier should successfully notify with valid executable script.
func Test_ScriptNotifier_Notify_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}
	runner.setResponse("success", "", nil)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	err = notifier.Notify(ctx, Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}, "test message", nil)
	require.NoError(t, err)
	require.Equal(t, 1, runner.callCount())
	require.Equal(t, scriptPath, runner.lastConfig().Command)
}

// Expectation: ScriptNotifier should pass correct arguments to runner.
func Test_ScriptNotifier_Notify_CorrectArguments_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}
	runner.setResponse("success", "", nil)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	device := Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}
	message := "test message"

	err = notifier.Notify(ctx, device, message, nil)
	require.NoError(t, err)

	lastConfig := runner.lastConfig()
	require.Equal(t, scriptPath, lastConfig.Command)
	require.Equal(t, []string{device.Path, device.Address, device.Description, message}, lastConfig.Args)
	require.Equal(t, *cfg.NotifyAttemptTimeout, lastConfig.AttemptTimeout)
	require.Equal(t, *cfg.NotifyAttempts, lastConfig.Attempts)
	require.Equal(t, *cfg.NotifyAttemptInterval, lastConfig.AttemptInterval)
}

// Expectation: ScriptNotifier should pass correct arguments to runner.
func Test_ScriptNotifier_Notify_CorrectArguments_WithExtra_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}
	runner.setResponse("success", "", nil)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	device := Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}
	message := "test message"

	extra := ChangeReport{Device: device, DetectedAt: time.Now().String()}
	extraJSON, err := json.Marshal(extra)
	require.NoError(t, err)

	err = notifier.Notify(ctx, device, message, extra)
	require.NoError(t, err)

	lastConfig := runner.lastConfig()
	require.Equal(t, scriptPath, lastConfig.Command)
	require.Equal(t, []string{device.Path, device.Address, device.Description, message, string(extraJSON)}, lastConfig.Args)
	require.Equal(t, *cfg.NotifyAttemptTimeout, lastConfig.AttemptTimeout)
	require.Equal(t, *cfg.NotifyAttempts, lastConfig.Attempts)
	require.Equal(t, *cfg.NotifyAttemptInterval, lastConfig.AttemptInterval)
}

// Expectation: ScriptNotifier should return error when runner fails.
func Test_ScriptNotifier_Notify_RunnerError_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}
	runnerErr := errors.New("runner failed")
	runner.setResponse("", "", runnerErr)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	err = notifier.Notify(ctx, Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}, "test message", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, runnerErr)
}

type noJSON struct{}

func (noJSON) MarshalJSON() ([]byte, error) { return nil, errors.New("JSON error") }

// Expectation: ScriptNotifier should return error when JSON marshalling fails.
func Test_ScriptNotifier_Notify_JSONError_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	err = notifier.Notify(ctx, Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}, "test message", noJSON{})
	require.Error(t, err)
	require.ErrorContains(t, err, "JSON")
}

// Expectation: ScriptNotifier should wrap errors with script name.
func Test_ScriptNotifier_Notify_ErrorWrapping_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	runner := &mockCommandRunner{}
	runnerErr := errors.New("runner failed")
	runner.setResponse("", "", runnerErr)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	ctx := t.Context()
	err = notifier.Notify(ctx, Device{Path: "/dev/sg25", Address: "0x00", Description: "Test Device"}, "test message", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), scriptPath)
	require.ErrorIs(t, err, runnerErr)
}

// Expectation: ScriptNotifier Name method should return correct name.
func Test_ScriptNotifier_Name_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	cfg := DefaultScriptNotifierConfig()
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.Equal(t, "script_notifier", notifier.Name())
}

// Expectation: ScriptNotifier Config method should return valid JSON with script path.
func Test_ScriptNotifier_Config_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	cfg := &ScriptNotifierConfig{
		NotifyAttempts:        ptr(2),
		NotifyAttemptTimeout:  ptr(5 * time.Second),
		NotifyAttemptInterval: ptr(100 * time.Millisecond),
	}
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	config := notifier.Config()
	require.Contains(t, config, scriptPath)
	require.Contains(t, config, "notify_attempts")
	require.Contains(t, config, "5s")
	require.Contains(t, config, "100ms")
}

// Expectation: ScriptNotifier Config method should handle nil config values.
func Test_ScriptNotifier_Config_NilValues_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	scriptPath := "/tmp/notify.sh"
	err := afero.WriteFile(fsys, scriptPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	cfg := &ScriptNotifierConfig{}
	runner := &mockCommandRunner{}

	notifier, err := NewScriptNotifier(scriptPath, cfg, fsys, runner, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	config := notifier.Config()
	require.Contains(t, config, scriptPath)
}
