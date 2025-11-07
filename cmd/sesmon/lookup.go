package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// DeviceLookuper is the contract for a SAS device resolver as part of a [Program].
type DeviceLookuper interface {
	FindAddress(devicePath string) (string, bool)
	FindDevice(deviceAddress string) (string, bool)
}

var _ DeviceLookuper = (*DeviceFinder)(nil)

// DeviceFinder is the principal [DeviceLookuper] implementation.
type DeviceFinder struct {
	devices map[string]string
}

// NewDeviceFinder returns a pointer to a new [DeviceFinder].
func NewDeviceFinder(fsys afero.Fs, logger *log.Logger) (*DeviceFinder, error) {
	devices := map[string]string{}
	ignored := map[string]struct{}{}

	matches, err := afero.Glob(fsys, "/sys/class/scsi_generic/sg*/device")
	if err != nil {
		return nil, fmt.Errorf("glob failure: %w", err)
	}

	for _, d := range matches {
		sasb, err := afero.ReadFile(fsys, filepath.Join(d, "sas_address"))
		if err != nil {
			continue
		}
		sas := strings.ToLower(strings.TrimSpace(string(sasb)))
		if sas == "" {
			continue
		}
		sg := "/dev/" + filepath.Base(filepath.Dir(d)) // sgN
		if _, ok := devices[sas]; ok {
			ignored[sas] = struct{}{}
		}
		devices[sas] = sg
	}

	for k := range ignored {
		logger.Printf("Warning: SAS address [%s] came up for multiple devices "+
			"(ignoring it for address lookups)", k)
		delete(devices, k)
	}

	return &DeviceFinder{
		devices: devices,
	}, nil
}

// FindAddress tries to resolve a device path to a SAS address.
func (f *DeviceFinder) FindAddress(devicePath string) (string, bool) {
	for k, v := range f.devices {
		if v == devicePath {
			return k, true
		}
	}

	return "", false
}

// FindDevice tries to resolve a SAS address to a device path.
func (f *DeviceFinder) FindDevice(deviceAddress string) (string, bool) {
	if v, ok := f.devices[deviceAddress]; ok {
		return v, true
	}

	return "", false
}
