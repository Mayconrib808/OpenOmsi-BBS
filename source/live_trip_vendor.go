package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// BCS can replace its plugin (including the UDP port) for the next shift.
// Mirror only two consecutive, complete versions; openOMSI reloads the Lua
// plugin itself. Never rewrite PeDePe's source or its telemetry logic.
type liveVendorPlugin struct {
	source, destination string
	candidate           []byte
}

func (v *liveVendorPlugin) poll() error {
	f, e := os.Open(v.source)
	if e != nil {
		return nil
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if e != nil {
		return e
	}
	if len(b) == 0 || len(b) > 256<<10 || !utf8.Valid(b) {
		return nil
	}
	old, e := os.ReadFile(v.destination)
	if e != nil {
		return e
	}
	if bytes.Equal(b, old) {
		v.candidate = nil
		return nil
	}
	if !bytes.Equal(b, v.candidate) {
		v.candidate = append([]byte(nil), b...)
		return nil
	}
	if e = writeDriverAtomic(v.destination, b); e != nil {
		return fmt.Errorf("cannot stage the next BCS telemetry plugin: %w", e)
	}
	v.candidate = nil
	return nil
}
