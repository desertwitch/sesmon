package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: parseSES should correctly unmarshal SES JSON data.
func Test_parseSES_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "Enclosure"},
					"element_number": 0,
					"status_descriptor": {
						"status": {"i": 1, "meaning": "OK"},
						"prdfail": 0,
						"disabled": 0,
						"swap": 0
					}
				},
				{
					"element_type": {"i": 23, "meaning": "Temperature sensor"},
					"element_number": 1,
					"status_descriptor": {
						"status": {"i": 1, "meaning": "OK"},
						"temperature": {"i": 25, "meaning": "25 C"}
					}
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Len(t, results, 2)

	enclosure := results["15#0"]
	require.Equal(t, 15, enclosure.Type)
	require.Equal(t, "Enclosure", *enclosure.TypeDesc)
	require.Equal(t, 0, enclosure.TypeNum)
	require.Equal(t, 1, *enclosure.Status)
	require.Equal(t, "OK", *enclosure.StatusDesc)
	require.Equal(t, 0, *enclosure.PrdFail)
	require.Equal(t, 0, *enclosure.Disabled)
	require.Equal(t, 0, *enclosure.Swap)

	temp := results["23#1"]
	require.Equal(t, 23, temp.Type)
	require.Equal(t, "Temperature sensor", *temp.TypeDesc)
	require.Equal(t, 1, temp.TypeNum)
	require.Equal(t, "25 C", *temp.Temperature)
}

// Expectation: parseSES should handle voltage and current fields.
func Test_parseSES_WithVoltageAndCurrent_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 2, "meaning": "Power supply"},
					"element_number": 0,
					"status_descriptor": {
						"status": {"i": 1, "meaning": "OK"},
						"voltage": {"raw_value": 120, "value_in_volts": "12.0 V"},
						"current": {"raw_value": 50, "value_in_amps": "5.0 A"}
					}
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Len(t, results, 1)

	psu := results["2#0"]
	require.Equal(t, "12.0 V", *psu.Voltage)
	require.Equal(t, "5.0 A", *psu.Amperage)
}

// Expectation: parseSES should handle missing required fields.
func Test_parseSES_MissingRequiredFields_Type_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"meaning": "Device"},
					"element_number": 5
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Empty(t, results)
}

// Expectation: parseSES should handle missing required fields.
func Test_parseSES_MissingRequired_TypeNum_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 10, "meaning": "Device"}
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Empty(t, results)
}

// Expectation: parseSES should handle missing optional fields.
func Test_parseSES_MissingOptionalFields_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 10},
					"element_number": 5
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Required fields must be there.
	dev := results["10#5"]
	require.Equal(t, 10, dev.Type)
	require.Equal(t, 5, dev.TypeNum)

	// Optional fields should be nil, not zero.
	require.Nil(t, dev.TypeDesc)
	require.Nil(t, dev.Status)
	require.Nil(t, dev.StatusDesc)
	require.Nil(t, dev.PrdFail)
	require.Nil(t, dev.Disabled)
	require.Nil(t, dev.Swap)
	require.Nil(t, dev.Temperature)
	require.Nil(t, dev.Voltage)
	require.Nil(t, dev.Amperage)
}

// Expectation: parseSES should trim whitespace from string fields.
func Test_parseSES_TrimsWhitespace_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": [
				{
					"element_type": {"i": 15, "meaning": "  Enclosure  "},
					"element_number": 0,
					"status_descriptor": {
						"status": {"i": 1, "meaning": "  OK  "},
						"temperature": {"i": 25, "meaning": "  25 C  "},
						"voltage": {"value_in_volts": "  12.0 V  "},
						"current": {"value_in_amps": "  5.0 A  "}
					}
				}
			]
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)

	r := results["15#0"]
	require.Equal(t, "Enclosure", *r.TypeDesc)
	require.Equal(t, "OK", *r.StatusDesc)
	require.Equal(t, "25 C", *r.Temperature)
	require.Equal(t, "12.0 V", *r.Voltage)
	require.Equal(t, "5.0 A", *r.Amperage)
}

// Expectation: parseSES should fail on invalid JSON.
func Test_parseSES_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`not json`)

	results, err := parseSES(jsonData)
	require.Error(t, err)
	require.Nil(t, results)
	require.Contains(t, err.Error(), "failure unmarshalling JSON")
}

// Expectation: parseSES should handle empty element list.
func Test_parseSES_EmptyElementList_Success(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{
		"join_of_diagnostic_pages": {
			"element_list": []
		}
	}`)

	results, err := parseSES(jsonData)
	require.NoError(t, err)
	require.Empty(t, results)
}

// Expectation: rowsDiff should detect no changes when results are equal.
func Test_rowsDiff_NoChanges_Success(t *testing.T) {
	t.Parallel()

	prev := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
	}
	curr := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
	}

	changes := rowsDiff(prev, curr)
	require.Empty(t, changes)
}

// Expectation: rowsDiff should detect status changes.
func Test_rowsDiff_StatusChange_Success(t *testing.T) {
	t.Parallel()

	prev := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
	}
	curr := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(2), StatusDesc: ptr("Critical")},
	}

	changes := rowsDiff(prev, curr)
	require.Len(t, changes, 1)
	require.Equal(t, "15#0", changes[0].ID)
	require.NotNil(t, changes[0].Before)
	require.NotNil(t, changes[0].After)
	require.Equal(t, 1, *changes[0].Before.Status)
	require.Equal(t, 2, *changes[0].After.Status)
}

// Expectation: rowsDiff should detect new elements.
func Test_rowsDiff_NewElement_Success(t *testing.T) {
	t.Parallel()

	prev := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
	}
	curr := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
		"23#1": {Type: 23, TypeDesc: ptr("Temperature"), TypeNum: 1, Status: ptr(1), StatusDesc: ptr("OK")},
	}

	changes := rowsDiff(prev, curr)
	require.Len(t, changes, 1)
	require.Equal(t, "23#1", changes[0].ID)
	require.Nil(t, changes[0].Before)
	require.NotNil(t, changes[0].After)
}

// Expectation: rowsDiff should detect removed elements.
func Test_rowsDiff_RemovedElement_Success(t *testing.T) {
	t.Parallel()

	prev := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
		"23#1": {Type: 23, TypeDesc: ptr("Temperature"), TypeNum: 1, Status: ptr(1), StatusDesc: ptr("OK")},
	}
	curr := map[string]Result{
		"15#0": {Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, Status: ptr(1), StatusDesc: ptr("OK")},
	}

	changes := rowsDiff(prev, curr)
	require.Len(t, changes, 1)
	require.Equal(t, "23#1", changes[0].ID)
	require.NotNil(t, changes[0].Before)
	require.Nil(t, changes[0].After)
}

// Expectation: rowsDiff should ignore temperature, voltage, amperage changes.
func Test_rowsDiff_IgnoresMetrics_Success(t *testing.T) {
	t.Parallel()

	prev := map[string]Result{
		"23#1": {Type: 23, TypeNum: 1, Status: ptr(1), StatusDesc: ptr("OK"), Temperature: ptr("25 C"), Voltage: ptr("12.0 V"), Amperage: ptr("5.0 A")},
	}
	curr := map[string]Result{
		"23#1": {Type: 23, TypeNum: 1, Status: ptr(1), StatusDesc: ptr("OK"), Temperature: ptr("30 C"), Voltage: ptr("12.5 V"), Amperage: ptr("5.5 A")},
	}

	changes := rowsDiff(prev, curr)
	require.Empty(t, changes)
}

// Expectation: rowsEqual should return true for identical results.
func Test_rowsEqual_Identical_Success(t *testing.T) {
	t.Parallel()

	a := Result{Status: ptr(1), StatusDesc: ptr("OK"), PrdFail: ptr(0), Disabled: ptr(0), Swap: ptr(0)}
	b := Result{Status: ptr(1), StatusDesc: ptr("OK"), PrdFail: ptr(0), Disabled: ptr(0), Swap: ptr(0)}

	require.True(t, rowsEqual(a, b))
}

// Expectation: rowsEqual should be case-insensitive for StatusDesc.
func Test_rowsEqual_CaseInsensitive_Success(t *testing.T) {
	t.Parallel()

	a := Result{Status: ptr(1), StatusDesc: ptr("OK"), PrdFail: ptr(0), Disabled: ptr(0), Swap: ptr(0)}
	b := Result{Status: ptr(1), StatusDesc: ptr("OK"), PrdFail: ptr(0), Disabled: ptr(0), Swap: ptr(0)}

	require.True(t, rowsEqual(a, b))
}

// Expectation: rowsEqual should return false for different status.
func Test_rowsEqual_DifferentStatus_Success(t *testing.T) {
	t.Parallel()

	a := Result{Status: ptr(1), StatusDesc: ptr("OK")}
	b := Result{Status: ptr(2), StatusDesc: ptr("OK")}

	require.False(t, rowsEqual(a, b))
}

// Expectation: buildMessage should concatenate lines with spaces.
func Test_buildMessage_Success(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3"}
	msg := buildMessage(lines)

	require.Equal(t, "line1 line2 line3", msg)
}

// Expectation: buildMessage should handle empty lines.
func Test_buildMessage_EmptyLines_Success(t *testing.T) {
	t.Parallel()

	lines := []string{}
	msg := buildMessage(lines)

	require.Empty(t, msg)
}

// Expectation: changesAsText should format changes correctly.
func Test_changesAsText_Success(t *testing.T) {
	t.Parallel()

	changes := []Change{
		{
			ID:       "15#0",
			Type:     15,
			TypeDesc: ptr("Enclosure"),
			TypeNum:  0,
			Before:   &Result{Status: ptr(1), StatusDesc: ptr("OK"), PrdFail: ptr(0), Disabled: ptr(0), Swap: ptr(0)},
			After:    &Result{Status: ptr(2), StatusDesc: ptr("Critical"), PrdFail: ptr(1), Disabled: ptr(0), Swap: ptr(0)},
		},
	}

	lines := changesAsText(changes)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], "element=\"15#0\"")
	require.Contains(t, lines[0], "type=\"Enclosure\"")
	require.Contains(t, lines[0], "number=0")
	require.Contains(t, lines[0], "status=1")
	require.Contains(t, lines[0], "status=2")
}

// Expectation: changesAsText should sort changes by ID.
func Test_changesAsText_Sorted_Success(t *testing.T) {
	t.Parallel()

	changes := []Change{
		{ID: "23#1", Type: 23, TypeDesc: ptr("Temp"), TypeNum: 1, After: &Result{Status: ptr(1)}},
		{ID: "23#2", Type: 23, TypeDesc: ptr("Temp"), TypeNum: 2, After: &Result{Status: ptr(1)}},
		{ID: "15#0", Type: 15, TypeDesc: ptr("Enclosure"), TypeNum: 0, After: &Result{Status: ptr(1)}},
		{ID: "2#5", Type: 2, TypeDesc: ptr("PSU"), TypeNum: 5, After: &Result{Status: ptr(1)}},
	}

	lines := changesAsText(changes)
	require.Len(t, lines, 4)
	require.Contains(t, lines[0], "2#5")
	require.Contains(t, lines[1], "15#0")
	require.Contains(t, lines[2], "23#1")
	require.Contains(t, lines[3], "23#2")
}

// Expectation: changesAsText should handle nil Before.
func Test_changesAsText_NilBefore_Success(t *testing.T) {
	t.Parallel()

	changes := []Change{
		{
			ID:       "15#0",
			Type:     15,
			TypeDesc: ptr("Enclosure"),
			TypeNum:  0,
			Before:   nil,
			After:    &Result{Status: ptr(1), StatusDesc: ptr("OK")},
		},
	}

	lines := changesAsText(changes)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], "Before: (-)")
}

// Expectation: changesAsText should handle nil After.
func Test_changesAsText_NilAfter_Success(t *testing.T) {
	t.Parallel()

	changes := []Change{
		{
			ID:       "15#0",
			Type:     15,
			TypeDesc: ptr("Enclosure"),
			TypeNum:  0,
			Before:   &Result{Status: ptr(1), StatusDesc: ptr("OK")},
			After:    nil,
		},
	}

	lines := changesAsText(changes)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], "After: (-)")
}

// Expectation: keyFor should generate consistent keys from Result.
func Test_keyFor_Success(t *testing.T) {
	t.Parallel()

	r := Result{Type: 15, TypeNum: 3}
	key := keyFor(r)
	require.Equal(t, "15#3", key)

	r2 := Result{Type: 0, TypeNum: 0}
	key2 := keyFor(r2)
	require.Equal(t, "0#0", key2)
}
