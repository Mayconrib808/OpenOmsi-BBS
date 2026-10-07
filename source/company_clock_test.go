package main

import (
	"testing"
	"time"
)

func TestCompanyClockTracksBCSCivilOffsetAcrossCalendarAndDSTChanges(t *testing.T) {
	clock := CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	for _, tc := range []struct {
		name, utc, want string
	}{
		{"user midnight example", "2026-10-07T03:14:00Z", "2026-10-06T21:14:00"},
		{"summer company midnight", "2026-10-07T06:00:00Z", "2026-10-07T00:00:00"},
		{"winter offset", "2026-12-15T03:14:00Z", "2026-12-14T20:14:00"},
		{"year boundary", "2027-01-01T02:14:00Z", "2026-12-31T19:14:00"},
		{"month boundary", "2026-11-01T02:14:00Z", "2026-10-31T19:14:00"},
		{"before spring transition", "2026-03-29T00:30:00Z", "2026-03-28T17:30:00"},
		{"after spring transition", "2026-03-29T01:30:00Z", "2026-03-28T19:30:00"},
		{"first autumn repeated hour", "2026-10-25T00:30:00Z", "2026-10-24T18:30:00"},
		{"second autumn repeated hour", "2026-10-25T01:30:00Z", "2026-10-24T18:30:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.utc)
			if err != nil {
				t.Fatal(err)
			}
			got, err := companyNow(clock, now)
			if err != nil || got.Format("2006-01-02T15:04:05") != tc.want {
				t.Fatalf("company clock = %v, %v; want %s", got, err, tc.want)
			}
			if got.Location() != time.UTC {
				t.Fatal("company clock must preserve civil calendar arithmetic without host timezone rules")
			}
		})
	}
}

func TestCompanyClockIgnoresComputerTimezoneAndPreservesFractionalSeconds(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 14, 12, 345000000, time.FixedZone("computer", -3*3600))
	clock := CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	first, err := companyNow(clock, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := companyNow(clock, now.In(time.FixedZone("other computer", 9*3600)))
	if err != nil || !first.Equal(second) || first.Format("2006-01-02 15:04:05.000") != "2026-10-06 21:14:12.345" {
		t.Fatal(first, second, err)
	}
}

func TestCompanyClockSupportsMinuteOffsetsAndBoundedDayShifts(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	for _, tc := range []struct {
		shift int
		want  string
	}{
		{-1440, "2025-12-31 00:15"},
		{-30, "2025-12-31 23:45"},
		{0, "2026-01-01 00:15"},
		{90, "2026-01-01 01:45"},
		{1440, "2026-01-02 00:15"},
	} {
		got, err := companyNow(CompanyClock{TimeZone: "UTC", ShiftMinutes: tc.shift}, now)
		if err != nil || got.Format("2006-01-02 15:04") != tc.want {
			t.Fatal(tc.shift, got, err)
		}
	}
}

func TestCompanyClockRejectsUnknownHostDependentAndUnboundedSettings(t *testing.T) {
	for _, clock := range []CompanyClock{
		{},
		{TimeZone: "Local"},
		{TimeZone: " Europe/Berlin"},
		{TimeZone: "Europe/Berlin "},
		{TimeZone: "Europe/NotAPlace"},
		{TimeZone: "../Europe/Berlin"},
		{TimeZone: "UTC", ShiftMinutes: -1441},
		{TimeZone: "UTC", ShiftMinutes: 1441},
	} {
		if err := validateCompanyClock(clock); err == nil {
			t.Fatalf("invalid clock accepted: %#v", clock)
		}
		if got, err := companyNow(clock, time.Now()); err == nil || !got.IsZero() {
			t.Fatalf("invalid clock produced time: %#v -> %v, %v", clock, got, err)
		}
	}
}
