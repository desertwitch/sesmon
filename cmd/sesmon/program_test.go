package main

import (
	"bytes"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type mockDeviceFinder struct {
	addrFound    bool
	addrResponse string
	devFound     bool
	devResponse  string
}

var _ DeviceLookuper = (*mockDeviceFinder)(nil)

func (m *mockDeviceFinder) SetAddressResponse(addrResponse string, addrFound bool) {
	m.addrResponse = addrResponse
	m.addrFound = addrFound
}

func (m *mockDeviceFinder) SetDeviceResponse(devResponse string, devFound bool) {
	m.devResponse = devResponse
	m.devFound = devFound
}

func (m *mockDeviceFinder) FindAddress(devicePath string) (string, bool) {
	return m.addrResponse, m.addrFound
}

func (m *mockDeviceFinder) FindDevice(deviceAddress string) (string, bool) {
	return m.devResponse, m.devFound
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) { //nolint:nonamedreturns
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.Write(p) //nolint:wrapcheck
}

func (sb *safeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.String()
}

// Expectation: NewProgram should use the default fsys and runner (when nil).
func Test_NewProgram_Success(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
devices:
  - device: /dev/null
    description: "Device 1"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, nil, nil, nil, &buf)

	require.NoError(t, err)

	require.NotNil(t, program)
	require.NotNil(t, program.monitors["/dev/null"].fsys)
	require.NotNil(t, program.monitors["/dev/null"].runner)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should return error for invalid YAML syntax.
func Test_NewProgram_InvalidYAMLSyntax_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failure parsing YAML")
}

// Expectation: NewProgram should return error when devices list is empty.
func Test_NewProgram_EmptyDevicesList_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	yaml := []byte(`devices: []`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no devices configured")
}

// Expectation: NewProgram should return error when devices key is missing.
func Test_NewProgram_MissingDevicesKey_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	yaml := []byte(`disable_timestamps: true`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no devices configured")
}

// Expectation: NewProgram should return error when unknown fields are present at root level.
func Test_NewProgram_UnknownFieldAtRoot_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
unknown_field: value
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "field unknown_field not found")
}

// Expectation: NewProgram should return error when unknown fields are present in device config.
func Test_NewProgram_UnknownFieldInDevice_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
    unknown_device_field: value
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "field unknown_device_field not found")
}

// Expectation: NewProgram should return error when unknown fields are present in script_notifier.
func Test_NewProgram_UnknownFieldInScriptNotifier_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/usr/local/bin/notify.sh", []byte("#!/bin/bash"), 0o755))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
    script_notifier:
      script: /usr/local/bin/notify.sh
      unknown_notifier_field: value
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "field unknown_notifier_field not found")
}

// Expectation: NewProgram should return error when unknown fields are present in monitor config.
func Test_NewProgram_UnknownFieldInMonitorConfig_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
    config:
      poll_interval: 60s
      unknown_config_field: value
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.Contains(t, err.Error(), "field unknown_config_field not found")
}

// Expectation: NewProgram should create monitors for all enabled devices.
func Test_NewProgram_AllDevicesEnabled_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Device 1"
    enabled: true
  - device: /dev/sg1
    description: "Device 2"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 2)
}

// Expectation: NewProgram should create zero monitors when all devices are disabled.
func Test_NewProgram_AllDevicesDisabled_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Device 1"
    enabled: false
  - device: /dev/sg1
    description: "Device 2"
    enabled: false
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Empty(t, monitors)
}

// Expectation: NewProgram should create monitors only for enabled devices when mixed.
func Test_NewProgram_MixedEnabledDisabled_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg2", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Enabled"
    enabled: true
  - device: /dev/sg1
    description: "Disabled"
    enabled: false
  - device: /dev/sg2
    description: "Enabled"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 2)
}

// Expectation: NewProgram should treat devices without enabled field as disabled.
func Test_NewProgram_DefaultEnabledFalse_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "No enabled field"
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Empty(t, monitors)
}

// Expectation: NewProgram should configure logger with timestamps when enabled.
func Test_NewProgram_LoggerWithTimestamps_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	ctx := t.Context()
	program.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	program.Stop()
	<-program.Done()

	output := buf.String()
	require.Contains(t, output, strconv.Itoa(time.Now().Year()))
	require.Contains(t, output, "/dev/sg0:")
}

// Expectation: NewProgram should configure logger without timestamps when disabled.
func Test_NewProgram_LoggerWithoutTimestamps_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
disable_timestamps: true
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	ctx := t.Context()
	program.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	program.Stop()
	<-program.Done()

	output := buf.String()
	require.NotContains(t, output, strconv.Itoa(time.Now().Year()))
	require.Contains(t, output, "/dev/sg0:")
}

// Expectation: NewProgram should configure separate logger prefix for each device.
func Test_NewProgram_MultipleDeviceLoggerPrefixes_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Device 0"
    enabled: true
  - device: /dev/sg1
    description: "Device 1"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	ctx := t.Context()
	program.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	program.Stop()
	<-program.Done()

	output := buf.String()
	require.Contains(t, output, "/dev/sg0:")
	require.Contains(t, output, "/dev/sg1:")
}

// Expectation: NewProgram should successfully create device with script notifier.
func Test_NewProgram_DeviceWithScriptNotifier_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/usr/local/bin/notify.sh", []byte("#!/bin/bash"), 0o755))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "With notifier"
    enabled: true
    script_notifier:
      script: /usr/local/bin/notify.sh
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)
	require.NotNil(t, program.monitors["/dev/sg0"].notifier)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should successfully create device without notifier (nil notifier).
func Test_NewProgram_DeviceWithoutNotifier_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "No notifier"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)
	require.Nil(t, program.monitors["/dev/sg0"].notifier)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should successfully create notifier with custom config.
func Test_NewProgram_NotifierWithCustomConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/usr/local/bin/notify.sh", []byte("#!/bin/bash"), 0o755))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Custom config"
    enabled: true
    script_notifier:
      script: /usr/local/bin/notify.sh
      config:
        notify_attempts: 5
        notify_attempt_timeout: 20s
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	require.NotNil(t, program.monitors["/dev/sg0"].notifier)
	n, ok := program.monitors["/dev/sg0"].notifier.(*ScriptNotifier)
	require.True(t, ok)
	require.Equal(t, "/usr/local/bin/notify.sh", n.script)
	require.Equal(t, 5, *n.cfg.NotifyAttempts)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should successfully create monitor with default config.
func Test_NewProgram_MonitorWithDefaultConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Default config"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should successfully create monitor with custom config.
func Test_NewProgram_MonitorWithCustomConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Custom config"
    enabled: true
    config:
      poll_interval: 60s
      poll_attempts: 5
      output_dir: /tmp/output
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)
	require.Equal(t, 60*time.Second, *program.monitors["/dev/sg0"].cfg.PollInterval)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
}

// Expectation: NewProgram should successfully handle complex multi-device configuration.
func Test_NewProgram_ComplexConfiguration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg2", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg3", []byte{}, 0o655))

	require.NoError(t, afero.WriteFile(fs, "/usr/local/bin/notify.sh", []byte("#!/bin/bash"), 0o755))

	yaml := []byte(`
disable_timestamps: true
devices:
  - device: /dev/sg0
    description: "Primary Storage"
    enabled: true
    config:
      poll_interval: 60s
      poll_attempts: 5
      output_dir: /var/log/sg0
    script_notifier:
      script: /usr/local/bin/notify.sh
      config:
        notify_attempts: 3
        notify_attempt_timeout: 15s

  - device: /dev/sg1
    description: "Secondary Storage"
    enabled: true
    config:
      poll_interval: 120s

  - device: /dev/sg2
    description: "Disabled device"
    enabled: false

  - device: /dev/sg3
    description: "Monitor only, no notifications"
    enabled: true
    config:
      output_dir: /var/log/sg3
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)
	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 3)
	require.Equal(t, 60*time.Second, *monitors["/dev/sg0"].cfg.PollInterval)
	require.Equal(t, 120*time.Second, *monitors["/dev/sg1"].cfg.PollInterval)
	require.Equal(t, "/var/log/sg3", *monitors["/dev/sg3"].cfg.OutputDir)
}

// Expectation: NewProgram should return error when both device and address are missing.
func Test_NewProgram_MissingDeviceAndAddress_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	yaml := []byte(`
devices:
  - description: "Missing both"
    enabled: true
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidArgument)
	require.Contains(t, err.Error(), "missing device and address")
}

// Expectation: NewProgram should successfully create monitor using address when resolved.
func Test_NewProgram_AddressResolved_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("/dev/sg0", true)

	yaml := []byte(`
devices:
  - address: "0:0:0:0"
    description: "Using address"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
	require.Equal(t, "/dev/sg0", monitors["/dev/sg0"].device.Path)
}

// Expectation: NewProgram should successfully create monitor using device path fallback.
func Test_NewProgram_AddressNotResolvedFallbackToDevice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("", false)

	yaml := []byte(`
devices:
  - address: "0:0:0:0"
    device: /dev/sg0
    description: "Fallback to device"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)
	require.Equal(t, "/dev/sg0", monitors["/dev/sg0"].device.Path)
}

// Expectation: NewProgram should return error when address cannot be resolved and no device path.
func Test_NewProgram_AddressNotResolvedNoDevice_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("", false)

	yaml := []byte(`
devices:
  - address: "0:0:0:0"
    description: "No device fallback"
    enabled: true
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, errDeviceLookupFailed)
	require.Contains(t, err.Error(), "not found")
}

// Expectation: NewProgram should suggest using address when device resolves to address.
func Test_NewProgram_DeviceResolvesToAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	finder := &mockDeviceFinder{}
	finder.SetAddressResponse("0:0:0:0", true)

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Device with address lookup"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 1)

	output := buf.String()
	require.Contains(t, output, "was resolved to")
	require.Contains(t, output, "0:0:0:0")
	require.Contains(t, output, "consider [address:")
}

// Expectation: NewProgram should work with multiple devices using mixed address/device paths.
func Test_NewProgram_MixedAddressAndDevice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg2", []byte{}, 0o644))

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("/dev/sg0", true)
	finder.SetAddressResponse("1:0:0:0", true)

	yaml := []byte(`
devices:
  - device: /dev/sg1
    description: "Using device"
    enabled: true
  - address: "2:0:0:0"
    device: /dev/sg2
    description: "Both specified"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 2)
	require.Equal(t, "/dev/sg1", monitors["/dev/sg1"].device.Path)
	require.Equal(t, "/dev/sg0", monitors["/dev/sg0"].device.Path) // resolved address to sg0
}

// Expectation: NewProgram should error when trying to monitor duplicate devices.
func Test_NewProgram_DuplicateMonitor_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg2", []byte{}, 0o644))

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("/dev/sg0", true)
	finder.SetAddressResponse("1:0:0:0", true)

	yaml := []byte(`
devices:
  - address: "0:0:0:0"
    description: "Resolves to /dev/sg0"
    enabled: true
  - device: /dev/sg1
    description: "Using device"
    enabled: true
  - address: "2:0:0:0"
    device: /dev/sg2
    description: "Resolves to /dev/sg0 also (duplicate!)"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, finder, &mockCommandRunner{}, &buf)
	require.Error(t, err)
	require.ErrorContains(t, err, "multiple")
	require.Nil(t, program)
}

// Expectation: NewProgram should handle nil finder gracefully when only device paths specified.
func Test_NewProgram_NilFinderDevicePaths_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Device 0"
    enabled: true
  - device: /dev/sg1
    description: "Device 1"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, nil, &mockCommandRunner{}, &buf)

	require.NoError(t, err)
	require.NotNil(t, program)

	monitors := program.getMonitors()
	require.Len(t, monitors, 2)
}

// Expectation: NewProgram should return error when multiple devices use the same output directory.
func Test_NewProgram_SameOutputDirs_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: ""
    enabled: true
    config:
      output_dir: /tmp
  - device: /dev/sg1
    description: ""
    enabled: true
    config:
      output_dir: /tmp
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidArgument)
	require.Contains(t, err.Error(), "same output directory")
}

// Expectation: NewProgram should return error when invalid values are present in monitor config.
func Test_NewProgram_Integration_InvalidValueInMonitorConfig_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
    config:
      poll_interval: 60s
      poll_attempts: 0
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidArgument)
}

// Expectation: NewProgram should return error when invalid values are present in notifier config.
func Test_NewProgram_Integration_InvalidValueInNotifierConfig_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/usr/local/bin/notify.sh", []byte("#!/bin/bash"), 0o755))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "With notifier"
    enabled: true
    script_notifier:
      script: /usr/local/bin/notify.sh
      config:
        notify_attempts: 0
`)

	var buf safeBuffer
	_, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidArgument)
}

// Expectation: Program should start and stop successfully.
func Test_Program_StartStop_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)
	require.NoError(t, err)

	ctx := t.Context()
	program.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	program.Stop()
	<-program.Done()

	select {
	case <-program.Done():
		require.Contains(t, buf.String(), "Monitoring")
		require.Contains(t, buf.String(), "shutting down")
	case <-time.After(2 * time.Second):
		t.Error("Program did not complete within timeout")
	}
}

// Expectation: Program should start and stop successfully.
func Test_Program_StartPanicStop_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    description: "Test"
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)
	require.NoError(t, err)

	// Deliberately cause a nil pointer dereference
	program.monitors["/dev/sg0"].cfg.PollAttempts = nil

	ctx := t.Context()
	program.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	program.Stop()
	<-program.Done()

	select {
	case <-program.Done():
		require.Contains(t, buf.String(), "panic recovered")
		require.Contains(t, buf.String(), "Monitoring")
		require.Contains(t, buf.String(), "shutting down")
	case <-time.After(2 * time.Second):
		t.Error("Program did not complete within timeout")
	}
}

// Expectation: getMonitors should return correct number of monitors.
func Test_Program_getMonitors_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    enabled: true
  - device: /dev/sg1
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)
	require.NoError(t, err)

	monitors := program.getMonitors()
	require.Len(t, monitors, 2)
}

// Expectation: getMonitors should return a copy of the slice, not the original.
func Test_Program_getMonitorsCopy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/dev/sg0", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/dev/sg1", []byte{}, 0o644))

	yaml := []byte(`
devices:
  - device: /dev/sg0
    enabled: true
  - device: /dev/sg1
    enabled: true
`)

	var buf safeBuffer
	program, err := NewProgram(yaml, fs, &mockDeviceFinder{}, &mockCommandRunner{}, &buf)
	require.NoError(t, err)

	monitors := program.getMonitors()
	delete(monitors, "/dev/sg0")

	monitors2 := program.getMonitors()
	require.NotNil(t, monitors2["/dev/sg0"])
}

// Expectation: lookupDevice should resolve address to device path when finder is available.
func Test_lookupDevice_AddressResolvedToDevice_Success(t *testing.T) {
	t.Parallel()

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("/dev/sg0", true)

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Address: "0:0:0:0",
	}

	err := lookupDevice(deviceCfg, finder, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Equal(t, "0:0:0:0", deviceCfg.Address)
}

// Expectation: lookupDevice should fall back to device path when address not found and device provided.
func Test_lookupDevice_AddressNotFoundFallbackToDevice_Success(t *testing.T) {
	t.Parallel()

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("", false)

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Address: "0:0:0:0",
		Device:  "/dev/sg0",
	}

	err := lookupDevice(deviceCfg, finder, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Empty(t, deviceCfg.Address)
	require.Contains(t, buf.String(), "is not resolvable")
	require.Contains(t, buf.String(), "using provided device path")
}

// Expectation: lookupDevice should return error when address not found and no device path.
func Test_lookupDevice_AddressNotFoundNoDevice_Error(t *testing.T) {
	t.Parallel()

	finder := &mockDeviceFinder{}
	finder.SetDeviceResponse("", false)

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Address: "0:0:0:0",
	}

	err := lookupDevice(deviceCfg, finder, logger)

	require.Error(t, err)
	require.ErrorIs(t, err, errDeviceLookupFailed)
	require.Contains(t, err.Error(), "not found")
}

// Expectation: lookupDevice should use device path and warn when no finder available but address specified.
func Test_lookupDevice_NoFinderAddressSpecifiedWithDevice_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Address: "0:0:0:0",
		Device:  "/dev/sg0",
	}

	err := lookupDevice(deviceCfg, nil, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Empty(t, deviceCfg.Address)
	require.Contains(t, buf.String(), "no lookup table")
	require.Contains(t, buf.String(), "using provided device path")
}

// Expectation: lookupDevice should return error when no finder and address specified without device.
func Test_lookupDevice_NoFinderAddressSpecifiedNoDevice_Error(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Address: "0:0:0:0",
	}

	err := lookupDevice(deviceCfg, nil, logger)

	require.Error(t, err)
	require.ErrorIs(t, err, errDeviceLookupFailed)
	require.Contains(t, err.Error(), "no lookup table")
}

// Expectation: lookupDevice should resolve device to address when only device specified.
func Test_lookupDevice_DeviceResolvedToAddress_Success(t *testing.T) {
	t.Parallel()

	finder := &mockDeviceFinder{}
	finder.SetAddressResponse("0:0:0:0", true)

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Device: "/dev/sg0",
	}

	err := lookupDevice(deviceCfg, finder, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Equal(t, "0:0:0:0", deviceCfg.Address)
	require.Contains(t, buf.String(), "was resolved to")
	require.Contains(t, buf.String(), "consider [address:")
}

// Expectation: lookupDevice should keep device path when address lookup fails.
func Test_lookupDevice_DeviceAddressNotFound_Success(t *testing.T) {
	t.Parallel()

	finder := &mockDeviceFinder{}
	finder.SetAddressResponse("", false)

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Device: "/dev/sg0",
	}

	err := lookupDevice(deviceCfg, finder, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Empty(t, deviceCfg.Address)
	require.Empty(t, buf.String())
}

// Expectation: lookupDevice should keep device path when no finder available.
func Test_lookupDevice_DeviceOnlyNoFinder_Success(t *testing.T) {
	t.Parallel()

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	deviceCfg := &DeviceYAML{
		Device: "/dev/sg0",
	}

	err := lookupDevice(deviceCfg, nil, logger)

	require.NoError(t, err)
	require.Equal(t, "/dev/sg0", deviceCfg.Device)
	require.Empty(t, deviceCfg.Address)
	require.Empty(t, buf.String())
}
