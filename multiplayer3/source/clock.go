package main

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Windows installs need not provide an IANA timezone database.
)

// CompanyClock describes the BCS company's offset from its reference civil
// clock. ShiftMinutes is applied to the displayed date and time, after the
// reference timezone's daylight-saving rule has been resolved.
type CompanyClock struct {
	TimeZone     string `json:"timezone"`
	ShiftMinutes int    `json:"shift_minutes"`
}

func validateCompanyClock(clock CompanyClock) error {
	if clock.TimeZone == "" || clock.TimeZone != strings.TrimSpace(clock.TimeZone) || clock.TimeZone == "Local" || len(clock.TimeZone) > 100 {
		return fmt.Errorf("company clock needs an explicit IANA timezone, such as Europe/Berlin")
	}
	if clock.ShiftMinutes < -1440 || clock.ShiftMinutes > 1440 {
		return fmt.Errorf("company clock shift must be between -1440 and 1440 minutes")
	}
	if _, err := time.LoadLocation(clock.TimeZone); err != nil {
		return fmt.Errorf("invalid company clock timezone %q: %w", clock.TimeZone, err)
	}
	return nil
}

// companyNow returns a civil clock represented in UTC for calendar arithmetic;
// it is not the physical instant at the company. Subtracting a duration from a
// time in Europe/Berlin directly can apply the wrong displayed offset when that
// duration crosses a daylight-saving transition. Copying the reference clock's
// fields first makes "Berlin's displayed time minus eight hours" unambiguous.
func companyNow(clock CompanyClock, now time.Time) (time.Time, error) {
	if err := validateCompanyClock(clock); err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(clock.TimeZone)
	if err != nil {
		return time.Time{}, err
	}
	reference := now.In(location)
	civil := time.Date(reference.Year(), reference.Month(), reference.Day(), reference.Hour(), reference.Minute(), reference.Second(), reference.Nanosecond(), time.UTC)
	return civil.Add(time.Duration(clock.ShiftMinutes) * time.Minute), nil
}

// Explicit historical BCS calendar choices still take precedence over a live clock.
func automaticBridgeDate(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto", "system", "today":
		return true
	}
	return false
}
