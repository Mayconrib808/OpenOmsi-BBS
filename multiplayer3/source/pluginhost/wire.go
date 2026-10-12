// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Mayconrib808
// Protocol adaptation from openOMSI, Copyright (c) 2026 usonskyyyy.
// See ../reference/OPENOMSI_LICENSE.txt for the original permission notice.
package main

import (
	"encoding/binary"
	"io"
	"math"
	"unicode/utf16"
)

const (
	opStart    = byte(1)
	opFrame    = byte(2)
	opFinalize = byte(3)
)

type floatEntry struct {
	index uint16
	value float32
}
type stringEntry struct {
	index uint16
	value string
}
type frame struct {
	system, variables []floatEntry
	strings           []stringEntry
	triggers          []uint16
}
type floatResult struct {
	write bool
	value float32
}
type stringResult struct {
	write bool
	value string
}
type reply struct {
	system, variables []floatResult
	strings           []stringResult
	triggers          []bool
}

func readByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}
func readU16(r io.Reader) (uint16, error) {
	var b [2]byte
	_, err := io.ReadFull(r, b[:])
	return binary.LittleEndian.Uint16(b[:]), err
}
func readFloat(r io.Reader) (float32, error) {
	var b [4]byte
	_, err := io.ReadFull(r, b[:])
	return math.Float32frombits(binary.LittleEndian.Uint32(b[:])), err
}
func readString(r io.Reader) (string, error) {
	n, err := readU16(r)
	if err != nil {
		return "", err
	}
	u := make([]uint16, int(n))
	for i := range u {
		if u[i], err = readU16(r); err != nil {
			return "", err
		}
	}
	return string(utf16.Decode(u)), nil
}
func readFloats(r io.Reader) ([]floatEntry, error) {
	n, err := readU16(r)
	if err != nil {
		return nil, err
	}
	list := make([]floatEntry, int(n))
	for i := range list {
		if list[i].index, err = readU16(r); err != nil {
			return nil, err
		}
		if list[i].value, err = readFloat(r); err != nil {
			return nil, err
		}
	}
	return list, nil
}
func readFrame(r io.Reader) (f frame, err error) {
	if f.system, err = readFloats(r); err != nil {
		return f, err
	}
	if f.variables, err = readFloats(r); err != nil {
		return f, err
	}
	n, err := readU16(r)
	if err != nil {
		return f, err
	}
	f.strings = make([]stringEntry, int(n))
	for i := range f.strings {
		if f.strings[i].index, err = readU16(r); err != nil {
			return f, err
		}
		if f.strings[i].value, err = readString(r); err != nil {
			return f, err
		}
	}
	if n, err = readU16(r); err != nil {
		return f, err
	}
	f.triggers = make([]uint16, int(n))
	for i := range f.triggers {
		if f.triggers[i], err = readU16(r); err != nil {
			return f, err
		}
	}
	return f, nil
}
func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}
func writeU16(w io.Writer, n uint16) error {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], n)
	_, err := w.Write(b[:])
	return err
}
func writeFloat(w io.Writer, v float32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	_, err := w.Write(b[:])
	return err
}
func writeString(w io.Writer, s string) error {
	u := utf16.Encode([]rune(s))
	if len(u) > 65535 {
		u = u[:65535] // same wire length limit as the pinned upstream implementation
	}
	if err := writeU16(w, uint16(len(u))); err != nil {
		return err
	}
	for _, c := range u {
		if err := writeU16(w, c); err != nil {
			return err
		}
	}
	return nil
}
func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
func writeReply(w io.Writer, r reply) error {
	for _, list := range [][]floatResult{r.system, r.variables} {
		for _, v := range list {
			if err := writeByte(w, boolByte(v.write)); err != nil {
				return err
			}
			value := float32(0)
			if v.write {
				value = v.value
			}
			if err := writeFloat(w, value); err != nil {
				return err
			}
		}
	}
	for _, s := range r.strings {
		if err := writeByte(w, boolByte(s.write)); err != nil {
			return err
		}
		value := ""
		if s.write {
			value = s.value
		}
		if err := writeString(w, value); err != nil {
			return err
		}
	}
	for _, active := range r.triggers {
		if err := writeByte(w, boolByte(active)); err != nil {
			return err
		}
	}
	return nil
}
