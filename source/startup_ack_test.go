package main

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func encodedLog(s, encoding string) []byte {
	if encoding == "utf8" {
		return []byte(s)
	}
	var order binary.ByteOrder = binary.LittleEndian
	b := []byte{0xff, 0xfe}
	if encoding == "be" {
		order = binary.BigEndian
		b = []byte{0xfe, 0xff}
	}
	for _, v := range utf16.Encode([]rune(s)) {
		var pair [2]byte
		order.PutUint16(pair[:], v)
		b = append(b, pair[:]...)
	}
	return b
}

func TestBCSAcknowledgmentHandlesBothUTF16OrdersAndFastArrival(t *testing.T) {
	for _, encoding := range []string{"utf8", "le", "be"} {
		t.Run(encoding, func(t *testing.T) {
			baseline := encodedLog("INFO Schicht ID: 1001\r\n", encoding)
			cursor := newBCSAckCursor(baseline)
			if cursor.acknowledged(baseline) {
				t.Fatal("acknowledged before BCS responded")
			}
			// The response arrives between showing Tform_start and starting the watcher.
			response := encodedLog("INFO Schicht ID: 1001\r\nINFO 23:17:35:264 "+bcsStartAcknowledgment+", 500 ms nach dem Erscheinen\r\n", encoding)
			if !cursor.acknowledged(response) {
				t.Fatal("fast BCS acknowledgment lost")
			}
		})
	}
}

func TestBCSAcknowledgmentNeverReusesPreviousLaunchAfterRotation(t *testing.T) {
	old := "INFO 23:17:35:264 " + bcsStartAcknowledgment + "\r\n"
	cursor := newBCSAckCursor(encodedLog("old header\r\n"+old, "be"))
	if cursor.acknowledged(encodedLog("rotated header\r\n"+old, "be")) {
		t.Fatal("old acknowledgment replayed")
	}
	if !cursor.acknowledged(encodedLog("rotated header\r\n"+old+"INFO 23:19:35:265 "+bcsStartAcknowledgment+"\r\n", "be")) {
		t.Fatal("fresh acknowledgment ignored after rotation")
	}
}
