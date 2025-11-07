package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
)

const (
	baseFilePerms   = 0o666
	baseFolderPerms = 0o777
)

// ensureDeviceFolder ensures that [DeviceMonitorConfig.OutputDir] exists.
func (d *DeviceMonitor) ensureDeviceFolder() (string, error) {
	if err := d.fsys.MkdirAll(*d.cfg.OutputDir, baseFolderPerms); err != nil {
		return "", fmt.Errorf("failure creating directory: %w", err)
	}

	return *d.cfg.OutputDir, nil
}

// writeDeviceSnapshot writes a [DeviceSnapshot] to a JSON file.
func (d *DeviceMonitor) writeDeviceSnapshot(snapshot DeviceSnapshot, filename string) error {
	deviceDir, err := d.ensureDeviceFolder()
	if err != nil {
		return fmt.Errorf("failure ensuring folder: %w", err)
	}

	currentPath := filepath.Join(deviceDir, filename)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failure marshalling to JSON: %w", err)
	}

	if err := afero.WriteFile(d.fsys, currentPath, data, baseFilePerms); err != nil {
		return fmt.Errorf("failure writing to file: %w", err)
	}

	return nil
}

// writeChangeReport writes a [ChangeReport] to a time-stamped JSON file.
func (d *DeviceMonitor) writeChangeReport(report ChangeReport) error {
	deviceDir, err := d.ensureDeviceFolder()
	if err != nil {
		return fmt.Errorf("failure ensuring folder: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("change-%s.json", timestamp)
	reportPath := filepath.Join(deviceDir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failure marshalling to JSON: %w", err)
	}

	if err := afero.WriteFile(d.fsys, reportPath, data, baseFilePerms); err != nil {
		return fmt.Errorf("failure writing to file: %w", err)
	}

	return nil
}
