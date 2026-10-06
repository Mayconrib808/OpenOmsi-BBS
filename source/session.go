package main

import "regexp"

var shiftIDLine = regexp.MustCompile(`\bSchicht ID:\s*([1-9][0-9]*)\s*$`)
var shiftClosedLine = regexp.MustCompile(`(?m)\bSchicht abschlie(?:ß|ss)en:\s*([1-9][0-9]*)\s*$`)
var numericShiftID = regexp.MustCompile(`^[1-9][0-9]*$`)

type completionGate struct {
	ShiftID       string
	AlreadyClosed bool
}

func shiftClosed(text, id string) bool {
	if !numericShiftID.MatchString(id) {
		return false
	}
	for _, match := range shiftClosedLine.FindAllStringSubmatch(text, -1) {
		if match[1] == id {
			return true
		}
	}
	return false
}

func newCompletionGate(id, baseline string) completionGate {
	return completionGate{ShiftID: id, AlreadyClosed: shiftClosed(baseline, id)}
}

// Match this launch's shift, even when BCS rewrites/trims its log history.
// An older shift, a partial ID, or a shift closed before launch cannot authorize
// closing the simulator. Process exit of the facade is checked by the caller.
func (g completionGate) accepts(text string) bool {
	return !g.AlreadyClosed && shiftClosed(text, g.ShiftID)
}
