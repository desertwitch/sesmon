package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Expectation: fne should return first non-empty string.
func Test_fne_FirstNonEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fne("first", "second")
	require.Equal(t, "first", result)
}

// Expectation: fne should return second string when first is empty.
func Test_fne_SecondWhenFirstEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fne("", "second")
	require.Equal(t, "second", result)
}

// Expectation: fne should return empty when both are empty.
func Test_fne_BothEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fne("", "")
	require.Empty(t, result)
}

// Expectation: fne should handle whitespace as non-empty.
func Test_fne_WhitespaceIsNonEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fne(" ", "second")
	require.Equal(t, " ", result)
}

// Expectation: fnz should return first non-zero int.
func Test_fnz_FirstNonZero_Success(t *testing.T) {
	t.Parallel()

	result := fnz(42, 100)
	require.Equal(t, 42, result)
}

// Expectation: fnz should return second int when first is zero.
func Test_fnz_SecondWhenFirstZero_Success(t *testing.T) {
	t.Parallel()

	result := fnz(0, 100)
	require.Equal(t, 100, result)
}

// Expectation: fnz should return zero when both are zero.
func Test_fnz_BothZero_Success(t *testing.T) {
	t.Parallel()

	result := fnz(0, 0)
	require.Equal(t, 0, result)
}

// Expectation: fnz should handle negative numbers correctly.
func Test_fnz_NegativeNumbers_Success(t *testing.T) {
	t.Parallel()

	result := fnz(-5, 10)
	require.Equal(t, -5, result)

	result = fnz(0, -10)
	require.Equal(t, -10, result)
}

// Expectation: ptr should return a pointer to an integer value.
func Test_ptr_Integer_Success(t *testing.T) {
	t.Parallel()

	result := ptr(42)
	require.NotNil(t, result)
	require.Equal(t, 42, *result)
}

// Expectation: ptr should return a pointer to a zero integer.
func Test_ptr_ZeroInteger_Success(t *testing.T) {
	t.Parallel()

	result := ptr(0)
	require.NotNil(t, result)
	require.Equal(t, 0, *result)
}

// Expectation: ptr should return a pointer to a negative integer.
func Test_ptr_NegativeInteger_Success(t *testing.T) {
	t.Parallel()

	result := ptr(-42)
	require.NotNil(t, result)
	require.Equal(t, -42, *result)
}

// Expectation: ptr should return a pointer to a string value.
func Test_ptr_String_Success(t *testing.T) {
	t.Parallel()

	result := ptr("hello")
	require.NotNil(t, result)
	require.Equal(t, "hello", *result)
}

// Expectation: ptr should return a pointer to an empty string.
func Test_ptr_EmptyString_Success(t *testing.T) {
	t.Parallel()

	result := ptr("")
	require.NotNil(t, result)
	require.Empty(t, *result)
}

// Expectation: ptr should return a pointer to a boolean value.
func Test_ptr_Boolean_Success(t *testing.T) {
	t.Parallel()

	result := ptr(true)
	require.NotNil(t, result)
	require.True(t, *result)

	result = ptr(false)
	require.NotNil(t, result)
	require.False(t, *result)
}

// Expectation: ptr should return a pointer to a time.Duration value.
func Test_ptr_Duration_Success(t *testing.T) {
	t.Parallel()

	result := ptr(5 * time.Second)
	require.NotNil(t, result)
	require.Equal(t, 5*time.Second, *result)
}

// Expectation: ptr should work with struct types.
func Test_ptr_Struct_Success(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Field1 string
		Field2 int
	}

	val := testStruct{Field1: "test", Field2: 42}
	result := ptr(val)
	require.NotNil(t, result)
	require.Equal(t, "test", result.Field1)
	require.Equal(t, 42, result.Field2)
}

// Expectation: ptr should create independent pointers for each call.
func Test_ptr_IndependentPointers_Success(t *testing.T) {
	t.Parallel()

	ptr1 := ptr(42)
	ptr2 := ptr(42)

	require.NotNil(t, ptr1)
	require.NotNil(t, ptr2)
	require.Equal(t, *ptr1, *ptr2)
	require.NotSame(t, ptr1, ptr2)
}

// Expectation: ptr should allow modification of the pointed value.
func Test_ptr_ModifyValue_Success(t *testing.T) {
	t.Parallel()

	result := ptr(10)
	require.Equal(t, 10, *result)

	*result = 20
	require.Equal(t, 20, *result)
}

// Expectation: ptrIntEqual should return true when both pointers are nil.
func Test_ptrIntEqual_BothNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(nil, nil)
	require.True(t, result)
}

// Expectation: ptrIntEqual should return false when first pointer is nil.
func Test_ptrIntEqual_FirstNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(nil, ptr(42))
	require.False(t, result)
}

// Expectation: ptrIntEqual should return false when second pointer is nil.
func Test_ptrIntEqual_SecondNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(ptr(42), nil)
	require.False(t, result)
}

// Expectation: ptrIntEqual should return true when both pointers have same value.
func Test_ptrIntEqual_SameValue_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(ptr(42), ptr(42))
	require.True(t, result)
}

// Expectation: ptrIntEqual should return false when pointers have different values.
func Test_ptrIntEqual_DifferentValues_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(ptr(42), ptr(100))
	require.False(t, result)
}

// Expectation: ptrIntEqual should handle zero values correctly.
func Test_ptrIntEqual_ZeroValues_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(ptr(0), ptr(0))
	require.True(t, result)
}

// Expectation: ptrIntEqual should handle negative numbers correctly.
func Test_ptrIntEqual_NegativeNumbers_Success(t *testing.T) {
	t.Parallel()

	result := ptrIntEqual(ptr(-5), ptr(-5))
	require.True(t, result)

	result = ptrIntEqual(ptr(-5), ptr(-10))
	require.False(t, result)
}

// Expectation: ptrStrEqualFold should return true when both pointers are nil.
func Test_ptrStrEqualFold_BothNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(nil, nil)
	require.True(t, result)
}

// Expectation: ptrStrEqualFold should return false when first pointer is nil.
func Test_ptrStrEqualFold_FirstNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(nil, ptr("hello"))
	require.False(t, result)
}

// Expectation: ptrStrEqualFold should return false when second pointer is nil.
func Test_ptrStrEqualFold_SecondNil_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(ptr("hello"), nil)
	require.False(t, result)
}

// Expectation: ptrStrEqualFold should return true when both pointers have same value.
func Test_ptrStrEqualFold_SameValue_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(ptr("hello"), ptr("hello"))
	require.True(t, result)
}

// Expectation: ptrStrEqualFold should return true for case-insensitive match.
func Test_ptrStrEqualFold_CaseInsensitive_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(ptr("Hello"), ptr("HELLO"))
	require.True(t, result)

	result = ptrStrEqualFold(ptr("WoRlD"), ptr("world"))
	require.True(t, result)
}

// Expectation: ptrStrEqualFold should return false when strings are different.
func Test_ptrStrEqualFold_DifferentValues_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(ptr("hello"), ptr("world"))
	require.False(t, result)
}

// Expectation: ptrStrEqualFold should handle empty strings correctly.
func Test_ptrStrEqualFold_EmptyStrings_Success(t *testing.T) {
	t.Parallel()

	result := ptrStrEqualFold(ptr(""), ptr(""))
	require.True(t, result)

	result = ptrStrEqualFold(ptr("hello"), ptr(""))
	require.False(t, result)
}

// Expectation: fmtPtrInt should return empty string when pointer is nil.
func Test_fmtPtrInt_NilPointer_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrInt(nil, "")
	require.Empty(t, result)
}

// Expectation: fmtPtrInt should return custom empty string when pointer is nil.
func Test_fmtPtrInt_NilPointerCustomEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrInt(nil, "-")
	require.Equal(t, "-", result)

	result = fmtPtrInt(nil, "N/A")
	require.Equal(t, "N/A", result)
}

// Expectation: fmtPtrInt should format positive integer correctly.
func Test_fmtPtrInt_PositiveValue_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrInt(ptr(42), "-")
	require.Equal(t, "42", result)
}

// Expectation: fmtPtrInt should format zero correctly.
func Test_fmtPtrInt_ZeroValue_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrInt(ptr(0), "-")
	require.Equal(t, "0", result)
}

// Expectation: fmtPtrInt should format negative integer correctly.
func Test_fmtPtrInt_NegativeValue_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrInt(ptr(-42), "-")
	require.Equal(t, "-42", result)
}

// Expectation: fmtPtrStr should return empty string when pointer is nil.
func Test_fmtPtrStr_NilPointer_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(nil, "")
	require.Empty(t, result)
}

// Expectation: fmtPtrStr should return custom empty string when pointer is nil.
func Test_fmtPtrStr_NilPointerCustomEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(nil, "-")
	require.Equal(t, "-", result)

	result = fmtPtrStr(nil, "N/A")
	require.Equal(t, "N/A", result)
}

// Expectation: fmtPtrStr should return the string value when pointer is not nil.
func Test_fmtPtrStr_ValidString_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(ptr("hello"), "-")
	require.Equal(t, "hello", result)
}

// Expectation: fmtPtrStr should handle empty string correctly.
func Test_fmtPtrStr_EmptyString_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(ptr(""), "-")
	require.Empty(t, result)
}

// Expectation: fmtPtrStr should handle whitespace correctly.
func Test_fmtPtrStr_Whitespace_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(ptr("   "), "-")
	require.Equal(t, "   ", result)
}

// Expectation: fmtPtrStr should handle special characters correctly.
func Test_fmtPtrStr_SpecialCharacters_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrStr(ptr("hello\nworld"), "-")
	require.Equal(t, "hello\nworld", result)

	result = fmtPtrStr(ptr("test@"), "-")
	require.Equal(t, "test@", result)
}

// Expectation: fmtPtrQStr should return empty string when pointer is nil.
func Test_fmtPtrQStr_NilPointer_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(nil, "")
	require.Empty(t, result)
}

// Expectation: fmtPtrQStr should return custom empty string when pointer is nil.
func Test_fmtPtrQStr_NilPointerCustomEmpty_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(nil, "-")
	require.Equal(t, "-", result)
	result = fmtPtrQStr(nil, "N/A")
	require.Equal(t, "N/A", result)
}

// Expectation: fmtPtrQStr should return the quoted string value when pointer is not nil.
func Test_fmtPtrQStr_ValidString_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(ptr("hello"), "-")
	require.Equal(t, `"hello"`, result)
}

// Expectation: fmtPtrQStr should handle empty string correctly with quotes.
func Test_fmtPtrQStr_EmptyString_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(ptr(""), "-")
	require.Equal(t, `""`, result)
}

// Expectation: fmtPtrQStr should handle whitespace correctly with quotes.
func Test_fmtPtrQStr_Whitespace_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(ptr("   "), "-")
	require.Equal(t, `"   "`, result)
}

// Expectation: fmtPtrQStr should handle special characters correctly with proper escaping.
func Test_fmtPtrQStr_SpecialCharacters_Success(t *testing.T) {
	t.Parallel()

	result := fmtPtrQStr(ptr("hello\nworld"), "-")
	require.Equal(t, `"hello\nworld"`, result)
	result = fmtPtrQStr(ptr("test@"), "-")
	require.Equal(t, `"test@"`, result)
	result = fmtPtrQStr(ptr(`test"quote`), "-")
	require.Equal(t, `"test\"quote"`, result)
}

// Expectation: The function should meet the table's expectations.
func Test_mergeDeviceMonitorConfig_Defaults_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userCfg *DeviceMonitorConfig
	}{
		{
			name:    "nil user config returns defaults",
			userCfg: nil,
		},
		{
			name:    "empty user config returns defaults",
			userCfg: &DeviceMonitorConfig{},
		},
		{
			name: "empty string OutputDir uses default",
			userCfg: &DeviceMonitorConfig{
				OutputDir: ptr(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := mergeDeviceMonitorConfig(tt.userCfg)
			require.NoError(t, err)

			defaultCfg := DefaultDeviceMonitorConfig()

			require.NotNil(t, result)
			require.Equal(t, defaultCfg.PollInterval, result.PollInterval)
			require.Equal(t, defaultCfg.PollAttempts, result.PollAttempts)
			require.Equal(t, defaultCfg.PollAttemptTimeout, result.PollAttemptTimeout)
			require.Equal(t, defaultCfg.PollAttemptInterval, result.PollAttemptInterval)
			require.Equal(t, defaultCfg.PollBackoffAfter, result.PollBackoffAfter)
			require.Equal(t, defaultCfg.PollBackoffTime, result.PollBackoffTime)
			require.Equal(t, defaultCfg.PollBackoffNotify, result.PollBackoffNotify)
			require.Equal(t, defaultCfg.PollBackoffStopMonitor, result.PollBackoffStopMonitor)
			require.Equal(t, defaultCfg.OutputDir, result.OutputDir)
			require.Equal(t, defaultCfg.Verbose, result.Verbose)
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_mergeDeviceMonitorConfig_UserConfig_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		userCfg  *DeviceMonitorConfig
		expected *DeviceMonitorConfig
	}{
		{
			name: "all fields provided by user",
			userCfg: &DeviceMonitorConfig{
				PollInterval:           ptr(10 * time.Second),
				PollAttempts:           ptr(5),
				PollAttemptTimeout:     ptr(30 * time.Second),
				PollAttemptInterval:    ptr(2 * time.Second),
				PollBackoffAfter:       ptr(3),
				PollBackoffTime:        ptr(15 * time.Second),
				PollBackoffNotify:      ptr(false),
				PollBackoffStopMonitor: ptr(true),
				OutputDir:              ptr("/custom/path"),
				Verbose:                ptr(true),
			},
			expected: &DeviceMonitorConfig{
				PollInterval:           ptr(10 * time.Second),
				PollAttempts:           ptr(5),
				PollAttemptTimeout:     ptr(30 * time.Second),
				PollAttemptInterval:    ptr(2 * time.Second),
				PollBackoffAfter:       ptr(3),
				PollBackoffTime:        ptr(15 * time.Second),
				PollBackoffNotify:      ptr(false),
				PollBackoffStopMonitor: ptr(true),
				OutputDir:              ptr("/custom/path"),
				Verbose:                ptr(true),
			},
		},
		{
			name: "only PollInterval provided",
			userCfg: &DeviceMonitorConfig{
				PollInterval: ptr(8 * time.Second),
			},
			expected: func() *DeviceMonitorConfig {
				cfg := DefaultDeviceMonitorConfig()
				cfg.PollInterval = ptr(8 * time.Second)

				return cfg
			}(),
		},
		{
			name: "only OutputDir provided",
			userCfg: &DeviceMonitorConfig{
				OutputDir: ptr("/another/path"),
			},
			expected: func() *DeviceMonitorConfig {
				cfg := DefaultDeviceMonitorConfig()
				cfg.OutputDir = ptr("/another/path")

				return cfg
			}(),
		},
		{
			name: "mix of user and default values",
			userCfg: &DeviceMonitorConfig{
				PollInterval:    ptr(7 * time.Second),
				PollAttempts:    ptr(10),
				PollBackoffTime: ptr(20 * time.Second),
			},
			expected: func() *DeviceMonitorConfig {
				cfg := DefaultDeviceMonitorConfig()
				cfg.PollInterval = ptr(7 * time.Second)
				cfg.PollAttempts = ptr(10)
				cfg.PollBackoffTime = ptr(20 * time.Second)

				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := mergeDeviceMonitorConfig(tt.userCfg)
			require.NoError(t, err)

			require.NotNil(t, result)
			require.Equal(t, tt.expected.PollInterval, result.PollInterval)
			require.Equal(t, tt.expected.PollAttempts, result.PollAttempts)
			require.Equal(t, tt.expected.PollAttemptTimeout, result.PollAttemptTimeout)
			require.Equal(t, tt.expected.PollAttemptInterval, result.PollAttemptInterval)
			require.Equal(t, tt.expected.PollBackoffAfter, result.PollBackoffAfter)
			require.Equal(t, tt.expected.PollBackoffTime, result.PollBackoffTime)
			require.Equal(t, tt.expected.PollBackoffNotify, result.PollBackoffNotify)
			require.Equal(t, tt.expected.PollBackoffStopMonitor, result.PollBackoffStopMonitor)
			require.Equal(t, tt.expected.OutputDir, result.OutputDir)
			require.Equal(t, tt.expected.Verbose, result.Verbose)
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_mergeScriptNotifierConfig_Defaults_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userCfg *ScriptNotifierConfig
	}{
		{
			name:    "nil user config returns defaults",
			userCfg: nil,
		},
		{
			name:    "empty user config returns defaults",
			userCfg: &ScriptNotifierConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := mergeScriptNotifierConfig(tt.userCfg)
			require.NoError(t, err)

			defaultCfg := DefaultScriptNotifierConfig()

			require.NotNil(t, result)
			require.Equal(t, defaultCfg.NotifyAttempts, result.NotifyAttempts)
			require.Equal(t, defaultCfg.NotifyAttemptTimeout, result.NotifyAttemptTimeout)
			require.Equal(t, defaultCfg.NotifyAttemptInterval, result.NotifyAttemptInterval)
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_mergeScriptNotifierConfig_UserConfig_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		userCfg  *ScriptNotifierConfig
		expected *ScriptNotifierConfig
	}{
		{
			name: "all fields provided by user",
			userCfg: &ScriptNotifierConfig{
				NotifyAttempts:        ptr(5),
				NotifyAttemptTimeout:  ptr(60 * time.Second),
				NotifyAttemptInterval: ptr(5 * time.Second),
			},
			expected: &ScriptNotifierConfig{
				NotifyAttempts:        ptr(5),
				NotifyAttemptTimeout:  ptr(60 * time.Second),
				NotifyAttemptInterval: ptr(5 * time.Second),
			},
		},
		{
			name: "only NotifyAttempts provided",
			userCfg: &ScriptNotifierConfig{
				NotifyAttempts: ptr(8),
			},
			expected: func() *ScriptNotifierConfig {
				cfg := DefaultScriptNotifierConfig()
				cfg.NotifyAttempts = ptr(8)

				return cfg
			}(),
		},
		{
			name: "only NotifyAttemptTimeout provided",
			userCfg: &ScriptNotifierConfig{
				NotifyAttemptTimeout: ptr(45 * time.Second),
			},
			expected: func() *ScriptNotifierConfig {
				cfg := DefaultScriptNotifierConfig()
				cfg.NotifyAttemptTimeout = ptr(45 * time.Second)

				return cfg
			}(),
		},
		{
			name: "mix of user and default values",
			userCfg: &ScriptNotifierConfig{
				NotifyAttempts:        ptr(3),
				NotifyAttemptInterval: ptr(10 * time.Second),
			},
			expected: func() *ScriptNotifierConfig {
				cfg := DefaultScriptNotifierConfig()
				cfg.NotifyAttempts = ptr(3)
				cfg.NotifyAttemptInterval = ptr(10 * time.Second)

				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := mergeScriptNotifierConfig(tt.userCfg)
			require.NoError(t, err)

			require.NotNil(t, result)
			require.Equal(t, tt.expected.NotifyAttempts, result.NotifyAttempts)
			require.Equal(t, tt.expected.NotifyAttemptTimeout, result.NotifyAttemptTimeout)
			require.Equal(t, tt.expected.NotifyAttemptInterval, result.NotifyAttemptInterval)
		})
	}
}

// Expectation: withRetries should succeed on first attempt and return attempt count 1.
func Test_withRetries_SuccessFirstAttempt_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	fn := func() error {
		callCount++

		return nil
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 3, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 1, attempt)
	require.Equal(t, 1, callCount)
}

// Expectation: withRetries should succeed on second attempt and return attempt count 2.
func Test_withRetries_SuccessSecondAttempt_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			return errors.New("temporary error")
		}

		return nil
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 3, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 2, attempt)
	require.Equal(t, 2, callCount)
}

// Expectation: withRetries should succeed on last attempt and return correct attempt count.
func Test_withRetries_SuccessLastAttempt_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 4 {
			return errors.New("temporary error")
		}

		return nil
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 4, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 4, attempt)
	require.Equal(t, 4, callCount)
}

// Expectation: withRetries should fail after exhausting all retries and return total attempt count.
func Test_withRetries_FailAllRetries_Error(t *testing.T) {
	t.Parallel()

	callCount := 0
	expectedErr := errors.New("persistent error")
	fn := func() error {
		callCount++

		return expectedErr
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 4, 10*time.Millisecond)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.Equal(t, 4, attempt)
	require.Equal(t, 4, callCount)
}

// Expectation: withRetries should call onRetryErr with 1-based attempt numbers for each failure.
func Test_withRetries_OnRetryErrCallback_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	retryErrCount := 0
	var retryAttempts []int
	var retryErrors []error

	expectedErr := errors.New("test error")
	fn := func() error {
		callCount++

		return expectedErr
	}

	onRetryErr := func(attempt int, err error) {
		retryErrCount++
		retryAttempts = append(retryAttempts, attempt)
		retryErrors = append(retryErrors, err)
	}

	attempt, err := withRetries(t.Context(), fn, onRetryErr, 3, 10*time.Millisecond)

	require.Error(t, err)
	require.Equal(t, 3, attempt)
	require.Equal(t, 3, callCount)
	require.Equal(t, 3, retryErrCount)
	require.Equal(t, []int{1, 2, 3}, retryAttempts)
	require.Len(t, retryErrors, 3)
	for _, e := range retryErrors {
		require.Equal(t, expectedErr, e)
	}
}

// Expectation: withRetries should return 0 attempts and context error if context is already cancelled.
func Test_withRetries_ContextCancelledBeforeStart_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	callCount := 0
	fn := func() error {
		callCount++

		return nil
	}

	attempt, err := withRetries(ctx, fn, func(int, error) {}, 3, 10*time.Millisecond)

	require.Error(t, err)
	require.Contains(t, err.Error(), "context error")
	require.Equal(t, 0, attempt)
	require.Equal(t, 0, callCount)
}

// Expectation: withRetries should return attempt count and context error if context is cancelled during retry interval.
func Test_withRetries_ContextCancelledDuringRetry_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			cancel()
		}

		return errors.New("test error")
	}

	attempt, err := withRetries(ctx, fn, func(int, error) {}, 3, 100*time.Millisecond)

	require.Error(t, err)
	require.Contains(t, err.Error(), "context error")
	require.Equal(t, 1, attempt)
	require.Equal(t, 1, callCount)
}

// Expectation: withRetries should handle zero retries correctly and not sleep after attempt.
func Test_withRetries_ZeroRetries_Error(t *testing.T) {
	t.Parallel()

	callCount := 0
	expectedErr := errors.New("error")
	fn := func() error {
		callCount++

		return expectedErr
	}

	interval := 1 * time.Second

	start := time.Now()
	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 1, interval)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.Equal(t, 1, attempt)
	require.Equal(t, 1, callCount)

	require.Less(t, elapsed, interval/2)
}

// Expectation: withRetries should respect retry interval timing.
func Test_withRetries_RespectsRetryInterval_Success(t *testing.T) {
	t.Parallel()

	start := time.Now()
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}

		return nil
	}

	interval := 50 * time.Millisecond
	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 3, interval)

	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, 3, attempt)
	require.Equal(t, 3, callCount)
	require.GreaterOrEqual(t, elapsed, 2*interval)
}

// Expectation: withRetries should handle nil fn without panicking.
func Test_withRetries_NilFn_Success(t *testing.T) {
	t.Parallel()

	attempt, err := withRetries(t.Context(), nil, nil, 3, 10*time.Millisecond)
	require.Zero(t, attempt)
	require.ErrorIs(t, err, errInvalidArgument)
}

// Expectation: withRetries should handle nil onRetryErr callback without panicking.
func Test_withRetries_NilOnRetryErr_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			return errors.New("error")
		}

		return nil
	}

	attempt, err := withRetries(t.Context(), fn, nil, 3, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 2, attempt)
	require.Equal(t, 2, callCount)
}

// Expectation: withRetries should return attempt count 1 for immediate success with multiple retries configured.
func Test_withRetries_ImmediateSuccessMultipleRetries_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	fn := func() error {
		callCount++

		return nil
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 10, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 1, attempt)
	require.Equal(t, 1, callCount)
}

// Expectation: withRetries should handle context timeout correctly.
func Test_withRetries_ContextTimeout_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()

	callCount := 0
	fn := func() error {
		callCount++
		time.Sleep(30 * time.Millisecond)

		return errors.New("error")
	}

	attempt, err := withRetries(ctx, fn, func(int, error) {}, 5, 100*time.Millisecond)

	require.Error(t, err)
	require.Contains(t, err.Error(), "context error")
	require.GreaterOrEqual(t, attempt, 1)
	require.LessOrEqual(t, attempt, 3)
}

// Expectation: withRetries should return correct attempt count when context is cancelled after multiple attempts.
func Test_withRetries_ContextCancelledAfterMultipleAttempts_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 3 {
			cancel()
		}

		return errors.New("test error")
	}

	attempt, err := withRetries(ctx, fn, func(int, error) {}, 5, 50*time.Millisecond)

	require.Error(t, err)
	require.Contains(t, err.Error(), "context error")
	require.Equal(t, 3, attempt)
	require.Equal(t, 3, callCount)
}

// Expectation: withRetries should handle single retry correctly.
func Test_withRetries_SingleRetry_Error(t *testing.T) {
	t.Parallel()

	callCount := 0
	expectedErr := errors.New("error")
	fn := func() error {
		callCount++

		return expectedErr
	}

	attempt, err := withRetries(t.Context(), fn, func(int, error) {}, 2, 10*time.Millisecond)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.Equal(t, 2, attempt)
	require.Equal(t, 2, callCount)
}

// Expectation: withRetries should call onRetryErr exactly once when succeeding on second attempt.
func Test_withRetries_OnRetryErrCalledOnceOnSecondSuccess_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	retryErrCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			return errors.New("first attempt error")
		}

		return nil
	}

	onRetryErr := func(attempt int, err error) {
		retryErrCount++
		require.Equal(t, 1, attempt)
	}

	attempt, err := withRetries(t.Context(), fn, onRetryErr, 3, 10*time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, 2, attempt)
	require.Equal(t, 2, callCount)
	require.Equal(t, 1, retryErrCount)
}

// Expectation: recoverGoPanic should log panic message with description and stack trace when panic occurs.
func Test_recoverGoPanic_WithLogger_PanicRecovered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	desc := "test operation"

	func() {
		defer recoverGoPanic(desc, logger)
		panic("intentional panic")
	}()

	output := buf.String()
	require.Contains(t, output, desc)
	require.Contains(t, output, "panic recovered")
	require.Contains(t, output, "intentional panic")
	require.Contains(t, output, "Test_recoverGoPanic")
}

// Expectation: recoverGoPanic should not log anything when no panic occurs.
func Test_recoverGoPanic_NoPanic_NoOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	desc := "test operation"

	func() {
		defer recoverGoPanic(desc, logger)
		// Normal execution, no panic
	}()

	output := buf.String()
	require.Empty(t, output)
}

// Expectation: recoverGoPanic should write to stderr when logger is nil.
//
//nolint:paralleltest
func Test_recoverGoPanic_NilLogger_WritesToStderr(t *testing.T) {
	// Note: Cannot use t.Parallel() due to stderr manipulation

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	desc := "test operation"

	func() {
		defer recoverGoPanic(desc, nil)
		panic("panic with nil logger")
	}()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	require.NoError(t, err)
	output := buf.String()

	require.Contains(t, output, desc)
	require.Contains(t, output, "panic recovered")
	require.Contains(t, output, "panic with nil logger")
}

// Expectation: recoverGoPanic should handle string panic values correctly.
func Test_recoverGoPanic_StringPanic_Recovered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	func() {
		defer recoverGoPanic("test", logger)
		panic("string error")
	}()

	output := buf.String()
	require.Contains(t, output, "panic recovered")
	require.Contains(t, output, "string error")
}

// Expectation: recoverGoPanic should handle integer panic values correctly.
func Test_recoverGoPanic_IntPanic_Recovered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	func() {
		defer recoverGoPanic("test", logger)
		panic(42)
	}()

	output := buf.String()
	require.Contains(t, output, "panic recovered")
	require.Contains(t, output, "42")
}

// Expectation: recoverGoPanic should handle struct panic values correctly.
func Test_recoverGoPanic_StructPanic_Recovered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	func() {
		defer recoverGoPanic("test", logger)
		panic(struct{ msg string }{"error"})
	}()

	output := buf.String()
	require.Contains(t, output, "panic recovered")
}

// Expectation: recoverGoPanic should handle nil panic values correctly.
func Test_recoverGoPanic_NilPanic_Recovered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	func() {
		defer recoverGoPanic("test", logger)
		// Pointless (pun intended) but I needed to trick the
		// code linters not to go crazy about panic(nil). ;-)
		var v *uintptr
		require.Nil(t, v)
		panic(v)
	}()

	output := buf.String()
	require.Contains(t, output, "panic recovered")
}
