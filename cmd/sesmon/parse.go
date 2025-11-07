package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// parseSES is the principal function for unmarshalling JSON-wrapped SES
// output into the program's internal map[string]Result result structure.
//
//nolint:nestif,gocognit
func parseSES(b []byte) (map[string]Result, error) {
	var root Root

	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("failure unmarshalling JSON: %w", err)
	}

	m := make(map[string]Result)
	for _, el := range root.Join.ElementList {
		r := Result{}
		if el.ElementType != nil {
			if el.ElementType.I != nil {
				r.Type = *el.ElementType.I
			} else {
				continue // required for ID
			}
			if el.ElementType.Meaning != nil {
				r.TypeDesc = ptr(strings.TrimSpace(*el.ElementType.Meaning))
			}
		}
		if el.ElementNumber != nil {
			r.TypeNum = *el.ElementNumber
		} else {
			continue // required for ID
		}
		if el.StatusDescriptor != nil {
			if el.StatusDescriptor.Status != nil {
				if el.StatusDescriptor.Status.I != nil {
					r.Status = el.StatusDescriptor.Status.I
				}
				if el.StatusDescriptor.Status.Meaning != nil {
					r.StatusDesc = ptr(strings.TrimSpace(*el.StatusDescriptor.Status.Meaning))
				}
			}
			if el.StatusDescriptor.PrdFail != nil {
				r.PrdFail = el.StatusDescriptor.PrdFail
			}
			if el.StatusDescriptor.Disabled != nil {
				r.Disabled = el.StatusDescriptor.Disabled
			}
			if el.StatusDescriptor.Swap != nil {
				r.Swap = el.StatusDescriptor.Swap
			}
			if el.StatusDescriptor.Temperature != nil {
				r.Temperature = ptr(strings.TrimSpace(*el.StatusDescriptor.Temperature.Meaning))
			}
			if el.StatusDescriptor.Voltage != nil {
				r.Voltage = ptr(strings.TrimSpace(*el.StatusDescriptor.Voltage.ValueInVolts))
			}
			if el.StatusDescriptor.Current != nil {
				r.Amperage = ptr(strings.TrimSpace(*el.StatusDescriptor.Current.ValueInAmps))
			}
		}
		m[keyFor(r)] = r
	}

	return m, nil
}

// rowsDiff compares two map[string]Result and returns a slice of [Change].
func rowsDiff(prev, curr map[string]Result) []Change {
	var out []Change
	seen := make(map[string]struct{})

	for k := range prev {
		seen[k] = struct{}{}
	}
	for k := range curr {
		seen[k] = struct{}{}
	}

	for k := range seen {
		p, pok := prev[k]
		c, cok := curr[k]
		if !pok || !cok || !rowsEqual(p, c) {
			ch := Change{ID: k, Type: fnz(c.Type, p.Type), TypeNum: fnz(c.TypeNum, p.TypeNum)}
			if c.TypeDesc != nil || p.TypeDesc != nil {
				ch.TypeDesc = ptr(fne(fmtPtrStr(c.TypeDesc, ""), fmtPtrStr(p.TypeDesc, "")))
			}
			if pok {
				ch.Before = &p
			}
			if cok {
				ch.After = &c
			}
			out = append(out, ch)
		}
	}

	return out
}

// rowsEqual returns if two [Result] should be considered as equal.
func rowsEqual(a, b Result) bool {
	return ptrIntEqual(a.Status, b.Status) &&
		ptrStrEqualFold(a.StatusDesc, b.StatusDesc) &&
		ptrIntEqual(a.PrdFail, b.PrdFail) &&
		ptrIntEqual(a.Disabled, b.Disabled) &&
		ptrIntEqual(a.Swap, b.Swap)
}

// buildMessage builds a string from a slice of strings.
func buildMessage(lines []string) string {
	return strings.Join(lines, " ")
}

// changesAsText formats a slice of [Change] into a textual representation.
func changesAsText(changes []Change) []string {
	out := make([]string, 0, len(changes))
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type == changes[j].Type {
			return changes[i].TypeNum < changes[j].TypeNum
		}

		return changes[i].Type < changes[j].Type
	})
	for _, ch := range changes {
		before := "-"
		if ch.Before != nil {
			before = fmt.Sprintf("status=%s status_txt=%s prdfail=%s disabled=%s swap=%s temp=%s volt=%s amp=%s",
				fmtPtrInt(ch.Before.Status, "-"), fmtPtrQStr(ch.Before.StatusDesc, "-"),
				fmtPtrInt(ch.Before.PrdFail, "-"), fmtPtrInt(ch.Before.Disabled, "-"), fmtPtrInt(ch.Before.Swap, "-"),
				fmtPtrQStr(ch.Before.Temperature, "-"), fmtPtrQStr(ch.Before.Voltage, "-"), fmtPtrQStr(ch.Before.Amperage, "-"))
		}
		after := "-"
		if ch.After != nil {
			after = fmt.Sprintf("status=%s status_txt=%s prdfail=%s disabled=%s swap=%s temp=%s volt=%s amp=%s",
				fmtPtrInt(ch.After.Status, "-"), fmtPtrQStr(ch.After.StatusDesc, "-"),
				fmtPtrInt(ch.After.PrdFail, "-"), fmtPtrInt(ch.After.Disabled, "-"), fmtPtrInt(ch.After.Swap, "-"),
				fmtPtrQStr(ch.After.Temperature, "-"), fmtPtrQStr(ch.After.Voltage, "-"), fmtPtrQStr(ch.After.Amperage, "-"))
		}
		out = append(out, fmt.Sprintf("[element=%q type=%s number=%d / Before: (%s) / After: (%s)]",
			ch.ID, fmtPtrQStr(ch.TypeDesc, "-"), ch.TypeNum, before, after))
	}

	return out
}

// keyFor is a helper function to derive a key from a [Result].
func keyFor(r Result) string {
	return fmt.Sprintf("%d#%d", r.Type, r.TypeNum) // Type#TypeNum
}
