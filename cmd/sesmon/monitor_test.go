package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Test helper to create monitors without validation for internal method testing.
func newTestDeviceMonitor(t *testing.T, device Device, cfg *DeviceMonitorConfig, fsys afero.Fs, runner CommandRunner, logger *log.Logger, notifier Notifier) *DeviceMonitor {
	t.Helper()

	var err error
	cfg, err = mergeDeviceMonitorConfig(cfg)
	require.NoError(t, err)

	return &DeviceMonitor{
		device:   device,
		cfg:      cfg,
		fsys:     fsys,
		runner:   runner,
		logger:   logger,
		notifier: notifier,
		state:    newDeviceMonitorState(),
	}
}

// Expectation: NewDeviceMonitor should create a monitor with correct values.
func Test_NewDeviceMonitor_Success(t *testing.T) {
	t.Parallel()

	cfg := &DeviceMonitorConfig{
		PollInterval:           ptr(30 * time.Second),
		PollAttemptTimeout:     ptr(10 * time.Second),
		PollAttemptInterval:    ptr(time.Second),
		PollAttempts:           ptr(2),
		PollBackoffAfter:       ptr(5),
		PollBackoffTime:        ptr(5 * time.Minute),
		PollBackoffNotify:      ptr(true),
		PollBackoffStopMonitor: ptr(false),
		OutputDir:              ptr("/output"),
		Verbose:                ptr(false),
	}

	logger := log.New(io.Discard, "", 0)
	notifier := newMockNotifier()
	fsys := afero.NewMemMapFs()
	runner := &mockCommandRunner{}

	err := afero.WriteFile(fsys, "/dev/null", []byte{}, 0o644)
	require.NoError(t, err)

	m, err := NewDeviceMonitor(Device{Type: 0, Path: "/dev/null"}, cfg, fsys, runner, logger, notifier)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, cfg, m.cfg)
}

// Expectation: NewDeviceMonitor should error on no dependencies provided.
func Test_NewDeviceMonitor_NoDependencies_Error(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	notifier := newMockNotifier()
	fsys := afero.NewMemMapFs()
	runner := &mockCommandRunner{}

	err := afero.WriteFile(fsys, "/dev/null", []byte{}, 0o644)
	require.NoError(t, err)

	m, err := NewDeviceMonitor(Device{Type: 0, Path: "/dev/null"}, nil, nil, runner, logger, notifier)

	require.ErrorContains(t, err, "dependency")
	require.Nil(t, m)
}

// Expectation: NewDeviceMonitor should error on no device provided.
func Test_NewDeviceMonitor_NoDevice_Error(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	notifier := newMockNotifier()
	fsys := afero.NewMemMapFs()
	runner := &mockCommandRunner{}
	m, err := NewDeviceMonitor(Device{Type: 0, Path: ""}, nil, fsys, runner, logger, notifier)

	require.ErrorContains(t, err, "no device provided")
	require.Nil(t, m)
}

// Expectation: NewDeviceMonitor should error on not existing device provided.
func Test_NewDeviceMonitor_DeviceNotExist_Error(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	notifier := newMockNotifier()
	fsys := afero.NewMemMapFs()
	runner := &mockCommandRunner{}
	m, err := NewDeviceMonitor(Device{Type: 0, Path: "/not/exist"}, nil, fsys, runner, logger, notifier)

	require.ErrorContains(t, err, "stat device failure")
	require.Nil(t, m)
}

// Expectation: NewDeviceMonitor should provide a default configuration on nil.
func Test_NewDeviceMonitor_NoConfigGiven_Success(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	notifier := &mockNotifier{}
	fsys := afero.NewMemMapFs()
	runner := &mockCommandRunner{}

	err := afero.WriteFile(fsys, "/dev/null", []byte{}, 0o644)
	require.NoError(t, err)

	m, err := NewDeviceMonitor(Device{Type: 0, Path: "/dev/null"}, nil, fsys, runner, logger, notifier)

	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, DefaultDeviceMonitorConfig(), m.cfg)
}

// Expectation: Start should begin monitoring and close done on stop.
func Test_DeviceMonitor_Start_Stop_Done_Success(t *testing.T) {
	t.Parallel()

	var logBuf safeBuffer

	runner := &mockCommandRunner{}
	runner.setResponse(`{"join_of_diagnostic_pages":{"element_list":[]}}`, "", nil)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollInterval:        ptr(100 * time.Millisecond),
			PollAttemptTimeout:  ptr(5 * time.Second),
			PollAttemptInterval: ptr(50 * time.Millisecond),
			PollAttempts:        ptr(1),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(&logBuf, "", 0),
		newMockNotifier(),
	)

	m.Start(t.Context())
	time.Sleep(500 * time.Millisecond)
	m.Stop()

	select {
	case <-m.Done():
		require.Contains(t, logBuf.String(), "Monitoring")
		require.Contains(t, logBuf.String(), "shutting down")
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor did not stop in time")
	}
}

// Expectation: Start should stop when context is cancelled.
func Test_DeviceMonitor_Start_ContextCancelled_Error(t *testing.T) {
	t.Parallel()

	runner := &mockCommandRunner{}
	runner.setResponse(`{"join_of_diagnostic_pages":{"element_list":[]}}`, "", nil)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollInterval:        ptr(100 * time.Millisecond),
			PollAttemptTimeout:  ptr(5 * time.Second),
			PollAttemptInterval: ptr(50 * time.Millisecond),
			PollAttempts:        ptr(1),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx, cancel := context.WithCancel(t.Context())
	m.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-m.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor did not stop in time")
	}
}

// Expectation: Stop should stop the monitoring and close the stop.
func Test_DeviceMonitor_Stop_Success(t *testing.T) {
	t.Parallel()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollInterval:       ptr(30 * time.Second),
			PollAttempts:       ptr(2),
			PollAttemptTimeout: ptr(10 * time.Second),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	m.Stop()

	select {
	case <-m.state.stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stop was not closed")
	}
}

// Expectation: Stop should not panic on twice called Stop().
func Test_DeviceMonitor_StopTwice_Success(t *testing.T) {
	t.Parallel()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollInterval:       ptr(30 * time.Second),
			PollAttempts:       ptr(2),
			PollAttemptTimeout: ptr(10 * time.Second),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	m.Stop()
	m.Stop()

	select {
	case <-m.state.stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stop was not closed")
	}
}

// Expectation: The monitor should handle poll failures and increase the counters.
func Test_DeviceMonitor_PollFailure_Success(t *testing.T) {
	t.Parallel()

	var logBuf safeBuffer

	runner := &mockCommandRunner{}
	runner.setResponse("", "", errInvalidJSON)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollInterval:        ptr(100 * time.Millisecond),
			PollAttemptTimeout:  ptr(5 * time.Second),
			PollAttemptInterval: ptr(50 * time.Millisecond),
			PollAttempts:        ptr(1),
			PollBackoffAfter:    ptr(10000),
			PollBackoffTime:     ptr(time.Minute),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(&logBuf, "", 0),
		newMockNotifier(),
	)

	m.Start(t.Context())
	time.Sleep(500 * time.Millisecond)
	m.Stop()

	select {
	case <-m.Done():
		require.NotZero(t, m.state.pollFailures)
		require.Contains(t, logBuf.String(), "Monitoring")
		require.Contains(t, logBuf.String(), "shutting down")
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor did not stop in time")
	}
}

// Expectation: poll should handle first successful fetch from device.
func Test_DeviceMonitor_poll_FirstPoll_Success(t *testing.T) {
	t.Parallel()

	jsonOutput := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`
	runner := &mockCommandRunner{}
	runner.setResponse(jsonOutput, "", nil)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()
	err := m.poll(ctx)
	require.NoError(t, err)
	require.NotNil(t, m.state.previousResults)
	require.Len(t, m.state.previousResults, 1)
}

// Expectation: poll should detect and notify on changes.
func Test_DeviceMonitor_poll_DetectsChanges_Success(t *testing.T) {
	t.Parallel()

	jsonOutput1 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	jsonOutput2 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 2, "meaning": "Critical"}}
				}
			]
		}
	}`

	runner := &mockCommandRunner{}
	notifier := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		notifier,
	)

	ctx := t.Context()

	runner.setResponse(jsonOutput1, "", nil)
	err := m.poll(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, notifier.callCount())

	runner.setResponse(jsonOutput2, "", nil)
	err = m.poll(ctx)
	require.NoError(t, err)

	require.True(t, notifier.waitForNotification(2*time.Second))
	require.Equal(t, 1, notifier.callCount())

	calls := notifier.getCalls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "15#0")
}

// Expectation: poll should print verbose information on changes.
func Test_DeviceMonitor_poll_DetectsChangesVerbose_Success(t *testing.T) {
	t.Parallel()

	jsonOutput1 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	jsonOutput2 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 2, "meaning": "Critical"}}
				}
			]
		}
	}`

	var buf safeBuffer

	runner := &mockCommandRunner{}
	notifier := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
			Verbose:             ptr(true),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(&buf, "", 0),
		notifier,
	)

	ctx := t.Context()

	runner.setResponse(jsonOutput1, "", nil)
	err := m.poll(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, notifier.callCount())

	runner.setResponse(jsonOutput2, "", nil)
	err = m.poll(ctx)
	require.NoError(t, err)

	require.True(t, notifier.waitForNotification(2*time.Second))
	require.Equal(t, 1, notifier.callCount())

	calls := notifier.getCalls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "15#0")

	require.Contains(t, buf.String(), "initial elements")
	require.Contains(t, buf.String(), "Retrieved batch of")
	require.Contains(t, buf.String(), "changes detected")
}

// Expectation: poll should not notify on identical consecutive states.
func Test_DeviceMonitor_poll_NoChangeNoNotify_Success(t *testing.T) {
	t.Parallel()

	jsonOutput := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	runner := &mockCommandRunner{}
	runner.setResponse(jsonOutput, "", nil)
	notifier := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		notifier,
	)

	ctx := t.Context()

	err := m.poll(ctx)
	require.NoError(t, err)

	err = m.poll(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 0, notifier.callCount())
}

// Expectation: poll should print verbose output on no changes detected.
func Test_DeviceMonitor_poll_NoChangeVerbose_Success(t *testing.T) {
	t.Parallel()

	jsonOutput := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	var buf safeBuffer

	runner := &mockCommandRunner{}
	runner.setResponse(jsonOutput, "", nil)
	notifier := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
			Verbose:             ptr(true),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(&buf, "", 0),
		notifier,
	)

	ctx := t.Context()

	err := m.poll(ctx)
	require.NoError(t, err)

	err = m.poll(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 0, notifier.callCount())

	require.Contains(t, buf.String(), "initial elements")
	require.Contains(t, buf.String(), "Retrieved batch of")
	require.Contains(t, buf.String(), "No changes detected")
}

// Expectation: poll should return an error when parsing fails.
func Test_DeviceMonitor_poll_ParseError_Error(t *testing.T) {
	t.Parallel()

	invalidJSON := `{"invalid json structure`
	runner := &mockCommandRunner{}
	runner.setResponse(invalidJSON, "", nil)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()
	err := m.poll(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure parsing")
}

// Expectation: poll should handle notification errors gracefully.
func Test_DeviceMonitor_poll_NotificationError_Success(t *testing.T) {
	t.Parallel()

	jsonOutput1 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	jsonOutput2 := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 2, "meaning": "Critical"}}
				}
			]
		}
	}`

	runner := &mockCommandRunner{}
	notifier := newMockNotifier()
	notifier.setError(errors.New("notification failed"))

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		notifier,
	)

	ctx := t.Context()

	runner.setResponse(jsonOutput1, "", nil)
	err := m.poll(ctx)
	require.NoError(t, err)

	runner.setResponse(jsonOutput2, "", nil)
	err = m.poll(ctx)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	require.Equal(t, 1, notifier.callCount())
}

// Expectation: poll should write snapshot files when OutputDir is configured.
func Test_DeviceMonitor_poll_WritesSnapshots_Success(t *testing.T) {
	t.Parallel()

	jsonOutput := `{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {"status": {"i": 1, "meaning": "OK"}}
				}
			]
		}
	}`

	runner := &mockCommandRunner{}
	runner.setResponse(jsonOutput, "", nil)

	fsys := afero.NewMemMapFs()
	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25", Description: "test-device"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
			OutputDir:           ptr("/output"),
		},
		fsys,
		runner,
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()
	err := m.poll(ctx)
	require.NoError(t, err)

	exists, err := afero.Exists(fsys, "/output/current.json")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = afero.Exists(fsys, "/output/current_parsed.json")
	require.NoError(t, err)
	require.True(t, exists)

	var snapshot DeviceSnapshot
	data, err := afero.ReadFile(fsys, "/output/current.json")
	require.NoError(t, err)
	err = json.Unmarshal(data, &snapshot)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg25", snapshot.Device.Path)
	require.Equal(t, "test-device", snapshot.Device.Description)

	var results DeviceSnapshot
	data, err = afero.ReadFile(fsys, "/output/current_parsed.json")
	require.NoError(t, err)
	err = json.Unmarshal(data, &results)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg25", results.Device.Path)
	require.Equal(t, "test-device", results.Device.Description)
}

// Expectation: poll should write change report when changes are detected and OutputDir is set.
func Test_DeviceMonitor_poll_WritesChangeReport_Success(t *testing.T) {
	t.Parallel()

	jsonGood := `{"join_of_diagnostic_pages":{"element_list":[{"element_type":{"i":15},"element_number":0,"status_descriptor":{"status":{"i":1}}}]}}`
	jsonBad := `{"join_of_diagnostic_pages":{"element_list":[{"element_type":{"i":15},"element_number":0,"status_descriptor":{"status":{"i":2}}}]}}`

	runner := &mockCommandRunner{}
	fsys := afero.NewMemMapFs()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
			OutputDir:           ptr("/output"),
		},
		fsys,
		runner,
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()

	runner.setResponse(jsonGood, "", nil)
	err := m.poll(ctx)
	require.NoError(t, err)

	runner.setResponse(jsonBad, "", nil)
	err = m.poll(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	files, err := afero.ReadDir(fsys, "/output")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 1)

	var foundChangeReport bool
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "change-") {
			foundChangeReport = true

			break
		}
	}
	require.True(t, foundChangeReport)
}

// Expectation: fetchFromDevice should read from file when type is [DeviceTypeFile].
func Test_DeviceMonitor_fetchFromDevice_FromFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	jsonOutput := `{"join_of_diagnostic_pages":{"element_list":[]}}`
	err := afero.WriteFile(fsys, "/tmp/device.json", []byte(jsonOutput), 0o644)
	require.NoError(t, err)

	m := newTestDeviceMonitor(t,
		Device{Type: 1, Path: "/tmp/device.json"},
		DefaultDeviceMonitorConfig(),
		fsys,
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	ctx := t.Context()
	result, err := m.fetchFromDevice(ctx)
	require.NoError(t, err)
	require.JSONEq(t, jsonOutput, string(result))
}

// Expectation: fetchFromDevice should read from file when type is [DeviceTypeFile].
func Test_DeviceMonitor_fetchFromDevice_FromFile_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	jsonOutput := `{json`
	err := afero.WriteFile(fsys, "/tmp/device.json", []byte(jsonOutput), 0o644)
	require.NoError(t, err)

	cfg := DefaultDeviceMonitorConfig()
	cfg.PollAttempts = ptr(1)

	m := newTestDeviceMonitor(t,
		Device{Type: 1, Path: "/tmp/device.json"},
		cfg,
		fsys,
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	ctx := t.Context()
	_, err = m.fetchFromDevice(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidJSON)
}

// Expectation: fetchFromDevice should execute sg_ses command for [DeviceTypeDevice].
func Test_DeviceMonitor_fetchFromDevice_FromDevPath_Success(t *testing.T) {
	t.Parallel()

	jsonOutput := `{"join_of_diagnostic_pages":{"element_list":[]}}`
	runner := &mockCommandRunner{}
	runner.setResponse(jsonOutput, "", nil)

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollAttemptTimeout:  ptr(10 * time.Second),
			PollAttempts:        ptr(2),
			PollAttemptInterval: ptr(100 * time.Millisecond),
		},
		afero.NewMemMapFs(),
		runner,
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	ctx := t.Context()
	result, err := m.fetchFromDevice(ctx)
	require.NoError(t, err)
	require.JSONEq(t, jsonOutput, string(result))
	require.Equal(t, 1, runner.callCount())
}

// Expectation: fetchFromDevice should return an error when file doesn't exist.
func Test_DeviceMonitor_fetchFromDevice_FileNotExist_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	cfg := DefaultDeviceMonitorConfig()
	cfg.PollAttemptInterval = ptr(100 * time.Millisecond)

	m := newTestDeviceMonitor(t,
		Device{Type: 1, Path: "/tmp/notxist.json"},
		cfg,
		fsys,
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		&mockNotifier{},
	)

	ctx := t.Context()
	result, err := m.fetchFromDevice(ctx)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "not exist")
}

// Expectation: pollFailure should increment pollFailures counter.
func Test_DeviceMonitor_pollFailure_IncrementCounter_Success(t *testing.T) {
	t.Parallel()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollBackoffAfter: ptr(5),
			PollBackoffTime:  ptr(30 * time.Second),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()
	err := errors.New("test error")

	m.pollFailure(ctx, err)
	require.Equal(t, 1, m.state.pollFailures)

	m.pollFailure(ctx, err)
	require.Equal(t, 2, m.state.pollFailures)
}

// Expectation: pollFailure should trigger backoff after threshold and reset counter.
func Test_DeviceMonitor_pollFailure_Backoff_Success(t *testing.T) {
	t.Parallel()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollBackoffAfter:       ptr(3),
			PollBackoffTime:        ptr(200 * time.Millisecond),
			PollBackoffNotify:      ptr(false),
			PollBackoffStopMonitor: ptr(false),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx := t.Context()
	err := errors.New("test error")

	m.pollFailure(ctx, err)
	require.Equal(t, 1, m.state.pollFailures)

	m.pollFailure(ctx, err)
	require.Equal(t, 2, m.state.pollFailures)

	start := time.Now()
	m.pollFailure(ctx, err)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	require.Equal(t, 0, m.state.pollFailures)
}

// Expectation: pollFailure should trigger backoff notification when configured.
func Test_DeviceMonitor_pollFailure_Backoff_Notify_Success(t *testing.T) {
	t.Parallel()

	n := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollBackoffAfter:       ptr(3),
			PollBackoffTime:        ptr(200 * time.Millisecond),
			PollBackoffNotify:      ptr(true),
			PollBackoffStopMonitor: ptr(false),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		n,
	)

	ctx := t.Context()
	err := errors.New("test error")

	m.pollFailure(ctx, err)
	require.Equal(t, 1, m.state.pollFailures)

	m.pollFailure(ctx, err)
	require.Equal(t, 2, m.state.pollFailures)

	start := time.Now()
	m.pollFailure(ctx, err)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	require.Equal(t, 0, m.state.pollFailures)
	require.Equal(t, 1, n.callCount())
}

// Expectation: pollFailure should trigger backoff monitor stop when configured.
func Test_DeviceMonitor_pollFailure_Backoff_Stop_Success(t *testing.T) {
	t.Parallel()

	n := newMockNotifier()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollBackoffAfter:       ptr(3),
			PollBackoffTime:        ptr(200 * time.Millisecond),
			PollBackoffNotify:      ptr(true),
			PollBackoffStopMonitor: ptr(true),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		n,
	)

	ctx := t.Context()
	err := errors.New("test error")

	m.pollFailure(ctx, err)
	require.Equal(t, 1, m.state.pollFailures)

	m.pollFailure(ctx, err)
	require.Equal(t, 2, m.state.pollFailures)

	m.pollFailure(ctx, err)

	select {
	case <-m.state.stop:
		time.Sleep(500 * time.Millisecond)
		require.Equal(t, 1, n.callCount())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Monitoring did not stop once entering back-off period")
	}
}

// Expectation: pollFailure should stop backoff when context is cancelled.
func Test_DeviceMonitor_pollFailure_Backoff_ContextCancelled_Error(t *testing.T) {
	t.Parallel()

	m := newTestDeviceMonitor(t,
		Device{Type: 0, Path: "/dev/sg25"},
		&DeviceMonitorConfig{
			PollBackoffAfter:       ptr(1),
			PollBackoffTime:        ptr(10 * time.Second),
			PollBackoffNotify:      ptr(false),
			PollBackoffStopMonitor: ptr(false),
		},
		afero.NewMemMapFs(),
		&mockCommandRunner{},
		log.New(io.Discard, "", 0),
		newMockNotifier(),
	)

	ctx, cancel := context.WithCancel(t.Context())
	err := errors.New("test error")

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	m.pollFailure(ctx, err)
	elapsed := time.Since(start)

	require.Less(t, elapsed, 5*time.Second)
}

// Expectation: pollFailure should return immediately when context is already cancelled.
func Test_DeviceMonitor_pollFailure_ContextCancelledBefore_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	pollBackoffAfter := 3
	dm := &DeviceMonitor{
		logger: logger,
		cfg: &DeviceMonitorConfig{
			PollBackoffAfter: &pollBackoffAfter,
		},
		state: newDeviceMonitorState(),
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	testErr := errors.New("test error")
	dm.pollFailure(ctx, testErr)

	require.Equal(t, 0, dm.state.pollFailures)
	require.Empty(t, buf.String())
}

// Expectation: pollFailure should return immediately when stop channel is closed before call.
func Test_DeviceMonitor_pollFailure_StopChannelClosedBefore_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	pollBackoffAfter := 3
	dm := &DeviceMonitor{
		logger: logger,
		cfg: &DeviceMonitorConfig{
			PollBackoffAfter: &pollBackoffAfter,
		},
		state: newDeviceMonitorState(),
	}

	close(dm.state.stop)

	testErr := errors.New("test error")
	dm.pollFailure(t.Context(), testErr)

	require.Equal(t, 0, dm.state.pollFailures)
	require.Empty(t, buf.String())
}

// Expectation: pollFailure should return when stop channel is closed during backoff wait.
func Test_DeviceMonitor_pollFailure_StopChannelClosedDuringBackoff_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	pollBackoffAfter := 1
	pollBackoffTime := 5 * time.Second
	pollBackoffStopMonitor := false
	dm := &DeviceMonitor{
		logger: logger,
		cfg: &DeviceMonitorConfig{
			PollBackoffAfter:       &pollBackoffAfter,
			PollBackoffTime:        &pollBackoffTime,
			PollBackoffStopMonitor: &pollBackoffStopMonitor,
		},
		state: newDeviceMonitorState(),
	}

	testErr := errors.New("test error")

	done := make(chan struct{})
	go func() {
		dm.pollFailure(t.Context(), testErr)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	close(dm.state.stop)

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("pollFailure did not return after stop channel closure")
	case <-done:
	}

	require.Equal(t, 1, dm.state.pollFailures)
	output := buf.String()
	require.Contains(t, output, "entering 5s back-off")
}

// Expectation: pollFailure should call Stop when PollBackoffStopMonitor is true.
func Test_DeviceMonitor_pollFailure_BackoffStopMonitor_CallsStop_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	pollBackoffAfter := 1
	pollBackoffStopMonitor := true
	dm := &DeviceMonitor{
		logger: logger,
		cfg: &DeviceMonitorConfig{
			PollBackoffAfter:       &pollBackoffAfter,
			PollBackoffStopMonitor: &pollBackoffStopMonitor,
		},
		state: newDeviceMonitorState(),
	}

	testErr := errors.New("test error")
	dm.pollFailure(t.Context(), testErr)

	select {
	case <-dm.state.stop:
	default:
		t.Fatal("Stop() was not called")
	}

	output := buf.String()
	require.Contains(t, output, "stopping device monitor")
}
