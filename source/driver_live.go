package main

import (
	"crypto/sha256"
	"fmt"
	"math"
)

// A running game's counters remain cumulative. Rebase actual deltas onto
// BCS's freshly selected personnel baseline, without inventing a trip result.
func liveDriverRecord(actual, origin, baseline driverRecord) (driverRecord, error) {
	if err := driverAdvanced(actual, origin); err != nil {
		return driverRecord{}, err
	}
	out := actual
	out.Ident = baseline.Ident
	add := func(v, b, n int32) (int32, error) {
		x := int64(v) - int64(b) + int64(n)
		if x < 0 || x > math.MaxInt32 {
			return 0, fmt.Errorf("live driver counter out of range")
		}
		return int32(x), nil
	}
	var err error
	for i := range out.Stops {
		if out.Stops[i], err = add(actual.Stops[i], origin.Stops[i], baseline.Stops[i]); err != nil {
			return out, err
		}
	}
	for i := range out.Crashes {
		if out.Crashes[i], err = add(actual.Crashes[i], origin.Crashes[i], baseline.Crashes[i]); err != nil {
			return out, err
		}
	}
	if out.Hectom, err = add(actual.Hectom, origin.Hectom, baseline.Hectom); err != nil {
		return out, err
	}
	if out.Tickets, err = add(actual.Tickets, origin.Tickets, baseline.Tickets); err != nil {
		return out, err
	}
	out.Money = actual.Money - origin.Money + baseline.Money
	for i := range out.Rating {
		out.Rating[i] = actual.Rating[i] - origin.Rating[i] + baseline.Rating[i]
	}
	return parseDriver(out.encode(true), true)
}
func (s *driverSync) beginLiveTrip(origin, baseline driverRecord, hash [sha256.Size]byte) {
	s.LiveOrigin, s.LiveBaseline = &origin, &baseline
	s.Last = baseline
	s.NativeHash = hash
	s.NativeConflicted = false
	s.EvaluationFrozen = false
	s.LastBytes = origin.encode(true)
	s.Candidate = nil
	s.Pending = false
	_ = s.writeState("LIVE_TRIP_BASELINE_READY: previous evaluation retained; new counters use actual simulator deltas.")
}
