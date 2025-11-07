package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// fne returns the first non-empty string of two strings.
func fne(a, b string) string {
	if a != "" {
		return a
	}

	return b
}

// fnz returns the first non-zero integer of two integers.
func fnz(a, b int) int {
	if a != 0 {
		return a
	}

	return b
}

// ptr converts a value of type [T] to a pointer of type [*T].
func ptr[T any](v T) *T {
	return &v
}

// ptrIntEqual compares two integer pointers (nil safe).
func ptrIntEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	return *a == *b
}

// ptrStrEqualFold compares case insensitive two string pointers (nil safe).
func ptrStrEqualFold(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	return strings.EqualFold(*a, *b)
}

// fmtPtrInt converts an integer pointer to a string or returns [e] if nil.
func fmtPtrInt(p *int, e string) string {
	if p == nil {
		return e
	}

	return strconv.Itoa(*p)
}

// fmtPtrStr dereferences a string pointer to a string or returns [e] if nil.
func fmtPtrStr(p *string, e string) string {
	if p == nil {
		return e
	}

	return *p
}

// fmtPtrQStr dereferences a string pointer to a %q string or returns [e] if nil.
func fmtPtrQStr(p *string, e string) string {
	if p == nil {
		return e
	}

	return fmt.Sprintf("%q", *p)
}

// durPtrToStrPtr converts a [time.Duration] pointer to a string pointer (nil safe).
func durPtrToStrPtr(d *time.Duration) *string {
	if d == nil {
		return nil
	}
	s := d.String()

	return &s
}

// mergeDeviceMonitorConfig merges a user-provided config with defaults.
// Any nil fields in the user config will be replaced with values from the default config.
func mergeDeviceMonitorConfig(userCfg *DeviceMonitorConfig) (*DeviceMonitorConfig, error) {
	if userCfg == nil {
		return DefaultDeviceMonitorConfig(), nil
	}

	merged := &DeviceMonitorConfig{}
	defaultCfg := DefaultDeviceMonitorConfig()

	if userCfg.PollInterval != nil {
		merged.PollInterval = userCfg.PollInterval
	} else {
		merged.PollInterval = defaultCfg.PollInterval
	}

	if userCfg.PollAttempts != nil {
		if *userCfg.PollAttempts <= 0 {
			return nil, fmt.Errorf("%w: poll_attempts must be > 0", errInvalidArgument)
		}
		merged.PollAttempts = userCfg.PollAttempts
	} else {
		merged.PollAttempts = defaultCfg.PollAttempts
	}

	if userCfg.PollAttemptTimeout != nil {
		merged.PollAttemptTimeout = userCfg.PollAttemptTimeout
	} else {
		merged.PollAttemptTimeout = defaultCfg.PollAttemptTimeout
	}

	if userCfg.PollAttemptInterval != nil {
		merged.PollAttemptInterval = userCfg.PollAttemptInterval
	} else {
		merged.PollAttemptInterval = defaultCfg.PollAttemptInterval
	}

	if userCfg.PollBackoffAfter != nil {
		merged.PollBackoffAfter = userCfg.PollBackoffAfter
	} else {
		merged.PollBackoffAfter = defaultCfg.PollBackoffAfter
	}

	if userCfg.PollBackoffTime != nil {
		merged.PollBackoffTime = userCfg.PollBackoffTime
	} else {
		merged.PollBackoffTime = defaultCfg.PollBackoffTime
	}

	if userCfg.PollBackoffNotify != nil {
		merged.PollBackoffNotify = userCfg.PollBackoffNotify
	} else {
		merged.PollBackoffNotify = defaultCfg.PollBackoffNotify
	}

	if userCfg.PollBackoffStopMonitor != nil {
		merged.PollBackoffStopMonitor = userCfg.PollBackoffStopMonitor
	} else {
		merged.PollBackoffStopMonitor = defaultCfg.PollBackoffStopMonitor
	}

	if userCfg.OutputDir != nil && *userCfg.OutputDir != "" {
		merged.OutputDir = ptr(filepath.Clean(*userCfg.OutputDir))
	} else {
		merged.OutputDir = defaultCfg.OutputDir
	}

	if userCfg.Verbose != nil {
		merged.Verbose = userCfg.Verbose
	} else {
		merged.Verbose = defaultCfg.Verbose
	}

	return merged, nil
}

// mergeScriptNotifierConfig merges a user-provided config with defaults.
// Any nil fields in the user config will be replaced with values from the default config.
func mergeScriptNotifierConfig(userCfg *ScriptNotifierConfig) (*ScriptNotifierConfig, error) {
	if userCfg == nil {
		return DefaultScriptNotifierConfig(), nil
	}

	merged := &ScriptNotifierConfig{}
	defaultCfg := DefaultScriptNotifierConfig()

	if userCfg.NotifyAttempts != nil {
		if *userCfg.NotifyAttempts <= 0 {
			return nil, fmt.Errorf("%w: notify_attempts must be > 0", errInvalidArgument)
		}
		merged.NotifyAttempts = userCfg.NotifyAttempts
	} else {
		merged.NotifyAttempts = defaultCfg.NotifyAttempts
	}

	if userCfg.NotifyAttemptTimeout != nil {
		merged.NotifyAttemptTimeout = userCfg.NotifyAttemptTimeout
	} else {
		merged.NotifyAttemptTimeout = defaultCfg.NotifyAttemptTimeout
	}

	if userCfg.NotifyAttemptInterval != nil {
		merged.NotifyAttemptInterval = userCfg.NotifyAttemptInterval
	} else {
		merged.NotifyAttemptInterval = defaultCfg.NotifyAttemptInterval
	}

	return merged, nil
}

// withRetries executes a fn() with retries and a onAttemptErr() callback.
func withRetries(ctx context.Context, fn func() error, onAttemptErr func(attempt int, err error), attempts int, interval time.Duration) (int, error) {
	var e error
	var attempt int

	if fn == nil {
		return attempt, fmt.Errorf("%w: function cannot be nil", errInvalidArgument)
	}

	for range attempts {
		if err := ctx.Err(); err != nil {
			return attempt, fmt.Errorf("context error: %w", err)
		}

		e = fn()
		attempt++

		if e == nil {
			return attempt, nil
		}

		if onAttemptErr != nil {
			onAttemptErr(attempt, e)
		}

		if attempt < attempts {
			select {
			case <-ctx.Done():
				return attempt, fmt.Errorf("context error: %w", ctx.Err())
			case <-time.After(interval):
			}
		}
	}

	return attempt, e
}

// recoverGoPanic recovers a panic and logs to [log.Logger] or [os.Stderr] (if nil).
func recoverGoPanic(desc string, logger *log.Logger) {
	r := recover()
	if r != nil {
		buf := debug.Stack()
		if logger != nil {
			logger.Printf("(%s) panic recovered: %v: %s", desc, r, buf)
		} else {
			fmt.Fprintf(os.Stderr, "(%s) panic recovered: %v: %s\n", desc, r, buf)
		}
	}
}
