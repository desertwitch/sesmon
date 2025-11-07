<div align="center">
    <img alt="Logo" src="assets/sesmon.png" width="260">
    <h1>sesmon</h1>
    <p>SCSI Enclosure Services (SES)<br><b>Monitoring and Alerting Daemon</b></p>
</div>

<div align="center">
    <a href="https://github.com/desertwitch/sesmon/releases"><img alt="Release" src="https://img.shields.io/github/release/desertwitch/sesmon.svg"></a>
    <a href="https://go.dev/"><img alt="Go Version" src="https://img.shields.io/badge/Go-%3E%3D%201.25.1-%23007d9c"></a>
    <a href="https://pkg.go.dev/github.com/desertwitch/sesmon"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/desertwitch/sesmon.svg"></a>
    <a href="https://goreportcard.com/report/github.com/desertwitch/sesmon"><img alt="Go Report" src="https://goreportcard.com/badge/github.com/desertwitch/sesmon"></a>
    <a href="./LICENSE"><img alt="License" src="https://img.shields.io/github/license/desertwitch/sesmon"></a>
    <br>
    <a href="https://app.codecov.io/gh/desertwitch/sesmon"><img alt="Codecov" src="https://codecov.io/github/desertwitch/sesmon/graph/badge.svg?token=5CR32ES41N"></a>
    <a href="https://github.com/desertwitch/sesmon/actions/workflows/golangci-lint.yml"><img alt="Lint" src="https://github.com/desertwitch/sesmon/actions/workflows/golangci-lint.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/sesmon/actions/workflows/golang-tests.yml"><img alt="Tests" src="https://github.com/desertwitch/sesmon/actions/workflows/golang-tests.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/sesmon/actions/workflows/golang-build.yml"><img alt="Build" src="https://github.com/desertwitch/sesmon/actions/workflows/golang-build.yml/badge.svg"></a>
</div><br>

## Overview

sesmon is a monitoring and alerting daemon for SES-capable SCSI enclosures. It
periodically polls `sg_ses` for a JSON-format `--all` dump, comparing changes
between previous and most recent device data. An alert is raised if changes of
status-relevant fields were detected. For each SES element of the SCSI
enclosure, specifically only the status descriptors and their following fields
(most are common among all element status descriptors) are monitored:

- `status`
- `prdfail`
- `disabled`
- `swap`
- `temperature` (if present)
- `voltage` (if present)
- `current` (if present)

The alerts themselves are emitted to standard error (`stderr`), and it is also
possible to configure an external notification agent for each device. Such an
agent could be a shell script or any other executable, which is then called
on alert, with the relevant information passed via positional arguments.

## Dependencies

- `sg_ses` (as usually a part of `sg3_utils` packages)

## Installation

To build from source, a `Makefile` is included with the project's source code.
Running `make all` will compile the application and pull in any necessary
dependencies. `make check` runs the test suite and static analysis tools.

For convenience, precompiled static binaries for common architectures are
released through GitHub. These can be installed into `/usr/bin/` or respective
system locations; ensure they are executable by running `chmod +x` before use.

> All builds from source are designed to generate [reproducible builds](https://reproducible-builds.org/),
> meaning that they should compile as byte-identical to the respective released binaries and also have the
> exact same checksums upon integrity verification.

## Building from source:

```bash
git clone https://github.com/desertwitch/sesmon.git
cd sesmon
make all
```

## Running a built executable:

```bash
./sesmon --help
```

## Configuration Example

```yaml
# sesmon configuration file
# "check" and "test" commands can help verify configuration files

# Disable timestamps in log output
disable_timestamps: false

# List of devices to monitor
#
# Devices can be defined either by device path or SAS address (or both)
# Defining by SAS address is more stable across reboots (and recommended)
# SAS addresses can be obtained by e.g. using the "lsscsi" utility ("-t")
#
# If defined by SAS address, the devices are resolved to their "/dev" paths
# at the begin of the program (can be tested with "sesmon test <config.yaml>")
# SAS address resolves using: "/sys/class/scsi_generic/sg*/device/sas_address"
devices:
  # Device 1 - resolve by SAS address (recommended)
  - address: "0x500a098012345678"

    # Type of device (0 = Device, 1 = JSON file)
    # JSON file "devices" can be useful for testing
    type: 0
    
    # Human-readable description of this device
    description: "JBOD"
    
    # Enable monitoring for this device
    enabled: true
    
    # Optional: Device monitoring configuration
    # Omitted settings use defaults as shown below
    config:
      # How often to poll the target device for data
      poll_interval: "1m30s"
      
      # How often to attempt a device poll (must be > 0)
      poll_attempts: 3
      
      # How long a device poll attempt can take (multiplies with attempts)
      poll_attempt_timeout: "15s"
      
      # How long to wait between device poll attempts (in case of failure)
      poll_attempt_interval: "15s"
      
      # How many consecutive poll failures trigger back-off period
      # Note: First failure = after 3 attempts (set value of poll_attempts)
      #       So backoff after 3 failures = after total 9 failed poll attempts
      poll_backoff_after: 3
      
      # How long to pause polling the device when in back-off period
      poll_backoff_time: "3m0s"
      
      # Dispatch notification through agent when entering back-off period
      # Applies only if a notification agent is configured for the device
      poll_backoff_notify: true
      
      # Permanently stop monitoring the device when entering back-off period
      # If false, monitoring resumes normally after poll_backoff_time elapses
      poll_backoff_stopmonitor: false
      
      # Folder to write JSON files of device state and alerts to
      # Must be unique per device and creates the following files:
      #   - current.json (raw snapshot of current device state)
      #   - current_parsed.json (parsed snapshot of current device state)
      #   - change-YYYYMMDD-HHMMSS.json (single timestamped change report)
      #   - change-YYYYMMDD-HHMMSS.json (single timestamped change report)
      #   - ...
      # Default: (none)
      output_dir: "/var/lib/sesmon/JBOD"
      
      # Output also verbose operational information as part of log output
      verbose: false
    
    # Optional: Notification agent (e.g., external script for alerts)
    # If omitted, alerts are emitted only as part of regular log output
    script_notifier:
      # Path to executable notification script
      # Script receives these arguments:
      #   $1: Device path (e.g., /dev/sg25)
      #   $2: SAS address (e.g., 0x500a098012345678)
      #   $3: Device description (e.g., "JBOD")
      #   $4: Notification message in textual format
      #   $5: Change report in JSON format (where applicable)
      script: "/usr/local/bin/my-notify-script.sh"
      
      # Optional: Notification agent configuration
      # Omitted settings use defaults as shown below
      config:
        # How often to attempt a notification (must be > 0)
        notify_attempts: 3
        
        # How long a notification attempt can take (multiplies with attempts)
        notify_attempt_timeout: "15s"
        
        # How long to wait between notification attempts (in case of failure)
        notify_attempt_interval: "15s"
  
  # Device 2 - resolve by device path (not recommended)
  - device: "/dev/sg25"
    type: 0
    description: "JBOD2"
    enabled: true
    # Uses all default settings and no notification agent

  # Device 3 - JSON file for simulations and/or testing
  - device: "/tmp/device.json"
    type: 1
    description: "JBOD3"
    enabled: true
    # Uses all default settings and no notification agent
```

## License

All code is licensed under the MIT License.
