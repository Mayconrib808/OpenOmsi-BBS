package main

import "encoding/binary"

// OmsiSpeicher.java of the supplied BCS 5.0.0.1 reads the Alt-menu pointer
// at this RVA, searches its fifteen button pointers (offset 940), and reads
// button id/enabled/rectangle at 468/476/72. Provide the schedule button so
// a second configuration attempt can run the same dialog handshake.
const omsiMenuPointerRVA = 4592464 // 0x461350
const scheduleButtonID = 5
const menuButtonsOffset = 940
const buttonIDOffset = 468
const buttonEnabledOffset = 476
const buttonRectOffset = 72

var scheduleButtonRect = [4]int32{120, 24, 72, 32}

func menuMemoryBlocks(buttonAddress uint32) ([1024]byte, [512]byte) {
	var menu [1024]byte
	var button [512]byte
	binary.LittleEndian.PutUint32(menu[menuButtonsOffset:], buttonAddress)
	binary.LittleEndian.PutUint32(button[buttonIDOffset:], scheduleButtonID)
	button[buttonEnabledOffset] = 1
	for i, value := range scheduleButtonRect {
		binary.LittleEndian.PutUint32(button[buttonRectOffset+i*4:], uint32(value))
	}
	return menu, button
}

func isScheduleButtonClick(packed uintptr) bool {
	x, y := int32(int16(packed&0xffff)), int32(int16((packed>>16)&0xffff))
	r := scheduleButtonRect
	return x >= r[0] && y >= r[1] && x < r[0]+r[2] && y < r[1]+r[3]
}
