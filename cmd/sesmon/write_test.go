package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type mockFs struct {
	afero.Fs

	mkdirAllErr  error
	writeFileErr error
}

func (m *mockFs) MkdirAll(path string, perm os.FileMode) error {
	if m.mkdirAllErr != nil {
		return m.mkdirAllErr
	}
	if m.Fs != nil {
		return m.Fs.MkdirAll(path, perm) //nolint:wrapcheck
	}

	return nil
}

func (m *mockFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if m.writeFileErr != nil {
		return nil, m.writeFileErr
	}
	if m.Fs != nil {
		return m.Fs.OpenFile(name, flag, perm) //nolint:wrapcheck
	}

	return nil, errors.New("no filesystem")
}

// Expectation: ensureDeviceFolder should create the device directory.
func Test_DeviceMonitor_ensureDeviceFolder_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	m := &DeviceMonitor{
		device: Device{Type: 0, Path: "/dev/sg25"},
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys: fsys,
	}

	deviceDir, err := m.ensureDeviceFolder()
	require.NoError(t, err)
	require.Equal(t, "/output", deviceDir)

	exists, err := afero.DirExists(fsys, "/output")
	require.NoError(t, err)
	require.True(t, exists)
}

// Expectation: ensureDeviceFolder should not fail if directory already exists.
func Test_DeviceMonitor_ensureDeviceFolder_AlreadyExists_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/output", 0o755))

	m := &DeviceMonitor{
		device: Device{Type: 0, Path: "/dev/sg25"},
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys: fsys,
	}

	deviceDir, err := m.ensureDeviceFolder()
	require.NoError(t, err)
	require.Equal(t, "/output", deviceDir)
}

// Expectation: writeDeviceSnapshot should write snapshot to file.
func Test_DeviceMonitor_writeDeviceSnapshot_Success(t *testing.T) {
	t.Parallel()

	dev := Device{Type: 0, Path: "/dev/sg25", Description: "test-device"}

	fsys := afero.NewMemMapFs()
	m := &DeviceMonitor{
		device: dev,
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys:   fsys,
		logger: log.New(io.Discard, "", 0),
	}

	snapshot := DeviceSnapshot{
		Device:     dev,
		CapturedAt: "2025-01-01T12:00:00Z",
		Raw:        json.RawMessage(`{"key":"value"}`),
	}

	err := m.writeDeviceSnapshot(snapshot, "snapshot.json")
	require.NoError(t, err)

	data, err := afero.ReadFile(fsys, "/output/snapshot.json")
	require.NoError(t, err)

	var loaded DeviceSnapshot
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	require.Equal(t, "/dev/sg25", loaded.Device.Path)
	require.Equal(t, "test-device", loaded.Device.Description)
	require.Equal(t, "2025-01-01T12:00:00Z", loaded.CapturedAt)
	require.JSONEq(t, `{"key":"value"}`, string(loaded.Raw))
}

// Expectation: writeDeviceSnapshot should overwrite existing snapshot.
func Test_DeviceMonitor_writeDeviceSnapshot_Overwrite_Success(t *testing.T) {
	t.Parallel()

	dev := Device{Type: 0, Path: "/dev/sg25", Description: "test-device"}

	fsys := afero.NewMemMapFs()
	m := &DeviceMonitor{
		device: dev,
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys:   fsys,
		logger: log.New(io.Discard, "", 0),
	}

	snapshot1 := DeviceSnapshot{
		Device:     dev,
		CapturedAt: "2025-01-01T12:00:00Z",
		Raw:        json.RawMessage(`{"version":1}`),
	}
	err := m.writeDeviceSnapshot(snapshot1, "snapshot.json")
	require.NoError(t, err)

	snapshot2 := DeviceSnapshot{
		Device:     dev,
		CapturedAt: "2025-01-01T13:00:00Z",
		Raw:        json.RawMessage(`{"version":2}`),
	}
	err = m.writeDeviceSnapshot(snapshot2, "snapshot.json")
	require.NoError(t, err)

	data, err := afero.ReadFile(fsys, "/output/snapshot.json")
	require.NoError(t, err)

	var loaded DeviceSnapshot
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	require.JSONEq(t, `{"version":2}`, string(loaded.Raw))
}

// Expectation: writeChangeReport should write change report with timestamp.
func Test_DeviceMonitor_writeChangeReport_Success(t *testing.T) {
	t.Parallel()

	dev := Device{Type: 0, Path: "/dev/sg25", Description: "test-device"}

	fsys := afero.NewMemMapFs()
	m := &DeviceMonitor{
		device: dev,
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys:   fsys,
		logger: log.New(io.Discard, "", 0),
	}

	report := ChangeReport{
		Device:     dev,
		DetectedAt: "2025-01-01T12:00:00Z",
		Changes: []Change{
			{
				ID:       "15#0",
				Type:     15,
				TypeDesc: ptr("Enclosure"),
				TypeNum:  0,
				Before:   &Result{Status: ptr(1)},
				After:    &Result{Status: ptr(2)},
			},
		},
	}

	err := m.writeChangeReport(report)
	require.NoError(t, err)

	files, err := afero.ReadDir(fsys, "/output")
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files[0].Name(), "change-")
	require.Contains(t, files[0].Name(), ".json")

	data, err := afero.ReadFile(fsys, "/output/"+files[0].Name())
	require.NoError(t, err)

	var loaded ChangeReport
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	require.Equal(t, "/dev/sg25", loaded.Device.Path)
	require.Len(t, loaded.Changes, 1)
}

// Expectation: writeChangeReport should create multiple reports without overwriting.
func Test_DeviceMonitor_writeChangeReport_MultipleReports_Success(t *testing.T) {
	t.Parallel()

	dev := Device{Type: 0, Path: "/dev/sg25", Description: "test-device"}

	fsys := afero.NewMemMapFs()
	m := &DeviceMonitor{
		device: dev,
		cfg: &DeviceMonitorConfig{
			OutputDir: ptr("/output"),
		},
		fsys:   fsys,
		logger: log.New(io.Discard, "", 0),
	}

	report1 := ChangeReport{
		Device:     dev,
		DetectedAt: "2025-01-01T12:00:00Z",
		Changes:    []Change{{ID: "15#0", Type: 15, TypeNum: 0}},
	}
	err := m.writeChangeReport(report1)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // Ensure different timestamp

	report2 := ChangeReport{
		Device:     dev,
		DetectedAt: "2025-01-01T13:00:00Z",
		Changes:    []Change{{ID: "23#1", Type: 23, TypeNum: 1}},
	}
	err = m.writeChangeReport(report2)
	require.NoError(t, err)

	files, err := afero.ReadDir(fsys, "/output")
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// Expectation: ensureDeviceFolder should return error when MkdirAll fails.
func Test_DeviceMonitor_ensureDeviceFolder_MkdirAllError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mkdir error")
	fsys := &mockFs{
		mkdirAllErr: expectedErr,
	}
	outputDir := "/test/output"
	dm := &DeviceMonitor{
		fsys: fsys,
		cfg: &DeviceMonitorConfig{
			OutputDir: &outputDir,
		},
	}

	path, err := dm.ensureDeviceFolder()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure creating directory")
	require.Empty(t, path)
}

// Expectation: writeDeviceSnapshot should return error when ensureDeviceFolder fails.
func Test_DeviceMonitor_writeDeviceSnapshot_EnsureFolderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mkdir error")
	fsys := &mockFs{
		mkdirAllErr: expectedErr,
	}
	outputDir := "/test/output"
	dm := &DeviceMonitor{
		fsys: fsys,
		cfg: &DeviceMonitorConfig{
			OutputDir: &outputDir,
		},
	}

	snapshot := DeviceSnapshot{
		Device:     Device{Path: "/dev/sg0"},
		CapturedAt: time.Now().Format(time.RFC3339),
		Raw:        json.RawMessage(`{}`),
	}

	err := dm.writeDeviceSnapshot(snapshot, "test.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure ensuring folder")
}

// Expectation: writeDeviceSnapshot should return error when WriteFile fails.
func Test_DeviceMonitor_writeDeviceSnapshot_WriteFileError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("write error")
	fsys := &mockFs{
		Fs:           afero.NewMemMapFs(),
		writeFileErr: expectedErr,
	}
	outputDir := "/test/output"
	dm := &DeviceMonitor{
		fsys: fsys,
		cfg: &DeviceMonitorConfig{
			OutputDir: &outputDir,
		},
	}

	snapshot := DeviceSnapshot{
		Device:     Device{Path: "/dev/sg0"},
		CapturedAt: time.Now().Format(time.RFC3339),
		Raw:        json.RawMessage(`{}`),
	}

	err := dm.writeDeviceSnapshot(snapshot, "test.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure writing to file")
}

// Expectation: writeChangeReport should return error when ensureDeviceFolder fails.
func Test_DeviceMonitor_writeChangeReport_EnsureFolderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mkdir error")
	fsys := &mockFs{
		mkdirAllErr: expectedErr,
	}
	outputDir := "/test/output"
	dm := &DeviceMonitor{
		fsys: fsys,
		cfg: &DeviceMonitorConfig{
			OutputDir: &outputDir,
		},
	}

	report := ChangeReport{
		Device:     Device{Path: "/dev/sg0"},
		DetectedAt: time.Now().Format(time.RFC3339),
		Changes:    []Change{},
	}

	err := dm.writeChangeReport(report)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure ensuring folder")
}

// Expectation: writeChangeReport should return error when WriteFile fails.
func Test_DeviceMonitor_writeChangeReport_WriteFileError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("write error")
	fsys := &mockFs{
		Fs:           afero.NewMemMapFs(),
		writeFileErr: expectedErr,
	}
	outputDir := "/test/output"
	dm := &DeviceMonitor{
		fsys: fsys,
		cfg: &DeviceMonitorConfig{
			OutputDir: &outputDir,
		},
	}

	report := ChangeReport{
		Device:     Device{Path: "/dev/sg0"},
		DetectedAt: time.Now().Format(time.RFC3339),
		Changes:    []Change{},
	}

	err := dm.writeChangeReport(report)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure writing to file")
}
