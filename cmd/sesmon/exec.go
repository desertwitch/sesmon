package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// waitDelay is the maximum time to wait for subprocesses to exit on termination.
const waitDelay = 5 * time.Second

// CommandRunner is the contract for a command execution helper as part of a [Program].
type CommandRunner interface {
	Run(ctx context.Context, cfg RunCommandConfig) (stdout, stderr string, err error)
}

// RunCommandConfig is the configuration for a [RetryCommandRunner] implementation.
type RunCommandConfig struct {
	Description string
	Command     string
	Args        []string

	Attempts        int
	AttemptTimeout  time.Duration
	AttemptInterval time.Duration

	ExpectJSON  bool
	PrintErrors bool
}

var _ CommandRunner = (*RetryCommandRunner)(nil)

// RetryCommandRunner is the principal [CommandRunner] implementation.
type RetryCommandRunner struct {
	logger *log.Logger
}

// Run executes a command according to a provided [RunCommandConfig].
// It both observes and respects context cancellation for earlier termination.
func (r *RetryCommandRunner) Run(ctx context.Context, cfg RunCommandConfig) (string, string, error) {
	var stdout, stderr string

	attempt, err := withRetries(
		ctx,
		func() error {
			var stdoutBuf, stderrBuf bytes.Buffer

			runCtx, runCancel := context.WithTimeout(ctx, cfg.AttemptTimeout)
			defer runCancel()

			cmd := exec.CommandContext(runCtx, cfg.Command, cfg.Args...)
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf
			cmd.WaitDelay = waitDelay

			err := cmd.Run()
			stdout = stdoutBuf.String()
			stderr = stderrBuf.String()

			if err == nil && cfg.ExpectJSON && !json.Valid(stdoutBuf.Bytes()) {
				err = errInvalidJSON
			}

			return err
		},
		func(attempt int, err error) {
			if cfg.PrintErrors {
				r.logger.Printf("%s: [%d/%d] execution failure: %v: stdout=[%s] stderr=[%s]",
					cfg.Description, attempt, cfg.Attempts, err, stdout, stderr)
			}
		},
		cfg.Attempts,
		cfg.AttemptInterval,
	)
	if err != nil {
		return stdout, stderr, fmt.Errorf("[%d/%d] execution failure: %w: stdout=[%s] stderr=[%s]",
			attempt, cfg.Attempts, err, stdout, stderr)
	}

	return stdout, stderr, nil
}
