package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/spf13/afero"
)

const (
	executableModeMask = 0o111
)

// errNotExecutable occurs when a target binary/script is not executable.
var errNotExecutable = errors.New("not executable permissions")

// Notifier is the contract for a notification agent as part of a [Program].
type Notifier interface {
	Notify(ctx context.Context, device Device, message string, extra any) error
	Name() string
	Config() string
}

// ScriptNotifierConfig is the configuration for a [ScriptNotifier] implementation.
type ScriptNotifierConfig struct {
	// How often to attempt a notification (must be > 0).
	NotifyAttempts *int `yaml:"notify_attempts"`

	// How long a notification attempt can take (multiplies with attempts).
	NotifyAttemptTimeout *time.Duration `yaml:"notify_attempt_timeout"`

	// How long to wait between notification attempts (in case of failure).
	NotifyAttemptInterval *time.Duration `yaml:"notify_attempt_interval"`
}

// MarshalJSON is a custom JSON marshaller for user readable [time.Duration] strings.
func (c ScriptNotifierConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct { //nolint:wrapcheck
		NotifyAttempts        *int    `json:"notify_attempts"`
		NotifyAttemptTimeout  *string `json:"notify_attempt_timeout"`
		NotifyAttemptInterval *string `json:"notify_attempt_interval"`
	}{
		NotifyAttempts:        c.NotifyAttempts,
		NotifyAttemptTimeout:  durPtrToStrPtr(c.NotifyAttemptTimeout),
		NotifyAttemptInterval: durPtrToStrPtr(c.NotifyAttemptInterval),
	})
}

// DefaultScriptNotifierConfig returns a pointer to a default [ScriptNotifierConfig].
//
//nolint:mnd
func DefaultScriptNotifierConfig() *ScriptNotifierConfig {
	return &ScriptNotifierConfig{
		NotifyAttempts:        ptr(3),
		NotifyAttemptTimeout:  ptr(15 * time.Second),
		NotifyAttemptInterval: ptr(15 * time.Second),
	}
}

var _ Notifier = (*ScriptNotifier)(nil)

// ScriptNotifier is a [Notifier] executing a custom user-defined script.
// The script receives these arguments as part of the command's execution:
//   - $1: Device path (e.g., /dev/sg25)
//   - $2: SAS address (e.g., 0x500a098012345678)
//   - $3: Device description (e.g., "JBOD")
//   - $4: Notification message text
//   - $5: Change report in JSON format (where applicable)
type ScriptNotifier struct {
	// Path to executable notification script.
	script string

	fsys   afero.Fs
	runner CommandRunner
	logger *log.Logger

	cfg *ScriptNotifierConfig
}

// NewScriptNotifier returns a pointer to a new [ScriptNotifier].
func NewScriptNotifier(
	script string, cfg *ScriptNotifierConfig,
	fsys afero.Fs, runner CommandRunner, logger *log.Logger,
) (*ScriptNotifier, error) {
	if fsys == nil || runner == nil || logger == nil {
		return nil, fmt.Errorf("%w: required dependency is nil", errInvalidArgument)
	}

	if script == "" {
		return nil, fmt.Errorf("%w: no script provided", errInvalidArgument)
	}
	st, err := fsys.Stat(script)
	if err != nil {
		return nil, fmt.Errorf("%q: stat script failure: %w", script, err)
	}
	if (st.Mode() & executableModeMask) == 0 {
		return nil, fmt.Errorf("%q: %w", script, errNotExecutable)
	}

	scfg, err := mergeScriptNotifierConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("configuration failure: %w", err)
	}

	return &ScriptNotifier{
		script: script,
		cfg:    scfg,
		fsys:   fsys,
		runner: runner,
		logger: logger,
	}, nil
}

// Notify executes the user-defined script using the internal [CommandRunner].
// It hands over as arguments the device, SAS address, device description and message.
// It both observes and respects context cancellations for earlier notification terminations.
func (n *ScriptNotifier) Notify(ctx context.Context, device Device, message string, extra any) error {
	args := []string{device.Path, device.Address, device.Description, message}

	if extra != nil {
		b, err := json.Marshal(extra)
		if err != nil {
			return fmt.Errorf("%q: failure marshalling extra to JSON: %w", n.script, err)
		}
		args = append(args, string(b))
	}

	_, _, err := n.runner.Run(ctx, RunCommandConfig{
		Description:     fmt.Sprintf("%q", n.script),
		Command:         n.script,
		Args:            args,
		Attempts:        *n.cfg.NotifyAttempts,
		AttemptTimeout:  *n.cfg.NotifyAttemptTimeout,
		AttemptInterval: *n.cfg.NotifyAttemptInterval,
		PrintErrors:     true,
	})
	if err != nil {
		return fmt.Errorf("%q: %w", n.script, err)
	}

	return nil
}

// Name returns the name of the notification agent as a string.
func (n *ScriptNotifier) Name() string {
	return "script_notifier"
}

// Config returns the configuration of the notification agent as a string.
func (n *ScriptNotifier) Config() string {
	cfgJSON, err := json.Marshal(n.cfg)
	if err != nil {
		cfgJSON = []byte("n/a")
	}

	return fmt.Sprintf("%q:%s", n.script, cfgJSON)
}
