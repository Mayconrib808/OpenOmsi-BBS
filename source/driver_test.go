package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSyntheticTripMatchesBCSMemoryLayout(t *testing.T) {
	before, err := parseDriver(fixture(t, "bbs-before.odr"), false)
	if err != nil {
		t.Fatal(err)
	}
	after, err := parseDriver(fixture(t, "bbs-after.odr"), false)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stops[0]-before.Stops[0] != 15 || after.Stops[1]-before.Stops[1] != 0 || after.Stops[2]-before.Stops[2] != 3 {
		t.Fatal("native trip stop deltas were not preserved")
	}
	const observed = "1015####20####8####2047####1####2####0####0####12####27.500000####3####4####2####4####0.500000"
	if got := after.evaluation(); got != observed {
		t.Fatalf("synthetic BCS evaluation differs:\n%s", got)
	}
	open, err := parseDriver(after.encode(true), true)
	if err != nil {
		t.Fatal(err)
	}
	if got := open.evaluation(); got != observed {
		t.Fatalf("conversion lost native values:\n%s", got)
	}
	if !bytes.HasPrefix(after.encode(true), []byte{0xff, 0xfe}) {
		t.Fatal("openOMSI file lacks UTF-16LE BOM")
	}
}

func TestImperfectComfortAndTicketPercentAreNotInverted(t *testing.T) {
	d, err := parseDriver(fixture(t, "bbs-before.odr"), false)
	if err != nil {
		t.Fatal(err)
	}
	d.Rating = [5]float64{10, 7, 8, 6, 0.4} // 70% comfort, 75% tickets, 60% driving
	openText, err := driverText(d.encode(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(openText, "[rating]\r\n0.400000000\r\n7.000000000\r\n4.000000000\r\n6.000000000\r\n10.000000000") {
		t.Fatal("wrong openOMSI rating order or ticket scaling")
	}
	back, err := parseDriver(d.encode(true), true)
	if err != nil {
		t.Fatal(err)
	}
	b := back.memoryBlock()
	value := func(off int) float64 { return float64(binary.LittleEndian.Uint32(b[off:])) }
	comfort := 100 * value(72) / value(84)
	tickets := 100 * value(80) / value(76)
	driving := 100 * (1 - math.Float64frombits(binary.LittleEndian.Uint64(b[64:])))
	if comfort != 70 || tickets != 75 || math.Abs(driving-60) > 1e-8 {
		t.Fatalf("percentages %g / %g / %g", comfort, tickets, driving)
	}
	native, err := parseDriver(back.encode(false), false)
	if err != nil || native.evaluation() != back.evaluation() {
		t.Fatal("native file fallback disagrees with memory")
	}
}

func newSync(t *testing.T) *driverSync {
	t.Helper()
	dir := t.TempDir()
	native := filepath.Join(dir, "bbs.odr")
	if err := os.WriteFile(native, fixture(t, "bbs-before.odr"), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := prepareDriver(native, dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeSaved(t *testing.T, s *driverSync, d driverRecord) {
	t.Helper()
	if err := os.WriteFile(s.OpenPath, d.encode(true), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestF9FilePublicationPreservesBaselineAndAvoidsDoubleCounting(t *testing.T) {
	s := newSync(t)
	before, err := os.ReadFile(s.NativePath)
	if err != nil {
		t.Fatal(err)
	}
	d := s.Last
	d.Stops[0] += 15
	d.Stops[2] += 3
	d.Hectom += 46
	d.Rating = [5]float64{10, 7, 8, 6, 0.4}
	writeSaved(t, s, d)
	if changed, err := s.poll(); changed || err != nil {
		t.Fatal("first read should wait for stability", changed, err)
	}
	if changed, err := s.poll(); !changed || err != nil {
		t.Fatal("saved trip was not published", changed, err)
	}
	nativeBytes, err := os.ReadFile(s.NativePath)
	if err != nil {
		t.Fatal(err)
	}
	native, err := parseDriver(nativeBytes, false)
	if err != nil || native.evaluation() != d.evaluation() {
		t.Fatal("BCS .odr and memory do not match", err)
	}
	backup, err := os.ReadFile(filepath.Join(filepath.Dir(s.OpenPath), "bcs-driver-before-v1.1.3.odr"))
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatal("original profile was not backed up exactly")
	}
	if changed, err := s.poll(); changed || err != nil {
		t.Fatal("unchanged save was counted twice", changed, err)
	}
	if s.Last.Stops[0] != 1015 {
		t.Fatal("stop counters changed on repeated reads")
	}
}

func TestPartialWritesResetsAndOtherDriversRetainLastValidData(t *testing.T) {
	s := newSync(t)
	original := s.Last.evaluation()
	valid := s.Last
	for _, bad := range [][]byte{nil, valid.encode(true)[:50], append(valid.encode(true), byte(0xff)), []byte("[busstops]\n0\n0\n0\n")} {
		if err := os.WriteFile(s.OpenPath, bad, 0644); err != nil {
			t.Fatal(err)
		}
		_, _ = s.poll()
		if changed, err := s.poll(); changed || err == nil {
			t.Fatal("partial profile was accepted", changed, err)
		}
		if s.Last.evaluation() != original {
			t.Fatal("partial profile zeroed published statistics")
		}
	}
	reset := valid
	reset.Stops[0]--
	other := valid
	other.Ident[0] = "Another driver"
	for _, bad := range []driverRecord{reset, other} {
		writeSaved(t, s, bad)
		_, _ = s.poll()
		if changed, err := s.poll(); changed || err == nil {
			t.Fatal("reset or different identity accepted")
		}
		if s.Last.evaluation() != original {
			t.Fatal("last good statistics changed")
		}
	}
}

func TestNativeFileConflictDoesNotOverwriteExternalChange(t *testing.T) {
	s := newSync(t)
	external := s.Last
	external.Hectom++
	b := external.encode(false)
	if err := os.WriteFile(s.NativePath, b, 0644); err != nil {
		t.Fatal(err)
	}
	saved := s.Last
	saved.Stops[0]++
	writeSaved(t, s, saved)
	_, _ = s.poll()
	if changed, err := s.poll(); !changed || err == nil || !s.NativeConflicted {
		t.Fatal("conflict should preserve real memory but suspend file mirroring", changed, err)
	}
	got, err := os.ReadFile(s.NativePath)
	if err != nil || !bytes.Equal(got, b) {
		t.Fatal("overwrote external BCS update")
	}
	if s.Last.Stops[0] != saved.Stops[0] {
		t.Fatal("conflict discarded saved trip")
	}
}

func TestTemporarilyBlockedOutputIsRetriedWithoutNewF9(t *testing.T) {
	s := newSync(t)
	path := s.SnapshotPath
	s.SnapshotPath = filepath.Join(filepath.Dir(path), "missing-dir", "record.odr")
	saved := s.Last
	saved.Stops[0]++
	writeSaved(t, s, saved)
	_, _ = s.poll()
	if changed, err := s.poll(); !changed || err == nil || !s.Pending {
		t.Fatal("missing directory failure not retained for retry", changed, err)
	}
	s.SnapshotPath = path
	if changed, err := s.poll(); changed || err != nil || s.Pending {
		t.Fatal("did not retry the saved data", changed, err)
	}
	b, err := os.ReadFile(s.NativePath)
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseDriver(b, false)
	if err != nil || d.Stops[0] != saved.Stops[0] {
		t.Fatal("retry did not update BCS profile")
	}
}

func TestSavedCollisionAndDrivingPenaltyReachBCSUnchanged(t *testing.T) {
	s := newSync(t)
	saved := s.Last
	saved.Crashes[0] += 2
	saved.Crashes[1]++
	saved.Crashes[3]++
	saved.Rating[4] = 0.625
	writeSaved(t, s, saved)
	_, _ = s.poll()
	if changed, err := s.poll(); !changed || err != nil {
		t.Fatal("saved collision and penalty were not published", changed, err)
	}
	memory := s.Last.memoryBlock()
	for i, want := range saved.Crashes {
		if got := int32(binary.LittleEndian.Uint32(memory[48+4*i:])); got != want {
			t.Fatalf("collision counter %d: BCS memory %d, simulator %d", i, got, want)
		}
	}
	if got := math.Float64frombits(binary.LittleEndian.Uint64(memory[64:])); got != saved.Rating[4] {
		t.Fatalf("BCS driving penalty %g, simulator %g", got, saved.Rating[4])
	}
	nativeBytes, err := os.ReadFile(s.NativePath)
	if err != nil {
		t.Fatal(err)
	}
	native, err := parseDriver(nativeBytes, false)
	if err != nil || native.Crashes != saved.Crashes || native.Rating[4] != saved.Rating[4] {
		t.Fatal("native personnel fallback differs from actual simulator data", native, err)
	}
	state, err := os.ReadFile(s.StatePath)
	if err != nil || !bytes.Contains(state, []byte("Saved driving penalty: 0.625000000")) || !bytes.Contains(state, []byte("Red-light offences: unavailable")) {
		t.Fatal("penalty diagnostics omit actual values or red-light limitation", err)
	}
}

func TestSavedDrivingPenaltyCanRecoverWithoutInventingEvents(t *testing.T) {
	s := newSync(t)
	saved := s.Last
	saved.Rating[4] = 0.8
	writeSaved(t, s, saved)
	_, _ = s.poll()
	if _, err := s.poll(); err != nil {
		t.Fatal(err)
	}
	// openOMSI decreases P with distance driven. P is a proportion, not a
	// monotonically increasing offence counter or a red-light event.
	recovered := saved
	recovered.Hectom += 20
	recovered.Rating[4] = 0.7
	writeSaved(t, s, recovered)
	_, _ = s.poll()
	if changed, err := s.poll(); !changed || err != nil {
		t.Fatal("real driving-penalty recovery was rejected", changed, err)
	}
	if s.Last.Crashes != saved.Crashes || s.Last.Rating[4] != recovered.Rating[4] {
		t.Fatal("penalty recovery changed collision counters or its real value")
	}
}
