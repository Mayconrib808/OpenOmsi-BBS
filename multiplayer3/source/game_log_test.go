package main

import (
	"bytes"
	"io"
	"testing"
)

func TestRealSpawnOutputAlsoReachesBCSGameLog(t *testing.T) {
	var diagnostics, bcs bytes.Buffer
	out := gameLogOutput(&diagnostics, &bcs)
	partial := "[2026-10-12T04:07:06Z INFO openomsi_game::spawn] spawned "
	io.WriteString(out, partial)
	if bytes.Contains(bcs.Bytes(), []byte("] spawned at")) {
		t.Fatal("premature readiness")
	}
	io.WriteString(out, "at entry point 7\n")
	if !bytes.Equal(diagnostics.Bytes(), bcs.Bytes()) || !bytes.Contains(bcs.Bytes(), []byte("] spawned at")) {
		t.Fatal("BCS cannot observe real spawn")
	}
}
