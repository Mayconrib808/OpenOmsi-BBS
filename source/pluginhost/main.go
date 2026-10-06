// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Mayconrib808
// Host protocol adapted from openOMSI, Copyright (c) 2026 usonskyyyy.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

type plugin interface {
	start()
	flags() byte
	frame(frame) reply
	finalize()
	close()
}
type request struct {
	op  byte
	f   frame
	err error
}

// Pipe reading happens off the DLL thread so Windows messages still run while
// openOMSI is waiting, paused or between frames. DLL calls stay on one OS thread.
func serve(input io.Reader, output io.Writer, load func() (plugin, error), pump func() bool) error {
	requests := make(chan request)
	done := make(chan struct{})
	defer close(done)
	go func() {
		r := bufio.NewReader(input)
		for {
			x := request{}
			x.op, x.err = readByte(r)
			if x.err == nil && x.op == opFrame {
				// Bound allocations even if an incompatible parent sends corrupt counts.
				x.f, x.err = readFrame(&io.LimitedReader{R: r, N: 16 << 20})
			}
			select {
			case requests <- x:
			case <-done:
				return
			}
			if x.err != nil || x.op == opFinalize {
				return
			}
		}
	}()
	var lib plugin
	started := false
	defer func() {
		if lib != nil {
			if started {
				lib.finalize()
			}
			lib.close()
		}
	}()
	out := bufio.NewWriter(output)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if !pump() {
				return nil
			}
		case x := <-requests:
			if x.err != nil {
				if errors.Is(x.err, io.EOF) && x.op == 0 {
					return nil
				}
				return fmt.Errorf("incomplete host request: %w", x.err)
			}
			if !pump() {
				return nil
			}
			switch x.op {
			case opStart:
				if started {
					return fmt.Errorf("duplicate START request")
				}
				var err error
				lib, err = load()
				if err != nil {
					_, _ = out.Write([]byte{0, 0})
					_ = out.Flush()
					return err
				}
				lib.start()
				started = true
				if _, err = out.Write([]byte{1, lib.flags()}); err != nil {
					return err
				}
			case opFrame:
				if !started {
					return fmt.Errorf("FRAME before START")
				}
				if err := writeReply(out, lib.frame(x.f)); err != nil {
					return err
				}
			case opFinalize:
				if !started {
					return fmt.Errorf("FINALIZE before START")
				}
				lib.finalize()
				started = false
				if err := writeByte(out, 1); err != nil {
					return err
				}
				return out.Flush()
			default:
				return fmt.Errorf("unknown host opcode %d", x.op)
			}
			if err := out.Flush(); err != nil {
				return err
			}
		}
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: omsi-plugin-host32 <installed Plugins/bbs.dll>")
		os.Exit(2)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := serve(os.Stdin, os.Stdout, func() (plugin, error) {
		return loadPlugin(os.Args[1])
	}, pumpMessages)
	if err != nil {
		// stdout is exclusively the binary protocol; diagnostics use stderr.
		fmt.Fprintln(os.Stderr, "OpenOMSI BCS plugin host:", err)
		os.Exit(1)
	}
}
