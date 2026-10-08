package main

import (
	"bytes"
	"strings"
)

const bcsStartAcknowledgment = "OMSI Startmenue: Karte laedt"

// The cursor must be captured before publishing any legacy windows or markers.
// BCS writes both byte orders of UTF-16; a fast acknowledgment remains valid
// even when the polling goroutine starts later. Old lines survive log rotation
// without becoming acknowledgments for a new launch.
type bcsAckCursor struct {
	baseline []byte
	oldLines map[string]bool
}

func newBCSAckCursor(baseline []byte) bcsAckCursor {
	c := bcsAckCursor{baseline: append([]byte(nil), baseline...), oldLines: map[string]bool{}}
	for _, line := range strings.Split(decodeText(baseline), "\n") {
		if strings.Contains(line, bcsStartAcknowledgment) {
			c.oldLines[strings.TrimSpace(line)] = true
		}
	}
	return c
}

func (c bcsAckCursor) acknowledged(current []byte) bool {
	if bytes.HasPrefix(current, c.baseline) {
		return containsTextBytes(current[len(c.baseline):], bcsStartAcknowledgment)
	}
	for _, line := range strings.Split(decodeText(current), "\n") {
		if strings.Contains(line, bcsStartAcknowledgment) && !c.oldLines[strings.TrimSpace(line)] {
			return true
		}
	}
	return false
}

func containsTextBytes(b []byte, text string) bool {
	if bytes.Contains(b, []byte(text)) {
		return true
	}
	le, be := make([]byte, 0, len(text)*2), make([]byte, 0, len(text)*2)
	for i := 0; i < len(text); i++ {
		le = append(le, text[i], 0)
		be = append(be, 0, text[i])
	}
	return bytes.Contains(b, le) || bytes.Contains(b, be)
}
