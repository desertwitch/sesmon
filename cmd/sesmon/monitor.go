package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/spf13/afero"
)

const (
	DeviceTypeDevice = 0
	DeviceTypeFile   = 1
)

type DeviceMonitorConfig struct {
	// How often to poll the target device for data.
	PollInterval *time.Duration `yaml:"poll_interval"`

	// How often to attempt a device poll (must be > 0).
	PollAttempts *int `yaml:"poll_attempts"`

	// How long a device poll attempt can take (multiplies with attempts).
	PollAttemptTimeout *time.Duration `yaml:"poll_attempt_timeout"`

	// How long to wait between device poll attempts (in case of failure).
	PollAttemptInterval *time.Duration `yaml:"poll_attempt_interval"`

	// How many consecutive poll failures trigger back-off period.
	// Note: First failure = after 3 attempts (set value of poll_attempts),
	// so backoff after 3 failures = after total 9 failed poll attempts.
	PollBackoffAfter *int `yaml:"poll_backoff_after"`

	// How long to pause polling the device when in back-off period.
	PollBackoffTime *time.Duration `yaml:"poll_backoff_time"`

	// Dispatch notification through agent when entering back-off period.
	// Applies only if a notification agent is configured for the device.
	PollBackoffNotify *bool `yaml:"poll_backoff_notify"`

	// Permanently stop monitoring the device when entering back-off period.
	// If false, monitoring resumes normally after [PollBackoffTime] elapses.
	PollBackoffStopMonitor *bool `yaml:"poll_backoff_stopmonitor"`

	// Folder to write JSON files of device state and alerts to.
	// Must be unique per device and creates the following files:
	//  - current.json (raw snapshot of current device state)
	//  - current_parsed.json (parsed snapshot of current device state)
	//  - change-YYYYMMDD-HHMMSS.json (single timestamped change report)
	//  - ...
	OutputDir *string `yaml:"output_dir"`

	// Output also verbose operational information as part of log output.
	Verbose *bool `yaml:"verbose"`
}

// MarshalJSON is a custom JSON marshaller for user readable [time.Duration] strings.
func (c DeviceMonitorConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct { //nolint:wrapcheck
		PollInterval           *string `json:"poll_interval"`
		PollAttempts           *int    `json:"poll_attempts"`
		PollAttemptTimeout     *string `json:"poll_attempt_timeout"`
		PollAttemptInterval    *string `json:"poll_attempt_interval"`
		PollBackoffAfter       *int    `json:"poll_backoff_after"`
		PollBackoffTime        *string `json:"poll_backoff_time"`
		PollBackoffNotify      *bool   `json:"poll_backoff_notify"`
		PollBackoffStopMonitor *bool   `json:"poll_backoff_stopmonitor"`
		OutputDir              *string `json:"output_dir"`
		Verbose                *bool   `json:"verbose"`
	}{
		PollInterval:           durPtrToStrPtr(c.PollInterval),
		PollAttempts:           c.PollAttempts,
		PollAttemptTimeout:     durPtrToStrPtr(c.PollAttemptTimeout),
		PollAttemptInterval:    durPtrToStrPtr(c.PollAttemptInterval),
		PollBackoffAfter:       c.PollBackoffAfter,
		PollBackoffTime:        durPtrToStrPtr(c.PollBackoffTime),
		PollBackoffNotify:      c.PollBackoffNotify,
		PollBackoffStopMonitor: c.PollBackoffStopMonitor,
		OutputDir:              c.OutputDir,
		Verbose:                c.Verbose,
	})
}

// DefaultDeviceMonitorConfig returns a pointer to a default [DeviceMonitorConfig].
//
//nolint:mnd
func DefaultDeviceMonitorConfig() *DeviceMonitorConfig {
	return &DeviceMonitorConfig{
		PollInterval:           ptr(90 * time.Second),
		PollAttempts:           ptr(3),
		PollAttemptTimeout:     ptr(15 * time.Second),
		PollAttemptInterval:    ptr(15 * time.Second),
		PollBackoffAfter:       ptr(3),
		PollBackoffTime:        ptr(3 * time.Minute),
		PollBackoffNotify:      ptr(true),
		PollBackoffStopMonitor: ptr(false),
		OutputDir:              nil,
		Verbose:                ptr(false),
	}
}

// deviceMonitorState is the state of a [DeviceMonitor].
type deviceMonitorState struct {
	// Amount of poll failures for the device (resets with back-off period).
	pollFailures int

	// Hash of the last alert that has been raised (to avoid duplicate alerts).
	lastAlertHash string

	// Map of the previous poll [Result] for comparison against current.
	previousResults map[string]Result

	// Stop is only allowed to run once, this [sync.Once] ensures that.
	once sync.Once

	// Closing of stop signals the monitor for the device to stop.
	stop chan struct{}

	// Closing of done signals the consumers that the monitor is done.
	done chan struct{}
}

type DeviceMonitor struct {
	device Device

	fsys     afero.Fs
	runner   CommandRunner
	notifier Notifier
	logger   *log.Logger

	cfg   *DeviceMonitorConfig
	state *deviceMonitorState
}

// newDeviceMonitorState returns a pointer to a new [deviceMonitorState].
func newDeviceMonitorState() *deviceMonitorState {
	return &deviceMonitorState{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// NewDeviceMonitor returns a pointer to a new [DeviceMonitor].
func NewDeviceMonitor(
	device Device,
	cfg *DeviceMonitorConfig,
	fsys afero.Fs, runner CommandRunner, logger *log.Logger, notifier Notifier,
) (*DeviceMonitor, error) {
	if fsys == nil || runner == nil || logger == nil {
		return nil, fmt.Errorf("%w: required dependency is nil", errInvalidArgument)
	}
	if device.Path == "" {
		return nil, fmt.Errorf("%w: no device provided", errInvalidArgument)
	}
	if _, err := fsys.Stat(device.Path); err != nil {
		return nil, fmt.Errorf("%w: stat device failure: %w", errInvalidArgument, err)
	}

	mcfg, err := mergeDeviceMonitorConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("configuration failure: %w", err)
	}

	m := &DeviceMonitor{
		device:   device,
		fsys:     fsys,
		runner:   runner,
		logger:   logger,
		notifier: notifier,
		cfg:      mcfg,
		state:    newDeviceMonitorState(),
	}

	return m, nil
}

// Stop stops the monitoring for the device.
func (d *DeviceMonitor) Stop() {
	d.state.once.Do(func() {
		d.logger.Println("Monitoring for this device is shutting down...")
		close(d.state.stop)
	})
}

// Done returns a channel that is closed when monitoring has stopped.
func (d *DeviceMonitor) Done() <-chan struct{} {
	return d.state.done
}

// Start starts monitoring of the device.
// The context is both observed and respected for earlier termination.
func (d *DeviceMonitor) Start(ctx context.Context) {
	cfgJSON, err := json.Marshal(d.cfg)
	if err != nil {
		cfgJSON = []byte("n/a")
	}
	if d.notifier == nil {
		d.logger.Printf("Monitoring [%s:%s] with configuration [%s]; "+
			"and no notification agent", d.device.Path, d.device.Address, cfgJSON)
	} else {
		d.logger.Printf("Monitoring [%s:%s] with configuration [%s]; "+
			"and notification agent [%s] with configuration [%s]",
			d.device.Path, d.device.Address, cfgJSON, d.notifier.Name(), d.notifier.Config())
	}

	go func() {
		defer recoverGoPanic("monitor", d.logger)
		defer close(d.state.done)
		defer d.Stop()

		if err := d.poll(ctx); err != nil {
			d.pollFailure(ctx, err)
		}

		ticker := time.NewTicker(*d.cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-d.state.stop:
				return
			case <-ticker.C:
				if err := d.poll(ctx); err != nil {
					d.pollFailure(ctx, err)
				}
			}
		}
	}()
}

// poll is a device polling attempt (including any retries on failure).
func (d *DeviceMonitor) poll(ctx context.Context) error {
	ret, err := d.fetchFromDevice(ctx)
	if err != nil {
		return fmt.Errorf("failure fetching from device: %w", err)
	}

	currentResults, err := parseSES(ret)
	if err != nil {
		return fmt.Errorf("failure parsing fetched data: %w", err)
	}

	defer func() {
		d.state.previousResults = currentResults
	}()

	if d.cfg.OutputDir != nil {
		d.writeCurrentData(ret, currentResults)
	}

	if d.state.previousResults == nil {
		d.logger.Printf("Retrieved %d initial elements from SES-capable device",
			len(currentResults))

		return nil
	} else if *d.cfg.Verbose {
		d.logger.Printf("Retrieved batch of %d elements from SES-capable device",
			len(currentResults))
	}

	changes := rowsDiff(d.state.previousResults, currentResults)
	if len(changes) == 0 {
		if *d.cfg.Verbose {
			d.logger.Println("No changes detected comparing previous vs. current results")
		}

		return nil
	} else if *d.cfg.Verbose {
		d.logger.Printf("%d changes detected comparing previous vs. current results",
			len(changes))
	}

	report := ChangeReport{
		Device:     d.device,
		DetectedAt: time.Now().Format(time.RFC3339),
		Changes:    changes,
	}

	msg := buildMessage(changesAsText(changes))
	h := sha256.Sum256([]byte(msg))
	hash := hex.EncodeToString(h[:])

	if d.state.lastAlertHash != "" && d.state.lastAlertHash == hash {
		d.logger.Println("Alert changes match the previous alert - skipping notification")
	} else {
		d.handleAlert(ctx, hash, msg, report)
	}

	return nil
}

// fetchFromDevice tries to fetch the SES information from the device.
// If the device path starts with "/dev" it uses sg_ses, otherwise it
// tries to open the device path as a file and expects it to contain JSON.
func (d *DeviceMonitor) fetchFromDevice(ctx context.Context) ([]byte, error) {
	if d.device.Type == DeviceTypeFile {
		var by []byte

		attempt, err := withRetries(
			ctx,
			func() error {
				var err error

				by, err = afero.ReadFile(d.fsys, d.device.Path)
				if err != nil {
					return fmt.Errorf("failure reading from file: %w", err)
				}
				if !json.Valid(by) {
					return fmt.Errorf("failure parsing from file: %w", errInvalidJSON)
				}

				return nil
			},
			func(attempt int, err error) {
				d.logger.Printf("[%d/%d] %v", attempt, *d.cfg.PollAttempts, err)
			},
			*d.cfg.PollAttempts,
			*d.cfg.PollAttemptInterval,
		)
		if err != nil {
			return nil, fmt.Errorf("[%d/%d] %w", attempt, *d.cfg.PollAttempts, err)
		}

		return by, nil
	}

	stdout, _, err := d.runner.Run(ctx, RunCommandConfig{
		Description:     fmt.Sprintf("%q", "sg_ses"),
		Command:         "sg_ses",
		Args:            []string{"--all", "--json", d.device.Path},
		Attempts:        *d.cfg.PollAttempts,
		AttemptTimeout:  *d.cfg.PollAttemptTimeout,
		AttemptInterval: *d.cfg.PollAttemptInterval,
		ExpectJSON:      true,
		PrintErrors:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("%q: %w", "sg_ses", err)
	}

	return []byte(stdout), nil
}

// writeCurrentData writes the current map[string]Result to JSON snapshot files.
func (d *DeviceMonitor) writeCurrentData(raw []byte, parsed map[string]Result) {
	snapshot := DeviceSnapshot{
		Device:     d.device,
		CapturedAt: time.Now().Format(time.RFC3339),
		Raw:        json.RawMessage(raw),
	}
	if err := d.writeDeviceSnapshot(snapshot, "current.json"); err != nil {
		d.logger.Printf("Error writing device snapshot to file: %v", err)
	}

	results, err := json.MarshalIndent(parsed, "", "  ")
	if err == nil {
		snapshot.Raw = json.RawMessage(results)
		if err := d.writeDeviceSnapshot(snapshot, "current_parsed.json"); err != nil {
			d.logger.Printf("Error writing parsed device snapshot to file: %v", err)
		}
	} else {
		d.logger.Printf("Error marshalling parsed device snapshot to JSON: %v", err)
	}
}

// handleAlert handles alerting for a slice of [Change] with a given message.
// If no notification agent was configured, it only emits the alert to log output.
func (d *DeviceMonitor) handleAlert(ctx context.Context, hash string, msg string, report ChangeReport) {
	d.logger.Println("Alert:", msg)

	if d.notifier != nil {
		go func() {
			defer recoverGoPanic("alert-notifier", d.logger)
			if err := d.notifier.Notify(ctx, d.device, msg, report); err != nil {
				d.logger.Printf("Alert notification agent error: %v", err)
			}
		}()
	}

	if d.cfg.OutputDir != nil {
		if err := d.writeChangeReport(report); err != nil {
			d.logger.Printf("Error writing change report to file: %v", err)
		}
	}

	d.state.lastAlertHash = hash
}

// pollFailure is called after a single device poll (including retries) has failed.
// It handles the back-off period alerting, stopping and time (if configured to do so).
// It both observes and respects the given context for earlier termination.
func (d *DeviceMonitor) pollFailure(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case <-d.state.stop:
		return
	default:
	}

	d.state.pollFailures++

	if d.state.pollFailures < *d.cfg.PollBackoffAfter {
		d.logger.Printf("Error polling device [%d/%d]: %v",
			d.state.pollFailures, *d.cfg.PollBackoffAfter, err)
	} else {
		var msg string
		if *d.cfg.PollBackoffStopMonitor {
			msg = fmt.Sprintf("Error polling device [%d/%d] (stopping device monitor): %v",
				d.state.pollFailures, *d.cfg.PollBackoffAfter, err)
		} else {
			msg = fmt.Sprintf("Error polling device [%d/%d] (entering %s back-off): %v",
				d.state.pollFailures, *d.cfg.PollBackoffAfter, *d.cfg.PollBackoffTime, err)
		}

		d.logger.Println(msg)

		if d.notifier != nil && *d.cfg.PollBackoffNotify {
			go func() {
				defer recoverGoPanic("failure-notifier", d.logger)
				if err := d.notifier.Notify(ctx, d.device, msg, nil); err != nil {
					d.logger.Printf("Alert notification agent error: %v", err)
				}
			}()
		}

		if *d.cfg.PollBackoffStopMonitor {
			d.Stop()

			return
		}

		select {
		case <-ctx.Done():
			return
		case <-d.state.stop:
			return
		case <-time.After(*d.cfg.PollBackoffTime):
		}

		d.state.pollFailures = 0
	}
}
