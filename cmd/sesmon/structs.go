package main

import (
	"encoding/json"
)

// SES Structure (as returned by the "sg_ses" program)
// ------------------------------------------------------

type Root struct {
	Join JoinPages `json:"join_of_diagnostic_pages"`
}

type JoinPages struct {
	ElementList []Element `json:"element_list"`
}

type Element struct {
	ElementType      *ElementType      `json:"element_type,omitempty"`
	ElementNumber    *int              `json:"element_number,omitempty"`
	StatusDescriptor *StatusDescriptor `json:"status_descriptor,omitempty"`
}

type ElementType struct {
	I       *int    `json:"i,omitempty"`
	Meaning *string `json:"meaning,omitempty"`
}

type StatusDescriptor struct {
	Status   *CodeMeaning `json:"status,omitempty"`
	PrdFail  *int         `json:"prdfail,omitempty"`
	Disabled *int         `json:"disabled,omitempty"`
	Swap     *int         `json:"swap,omitempty"`

	Temperature *CodeMeaning `json:"temperature,omitempty"`
	Voltage     *Voltage     `json:"voltage,omitempty"`
	Current     *Current     `json:"current,omitempty"`
}

type CodeMeaning struct {
	I       *int    `json:"i,omitempty"`
	Meaning *string `json:"meaning,omitempty"`
}

type Voltage struct {
	RawValue     *int    `json:"raw_value,omitempty"`
	ValueInVolts *string `json:"value_in_volts,omitempty"`
}

type Current struct {
	RawValue    *int    `json:"raw_value,omitempty"`
	ValueInAmps *string `json:"value_in_amps,omitempty"`
}

// Internal Structure
// ------------------------------------------------------

// Device is the device information needed for monitoring.
type Device struct {
	Type        int    `json:"type"` // 0 = Device, 1 = JSON File
	Path        string `json:"path"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

// DeviceSnapshot is a snapshot of the [Device] in a certain state.
type DeviceSnapshot struct {
	Device     Device          `json:"device"`
	CapturedAt string          `json:"captured_at"`
	Raw        json.RawMessage `json:"raw"`
}

// Result is a single parsed [Element] with the fields relevant for us.
type Result struct {
	Type    int `json:"element_type"`        // element type (as integer)
	TypeNum int `json:"element_type_number"` // element number (of type)

	TypeDesc   *string `json:"element_type_desc,omitempty"` // element type (as text)
	Status     *int    `json:"status,omitempty"`
	StatusDesc *string `json:"status_desc,omitempty"`
	PrdFail    *int    `json:"prdfail,omitempty"`
	Disabled   *int    `json:"disabled,omitempty"`
	Swap       *int    `json:"swap,omitempty"`

	Temperature *string `json:"temperature,omitempty"`
	Voltage     *string `json:"voltage,omitempty"`  // value_in_volts
	Amperage    *string `json:"amperage,omitempty"` // value_in_amps
}

// Change is a single change between two [Element] (internally [Result]).
type Change struct {
	ID      string `json:"id"`
	Type    int    `json:"element_type"`
	TypeNum int    `json:"element_type_number"`

	TypeDesc *string `json:"element_type_desc,omitempty"`
	Before   *Result `json:"before,omitempty"`
	After    *Result `json:"after,omitempty"`
}

// ChangeReport is a report of all [Change] between two [Device] polls.
type ChangeReport struct {
	Device     Device   `json:"device"`
	DetectedAt string   `json:"detected_at"`
	Changes    []Change `json:"changes"`
}
