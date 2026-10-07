package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func companyClockTestInstant(t *testing.T, text string) time.Time {
	t.Helper()
	now, err := time.Parse(time.RFC3339, text)
	if err != nil {
		t.Fatal(err)
	}
	return now
}

func TestLiveCompanyPreflightResolvesCalendarAndPreservesBCSDeparture(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	p.Sessions[0].Date = "company"
	status := testCompanyStatus()
	status["time"] = "21:14"
	p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
	saveTestCompany(t, dir, p)
	// The computer is already on October 7; the company is still on October 6.
	trip.Date = "2026-10-07"
	trip.Start = "00:00" // A timetable departure does not reset the shared world.
	now := companyClockTestInstant(t, "2026-10-07T03:14:00Z")
	plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now)
	if err != nil || len(problems) != 0 || plan == nil {
		t.Fatal(plan, problems, err)
	}
	if plan.Trip.Date != "2026-10-06" || plan.Trip.Start != "00:00" || plan.Clock == nil || plan.Session.Date != "company" {
		t.Fatal("company launch calendar or independent departure was lost", plan)
	}
	if trip.Date != "2026-10-07" || c.Date != "auto" {
		t.Fatal("preflight changed the caller's trip or configuration")
	}
	ready, err := checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 21:14:02.15 weather clear season summer\n", plan, now)
	if err != nil || !ready {
		t.Fatal("live server incorrectly compared to the timetable departure", ready, err)
	}
}

func TestLiveCompanySessionNeverReplacesExplicitHistoricalDate(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	p.Sessions[0].Date = "company"
	status := testCompanyStatus()
	status["time"] = "21:14"
	p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
	saveTestCompany(t, dir, p)
	c.Date, trip.Date = "2026-10-05", "2026-10-05"
	now := companyClockTestInstant(t, "2026-10-07T03:14:00Z")
	plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now)
	if err != nil || plan != nil || len(problems) == 0 {
		t.Fatal("historical trip silently joined today's company session", plan, problems, err)
	}
	if trip.Date != "2026-10-05" || c.Date != "2026-10-05" {
		t.Fatal("explicit date was overwritten")
	}
}

func TestLiveCompanyPreflightRejectsClockDriftAndInvalidStatus(t *testing.T) {
	for _, serverTime := range []string{"09:20", "21:10", "21:18", "24:00", "21:14:NaN"} {
		t.Run(serverTime, func(t *testing.T) {
			c, p, trip, dir := companyFixture(t)
			p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
			p.Sessions[0].Date = "company"
			status := testCompanyStatus()
			status["time"] = serverTime
			p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
			saveTestCompany(t, dir, p)
			now := companyClockTestInstant(t, "2026-10-07T03:14:00Z")
			plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now)
			if err != nil || plan != nil || len(problems) == 0 {
				t.Fatal("bad live server clock accepted", plan, problems, err)
			}
		})
	}
}

func TestLiveCompanyWorldGuardRejectsWrongDateDespiteMatchingClock(t *testing.T) {
	_, p, trip, _ := companyFixture(t)
	plan := &MultiplayerPlan{Session: p.Sessions[0], Trip: trip, Clock: &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}}
	now := companyClockTestInstant(t, "2026-10-07T03:14:00Z")
	for _, worldDate := range []string{"2026-10-05", "2026-10-07", "2025-10-06"} {
		ready, err := checkMultiplayerLogAt("LAN: taking the host's world: "+worldDate+" 21:14:00 weather clear season summer\n", plan, now)
		if ready || err == nil {
			t.Fatal("wrong server day accepted", worldDate, ready, err)
		}
	}
}

func TestLiveCompanyWorldGuardUsesCurrentTimeAfterLaunchDelay(t *testing.T) {
	_, p, trip, _ := companyFixture(t)
	plan := &MultiplayerPlan{Session: p.Sessions[0], Trip: trip, Clock: &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}}
	plan.Trip.Date, plan.Trip.Start = "2026-10-06", "21:10"
	now := companyClockTestInstant(t, "2026-10-07T03:20:00Z")
	ready, err := checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 21:20:00.50 weather clear season summer\n", plan, now)
	if !ready || err != nil {
		t.Fatal("live clock was incorrectly frozen at departure or preparation", ready, err)
	}
	ready, err = checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 21:14:00 weather clear season summer\n", plan, now)
	if ready || err == nil {
		t.Fatal("stale startup world passed live clock guard", ready, err)
	}
}

func TestLiveCompanyClockHandlesMidnightWithAbsoluteJoinedCalendar(t *testing.T) {
	clock := &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	_, p, trip, _ := companyFixture(t)
	plan := &MultiplayerPlan{Session: p.Sessions[0], Trip: trip, Clock: clock}
	plan.Trip.Date, plan.Trip.Start = "2026-10-07", "00:00"
	now := companyClockTestInstant(t, "2026-10-07T06:00:20Z")
	status := testCompanyStatus()
	status["time"] = "23:59"
	plan.Session.ServerURL = companyStatusServer(t, status).URL
	serverStatus, err := readCompanyServer(context.Background(), companyHTTPClient(), plan.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCompanyServerAt(serverStatus, plan.Session, plan.Trip, clock, now); err != nil {
		t.Fatal("status with no date should permit a near-midnight clock for the world guard to verify", err)
	}
	ready, err := checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-07 00:00:00.75 weather clear season summer\n", plan, now)
	if !ready || err != nil {
		t.Fatal("new-day world refused despite a matching company date and near-midnight clock", ready, err)
	}
	ready, err = checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 00:00:20 weather clear season summer\n", plan, now)
	if ready || err == nil {
		t.Fatal("one-day mismatch hidden by circular HH:MM comparison", ready, err)
	}
	ready, err = checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 23:59:59.75 weather clear season summer\n", plan, now)
	if ready || err == nil {
		t.Fatal("previous calendar date accepted despite timetable generated for the new day", ready, err)
	}
	// If map loading crosses midnight, the already-built timetable's date cannot
	// silently change. Retrying the trip resolves a fresh company date.
	plan.Trip.Date = "2026-10-06"
	ready, err = checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-07 00:00:20 weather clear season summer\n", plan, now)
	if ready || err == nil {
		t.Fatal("launch crossed midnight and silently replaced the prepared trip date", ready, err)
	}
	ready, err = checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 23:59:59 weather clear season summer\n", plan, now)
	if ready || err == nil {
		t.Fatal("world stayed on the prepared day after company calendar rolled over", ready, err)
	}
}

func TestFixedCompanySessionRetainsDepartureClockWithLiveProfileClock(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	// A manual historical date and fixed session keep the previous behavior.
	c.Date = trip.Date
	p.Sessions[0].ServerURL = companyStatusServer(t, testCompanyStatus()).URL
	saveTestCompany(t, dir, p)
	now := companyClockTestInstant(t, "2026-10-10T03:14:00Z")
	plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now)
	if err != nil || len(problems) != 0 || plan == nil || plan.Clock != nil || plan.Trip != trip {
		t.Fatal("fixed-date historical mode changed", plan, problems, err)
	}
	ready, err := checkMultiplayerLogAt("LAN: taking the host's world: 2026-10-06 09:20:01 weather clear season summer\n", plan, now)
	if !ready || err != nil {
		t.Fatal("fixed session incorrectly checked the live company clock", ready, err)
	}
}

func TestCompanyProfileRequiresClockForAutomaticSessionDate(t *testing.T) {
	_, p, _, _ := companyFixture(t)
	p.Sessions[0].Date = "company"
	if err := validateCompanyProfile(p); err == nil {
		t.Fatal("automatic company date without an explicit clock accepted")
	}
	p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	if err := validateCompanyProfile(p); err != nil {
		t.Fatal("valid company clock profile rejected", err)
	}
	p.Clock.ShiftMinutes = -1441
	if err := validateCompanyProfile(p); err == nil {
		t.Fatal("invalid company clock accepted through profile validation")
	}
}

func TestLiveCompanyPreflightRejectsLoadingWorldBeforeCheckingStartupClock(t *testing.T) {
	for _, world := range []any{nil, false, []int{}, "loading"} {
		c, p, trip, dir := companyFixture(t)
		p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
		p.Sessions[0].Date = "company"
		status := testCompanyStatus()
		status["time"], status["world"] = "22:30:10", world
		p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
		saveTestCompany(t, dir, p)
		now := companyClockTestInstant(t, "2026-10-07T04:37:41Z")
		plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now)
		if err != nil || plan != nil || len(problems) != 1 ||
			!strings.Contains(problems[0].Detail, "SINCRONIZADO") ||
			strings.Contains(problems[0].Detail, "incompatible clock") {
			t.Fatal("loading status was mistaken for the running company clock", world, plan, problems, err)
		}
	}
}

func TestLiveCompanyPreflightStillRequiresWorldWithMatchingClock(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	p.Sessions[0].Date = "company"
	status := testCompanyStatus()
	status["time"] = "22:37:41"
	delete(status, "world")
	p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
	saveTestCompany(t, dir, p)
	now := companyClockTestInstant(t, "2026-10-07T04:37:41Z")
	if plan, problems, err := prepareMultiplayerAt(context.Background(), c, dir, trip, companyHTTPClient(), now); err != nil || plan != nil || len(problems) == 0 {
		t.Fatal("startup clock coincidence allowed a company trip before the world loaded", plan, problems, err)
	}
}

func TestFixedDatePlayerHostDoesNotRequireDedicatedWorldCounts(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	status := testCompanyStatus()
	status["world"] = nil // A player-hosted session has no dedicated world counts.
	p.Sessions[0].ServerURL = companyStatusServer(t, status).URL
	saveTestCompany(t, dir, p)
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
	if err != nil || plan == nil || len(problems) != 0 {
		t.Fatal("fixed-date player host became unsupported", plan, problems, err)
	}
}
