package main

import (
	"context"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ CommandRunner = (*mockCommandRunner)(nil)

type mockCommandRunner struct {
	stdout  string
	stderr  string
	err     error
	calls   int
	configs []RunCommandConfig

	mu sync.Mutex
}

func (m *mockCommandRunner) Run(ctx context.Context, cfg RunCommandConfig) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++
	m.configs = append(m.configs, cfg)

	return m.stdout, m.stderr, m.err
}

func (m *mockCommandRunner) setResponse(stdout, stderr string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stdout = stdout
	m.stderr = stderr
	m.err = err
}

func (m *mockCommandRunner) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.calls
}

func (m *mockCommandRunner) lastConfig() RunCommandConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.configs) == 0 {
		return RunCommandConfig{}
	}

	return m.configs[len(m.configs)-1]
}

// Expectation: Command with stdout output should capture it correctly.
func Test_RetryCommandRunner_Run_CapturesStdout_Success(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "echo",
		Args:            []string{"hello"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        1,
		AttemptInterval: 100 * time.Millisecond,
	}

	stdout, stderr, err := runner.Run(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, "hello\n", stdout)
	require.Empty(t, stderr)
}

// Expectation: Command with stderr output should capture it correctly.
func Test_RetryCommandRunner_Run_CapturesStderr_Success(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sh",
		Args:            []string{"-c", "echo error >&2"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        1,
		AttemptInterval: 50 * time.Millisecond,
	}

	stdout, stderr, err := runner.Run(ctx, cfg)
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Equal(t, "error\n", stderr)
}

// Expectation: Command that fails should retry the specified number of times.
func Test_RetryCommandRunner_Run_RetriesOnFailure_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sh",
		Args:            []string{"-c", "exit 1"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        3,
		AttemptInterval: 50 * time.Millisecond,
		PrintErrors:     false,
	}

	start := time.Now()
	_, _, err := runner.Run(ctx, cfg)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Contains(t, err.Error(), "[3/3]")
	require.Contains(t, err.Error(), "execution failure")
	require.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

// Expectation: Context cancellation should stop execution immediately.
func Test_RetryCommandRunner_Run_ContextCanceled_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sleep",
		Args:            []string{"10"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        2,
		AttemptInterval: 100 * time.Millisecond,
	}

	_, _, err := runner.Run(ctx, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
}

// Expectation: Context timeout during command execution should be handled.
func Test_RetryCommandRunner_Run_CommandTimeout_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sleep",
		Args:            []string{"10"},
		AttemptTimeout:  100 * time.Millisecond,
		Attempts:        1,
		AttemptInterval: 50 * time.Millisecond,
	}

	_, _, err := runner.Run(ctx, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "execution failure")
}

// Expectation: ExpectJSON should validate JSON output.
func Test_RetryCommandRunner_Run_ExpectJSON_ValidJSON_Success(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "echo",
		Args:            []string{`{"key":"value"}`},
		AttemptTimeout:  5 * time.Second,
		Attempts:        1,
		AttemptInterval: 50 * time.Millisecond,
		ExpectJSON:      true,
	}

	stdout, stderr, err := runner.Run(ctx, cfg)
	require.NoError(t, err)
	require.JSONEq(t, `{"key":"value"}`, stdout)
	require.Empty(t, stderr)
}

// Expectation: ExpectJSON should fail and retry when output is not valid JSON.
func Test_RetryCommandRunner_Run_ExpectJSON_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "echo",
		Args:            []string{"not json"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        2,
		AttemptInterval: 50 * time.Millisecond,
		ExpectJSON:      true,
		PrintErrors:     true,
	}

	stdout, stderr, err := runner.Run(ctx, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidJSON)
	require.Contains(t, err.Error(), "[2/2]")
	require.Equal(t, "not json\n", stdout)
	require.Empty(t, stderr)
}

// Expectation: Context cancellation during retry interval should stop execution.
func Test_RetryCommandRunner_Run_ContextCancelledDuringRetry_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx, cancel := context.WithCancel(t.Context())

	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sh",
		Args:            []string{"-c", "exit 1"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        5,
		AttemptInterval: 5 * time.Second,
		PrintErrors:     false,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, _, err := runner.Run(ctx, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
}

// Expectation: PrintErrors flag should control error logging.
func Test_RetryCommandRunner_Run_PrintErrors_Error(t *testing.T) {
	t.Parallel()

	var logBuf safeBuffer
	runner := &RetryCommandRunner{
		logger: log.New(&logBuf, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sh",
		Args:            []string{"-c", "exit 1"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        1,
		AttemptInterval: 50 * time.Millisecond,
		PrintErrors:     true,
	}

	_, _, err := runner.Run(ctx, cfg)
	require.Error(t, err)
	require.NotEmpty(t, logBuf.String())
	require.Contains(t, logBuf.String(), "test command")
}

// Expectation: Zero retry amount should only attempt once.
func Test_RetryCommandRunner_Run_ZeroRetries_Error(t *testing.T) {
	t.Parallel()

	runner := &RetryCommandRunner{
		logger: log.New(io.Discard, "", 0),
	}

	ctx := t.Context()
	cfg := RunCommandConfig{
		Description:     "test command",
		Command:         "sh",
		Args:            []string{"-c", "exit 1"},
		AttemptTimeout:  5 * time.Second,
		Attempts:        1,
		AttemptInterval: 50 * time.Millisecond,
		PrintErrors:     false,
	}

	_, _, err := runner.Run(ctx, cfg)

	require.Error(t, err)
	require.Contains(t, err.Error(), "[1/1]")
}
