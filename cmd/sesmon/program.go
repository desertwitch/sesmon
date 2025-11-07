package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"sync"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

var (
	// errDeviceLookupFailed occurs when a lookup with [DeviceLookuper] fails.
	errDeviceLookupFailed = errors.New("device lookup failed")

	// errInvalidArgument occurs whenever a given argument is invalid or missing.
	errInvalidArgument = errors.New("invalid argument")

	// errInvalidJSON occurs whenever data that is expected as JSON is invalid.
	errInvalidJSON = errors.New("invalid JSON")

	// errNoDevices occurs when no devices were configured or enabled for monitoring.
	errNoDevices = errors.New("no devices configured")
)

// ConfigYAML represents the YAML configuration structure.
type ConfigYAML struct {
	DisableTimestamps bool         `yaml:"disable_timestamps"`
	Devices           []DeviceYAML `yaml:"devices"`
}

// DeviceYAML represents a single device configuration in YAML.
type DeviceYAML struct {
	Device         string               `yaml:"device"`
	Address        string               `yaml:"address"`
	Description    string               `yaml:"description"`
	Type           int                  `yaml:"type"`
	Enabled        bool                 `yaml:"enabled"`
	MonitorConfig  *DeviceMonitorConfig `yaml:"config,omitempty"`
	ScriptNotifier *ScriptNotifierYAML  `yaml:"script_notifier,omitempty"`
}

// ScriptNotifierYAML represents a [ScriptNotifier] configuration in YAML.
type ScriptNotifierYAML struct {
	Script string                `yaml:"script"`
	Config *ScriptNotifierConfig `yaml:"config,omitempty"`
}

// Program is the primary implementation and manages multiple device monitors.
type Program struct {
	monitors map[string]*DeviceMonitor
	done     chan struct{}
	logger   *log.Logger
}

// NewProgram creates a new Program from a YAML configuration string.
func NewProgram(yamlConfig []byte, f afero.Fs, d DeviceLookuper, r CommandRunner, o io.Writer) (*Program, error) {
	var config ConfigYAML
	decoder := yaml.NewDecoder(bytes.NewReader(yamlConfig))
	decoder.KnownFields(true)

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failure parsing YAML: %w", err)
	}

	if len(config.Devices) == 0 {
		return nil, errNoDevices
	}

	var fsys afero.Fs
	if f != nil {
		fsys = f
	} else {
		fsys = afero.NewOsFs()
	}

	var logger *log.Logger
	if config.DisableTimestamps {
		logger = log.New(o, "", log.Lmsgprefix)
	} else {
		logger = log.New(o, "", log.LstdFlags|log.Lmsgprefix)
	}

	p := &Program{
		monitors: make(map[string]*DeviceMonitor),
		done:     make(chan struct{}),
		logger:   logger,
	}

	var finder DeviceLookuper
	if d != nil {
		finder = d
	} else {
		if df, err := NewDeviceFinder(fsys, logger); err != nil {
			logger.Printf("Warning: Address lookup table not available: %v "+
				"(will not be able to monitor devices only defined by SAS address)", err)
		} else {
			finder = df
		}
	}

	seenOutputDirs := make(map[string]bool)
	for i, deviceCfg := range config.Devices {
		if !deviceCfg.Enabled {
			continue
		}

		if deviceCfg.Device == "" && deviceCfg.Address == "" {
			return nil, fmt.Errorf("[config:%d] %w: missing device and address "+
				"(needs to have at least one to be monitorable)", i, errInvalidArgument)
		}

		if deviceCfg.MonitorConfig != nil && deviceCfg.MonitorConfig.OutputDir != nil {
			if seenOutputDirs[*deviceCfg.MonitorConfig.OutputDir] {
				return nil, fmt.Errorf("[config:%d] %w: cannot use same output directory [%s] "+
					"for multiple devices", i, errInvalidArgument, *deviceCfg.MonitorConfig.OutputDir)
			}
			seenOutputDirs[*deviceCfg.MonitorConfig.OutputDir] = true
		}

		if err := lookupDevice(&deviceCfg, finder, logger); err != nil {
			return nil, fmt.Errorf("[config:%d] %w", i, err)
		}

		if _, exists := p.monitors[deviceCfg.Device]; exists {
			return nil, fmt.Errorf("[config:%d] %w: cannot monitor [%s:%s] multiple times",
				i, errInvalidArgument, deviceCfg.Device, deviceCfg.Address)
		}

		monitor, err := setupDeviceMonitor(config, deviceCfg, fsys, r, o)
		if err != nil {
			return nil, fmt.Errorf("[config:%d:%s:%s] %w", i, deviceCfg.Device, deviceCfg.Address, err)
		}

		p.monitors[deviceCfg.Device] = monitor
	}

	return p, nil
}

// lookupDevice attempts to lookup a single [DeviceYAML] using a [DeviceLookuper].
// It receives a pointer to a [DeviceYAML] configuration and completes the fields in-place.
func lookupDevice(deviceCfg *DeviceYAML, finder DeviceLookuper, logger *log.Logger) error {
	//nolint:nestif
	if deviceCfg.Address != "" {
		if finder != nil {
			if dev, ok := finder.FindDevice(deviceCfg.Address); ok {
				logger.Printf("SAS address [%s] was resolved to device [%s]",
					deviceCfg.Address, dev)
				deviceCfg.Device = dev
			} else if deviceCfg.Device == "" {
				return fmt.Errorf("%w: SAS address [%s] is not resolvable (not found)",
					errDeviceLookupFailed, deviceCfg.Address)
			} else {
				logger.Printf("Warning: SAS address [%s] is not resolvable (not found), "+
					"using provided device path instead", deviceCfg.Address)
				deviceCfg.Address = ""
			}
		} else {
			if deviceCfg.Device == "" {
				return fmt.Errorf("%w: SAS address [%s] is not resolvable (no lookup table)",
					errDeviceLookupFailed, deviceCfg.Address)
			}
			logger.Printf("Warning: SAS address [%s] is not resolvable (no lookup table), "+
				"using provided device path instead", deviceCfg.Address)
			deviceCfg.Address = ""
		}
	} else if deviceCfg.Device != "" && finder != nil {
		if addr, ok := finder.FindAddress(deviceCfg.Device); ok {
			logger.Printf("Device [%s] was resolved to SAS address [%s] - consider [address: %q] "+
				"instead of [device: %q] for your configuration (more stable across reboots)",
				deviceCfg.Device, addr, addr, deviceCfg.Device)
			deviceCfg.Address = addr
		}
	}

	return nil
}

// setupDeviceMonitor creates and sets up the [DeviceMonitor] for a [DeviceYAML].
func setupDeviceMonitor(cfg ConfigYAML, deviceCfg DeviceYAML, fsys afero.Fs, r CommandRunner, o io.Writer) (*DeviceMonitor, error) {
	var logger *log.Logger
	if cfg.DisableTimestamps {
		logger = log.New(o, deviceCfg.Device+":"+deviceCfg.Address+": ", log.Lmsgprefix)
	} else {
		logger = log.New(o, deviceCfg.Device+":"+deviceCfg.Address+": ", log.LstdFlags|log.Lmsgprefix)
	}

	var runner CommandRunner
	if r != nil {
		runner = r
	} else {
		runner = &RetryCommandRunner{logger: logger}
	}

	var notifier Notifier
	if deviceCfg.ScriptNotifier != nil {
		var err error
		notifier, err = NewScriptNotifier(
			deviceCfg.ScriptNotifier.Script, deviceCfg.ScriptNotifier.Config,
			fsys, runner, logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failure creating notification agent: %w", err)
		}
	}

	monitor, err := NewDeviceMonitor(
		Device{
			Type:        deviceCfg.Type,
			Path:        deviceCfg.Device,
			Address:     deviceCfg.Address,
			Description: deviceCfg.Description,
		},
		deviceCfg.MonitorConfig,
		fsys,
		runner,
		logger,
		notifier,
	)
	if err != nil {
		return nil, fmt.Errorf("failure creating monitoring agent: %w", err)
	}

	return monitor, nil
}

// Start begins monitoring all enabled devices.
func (p *Program) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, monitor := range p.monitors {
		wg.Go(func() {
			defer recoverGoPanic("program", p.logger)
			monitor.Start(ctx)
			<-monitor.Done()
		})
	}

	go func() {
		defer recoverGoPanic("program-waiter", p.logger)
		wg.Wait()
		close(p.done)
	}()
}

// Stop signals all monitors to stop.
func (p *Program) Stop() {
	for _, monitor := range p.monitors {
		monitor.Stop()
	}
}

// Done returns a channel that's closed when all monitors have stopped.
func (p *Program) Done() <-chan struct{} {
	return p.done
}

// getMonitors returns a copy of the monitors map (for testing).
func (p *Program) getMonitors() map[string]*DeviceMonitor {
	result := make(map[string]*DeviceMonitor, len(p.monitors))

	maps.Copy(result, p.monitors)

	return result
}
