package main

import "regexp"

var multilineShiftID = regexp.MustCompile(`(?m)\bSchicht ID:\s*([1-9][0-9]*)[ \t]*\r?$`)

// nextTripGate watches a BCS "next trip" request without interpreting a
// timetable retry or an old log block as permission to stop the running game.
// Completion belongs to the currently running shift; the replacement must
// have a distinct shift ID and its own complete map/line/tour block.
type nextTripGate struct {
	completion  completionGate
	baselineIDs map[string]bool
	completed   bool
	delivered   string
}

func newNextTripGate(id, baseline string) nextTripGate {
	g := nextTripGate{completion: newCompletionGate(id, baseline), baselineIDs: map[string]bool{}}
	for _, m := range multilineShiftID.FindAllStringSubmatch(baseline, -1) {
		g.baselineIDs[m[1]] = true
	}
	return g
}

func (g *nextTripGate) observe(text string) (TripInfo, bool) {
	if !numericShiftID.MatchString(g.completion.ShiftID) || g.completion.AlreadyClosed {
		return TripInfo{}, false
	}
	// Prefer explicit event order when the completion remains in the log.
	// If BCS trims/replaces its history, keep the completion already observed,
	// but still reject every shift that existed before this launch.
	suffix := text
	completionEnd := -1
	for _, m := range shiftClosedLine.FindAllStringSubmatchIndex(text, -1) {
		if text[m[2]:m[3]] == g.completion.ShiftID {
			completionEnd = m[1]
		}
	}
	if completionEnd >= 0 {
		g.completed = true
		for _, m := range multilineShiftID.FindAllStringSubmatch(text[:completionEnd], -1) {
			g.baselineIDs[m[1]] = true
		}
		suffix = text[completionEnd:]
	}
	if !g.completed {
		for _, m := range multilineShiftID.FindAllStringSubmatch(text, -1) {
			g.baselineIDs[m[1]] = true
		}
		return TripInfo{}, false
	}
	ids := multilineShiftID.FindAllStringSubmatchIndex(suffix, -1)
	if len(ids) == 0 {
		return TripInfo{}, false
	}
	last := ids[len(ids)-1]
	id := suffix[last[2]:last[3]]
	if id == g.completion.ShiftID || id == g.delivered || g.baselineIDs[id] || shiftClosed(text, id) {
		return TripInfo{}, false
	}
	// Parse only the last shift's block. Parsing the whole log could return the
	// previous trip while the new Karte/Tour/Umlauf lines are still being written.
	trip, err := parseBCSLogText(suffix[last[0]:])
	if err != nil || trip.ShiftID != id || trip.MapName == "" || trip.Line == "" || trip.Tour == "" || trip.TripStart == "" || trip.TripEnd == "" || trip.RouteText == "" {
		return TripInfo{}, false
	}
	g.delivered = id
	return trip, true
}
