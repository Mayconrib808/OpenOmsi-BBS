package main

// Driver translation for openOMSI 0.1.1740 (3df2f99) and BCS 5.0.0.1.
// The two .odr formats have the same counters but different [rating] orders.
// BCS receives the actual saved simulator counters; it owns the remuneration.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

type driverRecord struct {
	Ident   [4]string
	Stops   [3]int32 // total, late, early, in FILE order
	Hectom  int32
	Crashes [4]int32
	Tickets int32
	Money   float64
	Rating  [5]float64 // native: total, content, ticket denominator, numerator, P
	PerBus  []string
}

func driverText(b []byte) (string, error) {
	if len(b) >= 2 && ((b[0] == 0xff && b[1] == 0xfe) || (b[0] == 0xfe && b[1] == 0xff)) {
		if len(b)%2 != 0 {
			return "", fmt.Errorf("incomplete UTF-16 personnel file")
		}
		var order binary.ByteOrder = binary.LittleEndian
		if b[0] == 0xfe {
			order = binary.BigEndian
		}
		u := make([]uint16, (len(b)-2)/2)
		for i := range u {
			u[i] = order.Uint16(b[2+2*i : 4+2*i])
		}
		for i := 0; i < len(u); i++ {
			if u[i] >= 0xd800 && u[i] <= 0xdbff {
				if i+1 >= len(u) || u[i+1] < 0xdc00 || u[i+1] > 0xdfff {
					return "", fmt.Errorf("incomplete UTF-16 surrogate")
				}
				i++
			} else if u[i] >= 0xdc00 && u[i] <= 0xdfff {
				return "", fmt.Errorf("invalid UTF-16 surrogate")
			}
		}
		return string(utf16.Decode(u)), nil
	}
	return string(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})), nil
}

func parseDriver(b []byte, openFormat bool) (driverRecord, error) {
	var d driverRecord
	t, err := driverText(b)
	if err != nil {
		return d, err
	}
	blocks := map[string][]string{}
	key := ""
	for _, raw := range strings.Split(strings.ReplaceAll(t, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			key = strings.ToLower(strings.Trim(line, "[]"))
			if _, exists := blocks[key]; exists {
				return d, fmt.Errorf("duplicate [%s]", key)
			}
			blocks[key] = nil
		} else if key != "" && line != "" {
			blocks[key] = append(blocks[key], line)
		}
	}
	counts := map[string]int{"ident": 4, "busstops": 3, "hektom": 1, "crashs": 4, "tickets": 2, "rating": 5}
	for k, count := range counts {
		if len(blocks[k]) != count {
			return d, fmt.Errorf("[%s]: expected %d fields, got %d", k, count, len(blocks[k]))
		}
	}
	copy(d.Ident[:], blocks["ident"])
	if d.Ident[0] == "" {
		return d, fmt.Errorf("empty driver name")
	}
	for _, date := range d.Ident[2:] {
		if _, err := strconv.ParseInt(date, 10, 32); err != nil {
			return d, fmt.Errorf("invalid driver date: %w", err)
		}
	}
	integer := func(k string, i int) (int32, error) {
		v, err := strconv.ParseFloat(strings.ReplaceAll(blocks[k][i], ",", "."), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > math.MaxInt32 || v != math.Trunc(v) {
			return 0, fmt.Errorf("invalid [%s] integer %q", k, blocks[k][i])
		}
		return int32(v), nil
	}
	for i := range d.Stops {
		if d.Stops[i], err = integer("busstops", i); err != nil {
			return d, err
		}
	}
	if d.Hectom, err = integer("hektom", 0); err != nil {
		return d, err
	}
	for i := range d.Crashes {
		if d.Crashes[i], err = integer("crashs", i); err != nil {
			return d, err
		}
	}
	if d.Tickets, err = integer("tickets", 0); err != nil {
		return d, err
	}
	d.Money, err = strconv.ParseFloat(strings.ReplaceAll(blocks["tickets"][1], ",", "."), 64)
	if err != nil || math.IsNaN(d.Money) || math.IsInf(d.Money, 0) || math.Abs(d.Money) > math.MaxFloat32 {
		return d, fmt.Errorf("invalid ticket money")
	}
	for i := range d.Rating {
		d.Rating[i], err = strconv.ParseFloat(strings.ReplaceAll(blocks["rating"][i], ",", "."), 64)
		if err != nil || math.IsNaN(d.Rating[i]) || math.IsInf(d.Rating[i], 0) {
			return d, fmt.Errorf("invalid rating field %d", i)
		}
	}
	if openFormat {
		// openOMSI: P, content, requests, points (0/1/2), total.
		// BCS's memory AND file readers use numerator / denominator without /2.
		r := d.Rating
		d.Rating = [5]float64{r[4], r[1], 2 * r[2], r[3], r[0]}
	}
	if err = d.validate(); err != nil {
		return d, err
	}
	d.PerBus = append([]string(nil), blocks["perbusinfo"]...)
	return d, nil
}

func (d driverRecord) validate() error {
	if d.Stops[0] > 1000000 || d.Stops[1] > d.Stops[0] || d.Stops[2] > d.Stops[0] {
		return fmt.Errorf("invalid stop counters for BCS")
	}
	for i := 0; i < 4; i++ {
		v := d.Rating[i]
		if v < 0 || v > math.MaxInt32 || v != math.Trunc(v) {
			return fmt.Errorf("invalid rating counter %d", i)
		}
	}
	if d.Rating[1] > d.Rating[0] || d.Rating[3] > d.Rating[2] || d.Rating[4] < 0 || d.Rating[4] > 1 {
		return fmt.Errorf("invalid rating proportions")
	}
	return nil
}

func (d driverRecord) encode(openFormat bool) []byte {
	var t strings.Builder
	creator := "BCS Bridge v" + bridgeVersion + " by " + bridgeAuthor + " - OMSI format"
	if openFormat {
		creator = "BCS Bridge v" + bridgeVersion + " by " + bridgeAuthor + " - openOMSI 0.1.1740 format"
	}
	fmt.Fprintf(&t, "Driver File\r\nCreated by %s\r\n\r\n[ident]\r\n%s\r\n%s\r\n%s\r\n%s\r\n\r\n", creator, d.Ident[0], d.Ident[1], d.Ident[2], d.Ident[3])
	fmt.Fprintf(&t, "[busstops]\r\n%d\r\n%d\r\n%d\r\n\r\n[hektom]\r\n%d\r\n\r\n", d.Stops[0], d.Stops[1], d.Stops[2], d.Hectom)
	fmt.Fprintf(&t, "[crashs]\r\n%d\r\n%d\r\n%d\r\n%d\r\n\r\n[tickets]\r\n%d\r\n%.9f\r\n\r\n[rating]\r\n", d.Crashes[0], d.Crashes[1], d.Crashes[2], d.Crashes[3], d.Tickets, d.Money)
	r := d.Rating
	if openFormat {
		r = [5]float64{r[4], r[1], r[2] / 2, r[3], r[0]}
	}
	for _, v := range r {
		fmt.Fprintf(&t, "%.9f\r\n", v)
	}
	if len(d.PerBus) > 0 {
		t.WriteString("\r\n[perbusinfo]\r\n" + strings.Join(d.PerBus, "\r\n") + "\r\n")
	}
	u := utf16.Encode([]rune(t.String()))
	b := make([]byte, 2+len(u)*2)
	b[0], b[1] = 0xff, 0xfe
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[2+2*i:], v)
	}
	return b
}

func (d driverRecord) memoryBlock() [96]byte {
	var b [96]byte
	put := func(offset int, v int32) { binary.LittleEndian.PutUint32(b[offset:], uint32(v)) }
	for i, v := range d.Stops {
		put(32+4*i, v)
	}
	put(44, d.Hectom)
	for i, v := range d.Crashes {
		put(48+4*i, v)
	}
	binary.LittleEndian.PutUint64(b[64:], math.Float64bits(d.Rating[4]))
	put(72, int32(d.Rating[1])) // comfort numerator: satisfied passengers
	put(76, int32(d.Rating[2])) // ticket denominator: twice openOMSI requests
	put(80, int32(d.Rating[3])) // ticket numerator: openOMSI points
	put(84, int32(d.Rating[0])) // comfort denominator: total passengers
	put(88, d.Tickets)
	binary.LittleEndian.PutUint32(b[92:], math.Float32bits(float32(d.Money)))
	return b
}

func (d driverRecord) evaluation() string {
	b := d.memoryBlock()
	var parts []string
	for _, off := range []int{32, 36, 40, 44, 48, 52, 56, 60, 88} {
		parts = append(parts, strconv.Itoa(int(int32(binary.LittleEndian.Uint32(b[off:])))))
	}
	parts = append(parts, fmt.Sprintf("%.6f", float64(math.Float32frombits(binary.LittleEndian.Uint32(b[92:])))))
	for _, off := range []int{72, 84, 80, 76} {
		parts = append(parts, strconv.Itoa(int(int32(binary.LittleEndian.Uint32(b[off:])))))
	}
	return strings.Join(append(parts, fmt.Sprintf("%.6f", d.Rating[4])), "####")
}

func driverAdvanced(current, previous driverRecord) error {
	if current.Ident != previous.Ident {
		return fmt.Errorf("driver identity changed; refusing another driver's data")
	}
	for i, v := range current.Stops {
		if v < previous.Stops[i] {
			return fmt.Errorf("stop counter moved backwards")
		}
	}
	if current.Hectom < previous.Hectom || current.Tickets < previous.Tickets {
		return fmt.Errorf("distance or ticket counter moved backwards")
	}
	for i, v := range current.Crashes {
		if v < previous.Crashes[i] {
			return fmt.Errorf("crash counter moved backwards")
		}
	}
	for i := 0; i < 4; i++ {
		if current.Rating[i] < previous.Rating[i] {
			return fmt.Errorf("rating counter moved backwards")
		}
	}
	return nil
}

// Write to a temporary file first. On Windows os.Rename replaces the destination
// with MoveFileExW; a reader gets a complete old or new file, never a truncation.
func writeDriverAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".bcs-driver-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

type driverSync struct {
	OpenPath, NativePath, SnapshotPath, StatePath string
	Last                                          driverRecord
	NativeHash                                    [32]byte
	NativeConflicted                              bool
	Pending                                       bool
	LastBytes, Candidate                          []byte
}

func prepareDriver(nativePath, dir string) (*driverSync, error) {
	b, err := os.ReadFile(nativePath)
	if err != nil {
		return nil, fmt.Errorf("BCS personnel file %s: %w", nativePath, err)
	}
	d, err := parseDriver(b, false)
	if err != nil {
		return nil, fmt.Errorf("BCS personnel file: %w", err)
	}
	if d.Ident[0] != "Bus Company Simulator" {
		return nil, fmt.Errorf("expected the BCS driver, got %q", d.Ident[0])
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	if err = writeDriverAtomic(filepath.Join(dir, "bcs-driver-before-v1.1.2.odr"), b); err != nil {
		return nil, err
	}
	s := &driverSync{OpenPath: filepath.Join(dir, "bcs-driver-openomsi-v1.1.2.odr"), NativePath: nativePath, SnapshotPath: filepath.Join(dir, "bcs-driver-current-v1.1.2.odr"), StatePath: filepath.Join(dir, "driver-state-v1.1.2.txt"), Last: d, NativeHash: sha256.Sum256(b)}
	s.LastBytes = d.encode(true)
	if err = writeDriverAtomic(s.OpenPath, s.LastBytes); err != nil {
		return nil, err
	}
	if err = writeDriverAtomic(s.SnapshotPath, d.encode(false)); err != nil {
		return nil, err
	}
	if err = s.writeState("BASELINE_READY: press F9 in openOMSI to publish the current data."); err != nil {
		return nil, err
	}
	return s, nil
}

func loadDriverSync(openPath, nativePath, dir string) (*driverSync, error) {
	b, err := os.ReadFile(openPath)
	if err != nil {
		return nil, err
	}
	d, err := parseDriver(b, true)
	if err != nil {
		return nil, err
	}
	n, err := os.ReadFile(nativePath)
	if err != nil {
		return nil, err
	}
	nd, err := parseDriver(n, false)
	if err != nil {
		return nil, err
	}
	if nd.evaluation() != d.evaluation() || nd.Ident != d.Ident {
		return nil, fmt.Errorf("BCS baseline changed between preparation and facade startup")
	}
	return &driverSync{OpenPath: openPath, NativePath: nativePath, SnapshotPath: filepath.Join(dir, "bcs-driver-current-v1.1.2.odr"), StatePath: filepath.Join(dir, "driver-state-v1.1.2.txt"), Last: d, NativeHash: sha256.Sum256(n), LastBytes: b}, nil
}

func (s *driverSync) writeState(status string) error {
	t := fmt.Sprintf("BCS Bridge v%s\r\n%s\r\n\r\nopenOMSI driver: %s\r\nBCS driver: %s\r\nSaved stops: %d (late %d, early %d)\r\nSaved distance: %d hectometres\r\nSaved tickets: %d / %.6f\r\nBCS Auswertungsdaten expected:\r\n%s\r\n", bridgeVersion, status, s.OpenPath, s.NativePath, s.Last.Stops[0], s.Last.Stops[1], s.Last.Stops[2], s.Last.Hectom, s.Last.Tickets, s.Last.Money, s.Last.evaluation())
	return writeDriverAtomic(s.StatePath, []byte(t))
}

// Require two identical complete reads. Retain the last valid record on a
// transient write, malformed profile, reset counter, or changed identity.
func (s *driverSync) poll() (bool, error) {
	b, err := os.ReadFile(s.OpenPath)
	if err != nil {
		return false, err
	}
	if bytes.Equal(b, s.LastBytes) {
		s.Candidate = nil
		if s.Pending {
			return false, s.flush()
		}
		return false, nil
	}
	if !bytes.Equal(b, s.Candidate) {
		s.Candidate = append([]byte(nil), b...)
		return false, nil
	}
	d, err := parseDriver(b, true)
	if err != nil {
		return false, err
	}
	if err = driverAdvanced(d, s.Last); err != nil {
		return false, err
	}
	s.Last = d
	s.LastBytes = append([]byte(nil), b...)
	s.Candidate = nil
	s.Pending = true
	return true, s.flush()
}

func (s *driverSync) flush() error {
	if err := writeDriverAtomic(s.SnapshotPath, s.Last.encode(false)); err != nil {
		return err
	}
	var err error
	status := "SAVED: actual openOMSI data saved; awaiting BCS evaluation."
	if !s.NativeConflicted {
		current, e := os.ReadFile(s.NativePath)
		if e != nil {
			err = e
		} else if sha256.Sum256(current) != s.NativeHash {
			s.NativeConflicted = true
			err = fmt.Errorf("BCS changed the native personnel file; file mirroring suspended (memory data retained)")
		} else {
			out := s.Last.encode(false)
			if e = writeDriverAtomic(s.NativePath, out); e != nil {
				err = e
			} else {
				s.NativeHash = sha256.Sum256(out)
			}
		}
	}
	if s.NativeConflicted {
		status += "\r\nFILE_CONFLICT: native profile changed externally; memory and diagnostic copy updated."
	}
	if e := s.writeState(status); err == nil {
		err = e
	}
	if err == nil || s.NativeConflicted {
		s.Pending = false
	}
	return err
}
