package main

import (
	"bytes"
	"encoding/binary"
)

func decodeText(b []byte) string {
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
		u := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[i:i+2]))
		}
		return string(runesFromUTF16(u))
	}
	if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
		u := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, binary.BigEndian.Uint16(b[i:i+2]))
		}
		return string(runesFromUTF16(u))
	}
	return string(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}))
}

func runesFromUTF16(u []uint16) []rune {
	out := make([]rune, 0, len(u))
	for i := 0; i < len(u); i++ {
		r := rune(u[i])
		if r >= 0xD800 && r <= 0xDBFF && i+1 < len(u) {
			r2 := rune(u[i+1])
			if r2 >= 0xDC00 && r2 <= 0xDFFF {
				out = append(out, 0x10000+((r-0xD800)<<10)+(r2-0xDC00))
				i++
				continue
			}
		}
		out = append(out, r)
	}
	return out
}
