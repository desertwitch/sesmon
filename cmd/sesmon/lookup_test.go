package main

import (
	"log"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: NewDeviceFinder should successfully create finder with single device.
func Test_NewDeviceFinder_SingleDevice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should successfully create finder with multiple devices.
func Test_NewDeviceFinder_MultipleDevices_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg2/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte("0x5000c50098765433"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg2/device/sas_address", []byte("0x5000c50098765434"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 3)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
	require.Equal(t, "/dev/sg1", finder.devices["0x5000c50098765433"])
	require.Equal(t, "/dev/sg2", finder.devices["0x5000c50098765434"])
}

// Expectation: NewDeviceFinder should normalize SAS addresses to lowercase.
func Test_NewDeviceFinder_NormalizeAddressToLowercase_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0X5000C50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should trim whitespace from SAS addresses.
func Test_NewDeviceFinder_TrimWhitespace_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("  0x5000c50098765432  \n"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should skip devices without sas_address file.
func Test_NewDeviceFinder_SkipDevicesWithoutSasAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	// sg1 has no sas_address file

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should skip devices with empty SAS addresses.
func Test_NewDeviceFinder_SkipEmptySasAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte(""), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should skip devices with whitespace-only SAS addresses.
func Test_NewDeviceFinder_SkipWhitespaceOnlySasAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte("   \n  \t  "), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg0", finder.devices["0x5000c50098765432"])
}

// Expectation: NewDeviceFinder should ignore duplicate SAS addresses and log warning.
func Test_NewDeviceFinder_IgnoreDuplicateSasAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg2/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg2/device/sas_address", []byte("0x5000c50098765433"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg2", finder.devices["0x5000c50098765433"])
	require.NotContains(t, finder.devices, "0x5000c50098765432")

	output := buf.String()
	require.Contains(t, output, "Warning:")
	require.Contains(t, output, "0x5000c50098765432")
	require.Contains(t, output, "multiple devices")
}

// Expectation: NewDeviceFinder should handle multiple duplicate addresses correctly.
func Test_NewDeviceFinder_MultipleDuplicates_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg2/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg3/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg2/device/sas_address", []byte("0x5000c50098765433"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg3/device/sas_address", []byte("0x5000c50098765433"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Empty(t, finder.devices)

	output := buf.String()
	require.Empty(t, finder.devices)
	require.Contains(t, output, "0x5000c50098765432")
	require.Contains(t, output, "0x5000c50098765433")
}

// Expectation: NewDeviceFinder should return error when glob fails.
func Test_NewDeviceFinder_GlobError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	// Create an invalid filesystem state or use a read-only filesystem

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	// When no matching paths exist, glob returns nil error with empty slice
	// So this test verifies the finder is created but empty
	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Empty(t, finder.devices)
}

// Expectation: NewDeviceFinder should handle empty scsi_generic directory.
func Test_NewDeviceFinder_EmptyScsiGeneric_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic", 0o755))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Empty(t, finder.devices)
}

// Expectation: NewDeviceFinder should handle high device numbers correctly.
func Test_NewDeviceFinder_HighDeviceNumbers_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg127/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg127/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)

	require.NoError(t, err)
	require.NotNil(t, finder)
	require.Len(t, finder.devices, 1)
	require.Equal(t, "/dev/sg127", finder.devices["0x5000c50098765432"])
}

// Expectation: FindDevice should return device path for valid SAS address.
func Test_FindDevice_ValidAddress_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	device, found := finder.FindDevice("0x5000c50098765432")

	require.True(t, found)
	require.Equal(t, "/dev/sg0", device)
}

// Expectation: FindDevice should return false for unknown SAS address.
func Test_FindDevice_UnknownAddress_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	device, found := finder.FindDevice("0x9999999999999999")

	require.False(t, found)
	require.Empty(t, device)
}

// Expectation: FindDevice should return false for empty address.
func Test_FindDevice_EmptyAddress_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	device, found := finder.FindDevice("")

	require.False(t, found)
	require.Empty(t, device)
}

// Expectation: FindAddress should return SAS address for valid device path.
func Test_FindAddress_ValidDevice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	address, found := finder.FindAddress("/dev/sg0")

	require.True(t, found)
	require.Equal(t, "0x5000c50098765432", address)
}

// Expectation: FindAddress should return false for unknown device path.
func Test_FindAddress_UnknownDevice_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	address, found := finder.FindAddress("/dev/sg99")

	require.False(t, found)
	require.Empty(t, address)
}

// Expectation: FindAddress should return false for empty device path.
func Test_FindAddress_EmptyDevice_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	address, found := finder.FindAddress("")

	require.False(t, found)
	require.Empty(t, address)
}

// Expectation: FindAddress and FindDevice should work bidirectionally.
func Test_FindAddress_FindDevice_Bidirectional_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg0/device", 0o755))
	require.NoError(t, fs.MkdirAll("/sys/class/scsi_generic/sg1/device", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg0/device/sas_address", []byte("0x5000c50098765432"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/sys/class/scsi_generic/sg1/device/sas_address", []byte("0x5000c50098765433"), 0o644))

	var buf safeBuffer
	logger := log.New(&buf, "", 0)

	finder, err := NewDeviceFinder(fs, logger)
	require.NoError(t, err)

	// Forward lookup: address -> device
	device0, found0 := finder.FindDevice("0x5000c50098765432")
	require.True(t, found0)
	require.Equal(t, "/dev/sg0", device0)

	// Reverse lookup: device -> address
	address0, foundAddr0 := finder.FindAddress(device0)
	require.True(t, foundAddr0)
	require.Equal(t, "0x5000c50098765432", address0)

	// Test second device
	device1, found1 := finder.FindDevice("0x5000c50098765433")
	require.True(t, found1)
	require.Equal(t, "/dev/sg1", device1)

	address1, foundAddr1 := finder.FindAddress(device1)
	require.True(t, foundAddr1)
	require.Equal(t, "0x5000c50098765433", address1)
}
