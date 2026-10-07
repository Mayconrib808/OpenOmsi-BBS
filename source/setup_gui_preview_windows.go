//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"
)

// Capture the real native controls for CI review. This is called only by
// --gui-smoke with an explicit output directory; normal Setup captures nothing.
func captureSetupPreview(window uintptr, path string) error {
	var rect guiRect
	guiUser.NewProc("GetWindowRect").Call(window, uintptr(unsafe.Pointer(&rect)))
	w, h := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
		return fmt.Errorf("invalid GUI preview size")
	}
	dc, _, _ := guiUser.NewProc("GetWindowDC").Call(window)
	defer guiUser.NewProc("ReleaseDC").Call(window, dc)
	memory, _, _ := guiGDI.NewProc("CreateCompatibleDC").Call(dc)
	defer guiGDI.NewProc("DeleteDC").Call(memory)
	bitmap, _, _ := guiGDI.NewProc("CreateCompatibleBitmap").Call(dc, uintptr(w), uintptr(h))
	if dc == 0 || memory == 0 || bitmap == 0 {
		return fmt.Errorf("could not allocate GUI preview")
	}
	defer guiGDI.NewProc("DeleteObject").Call(bitmap)
	previous, _, _ := guiGDI.NewProc("SelectObject").Call(memory, bitmap)
	ok, _, _ := guiUser.NewProc("PrintWindow").Call(window, memory, 2)
	guiGDI.NewProc("SelectObject").Call(memory, previous)
	if ok == 0 {
		return fmt.Errorf("PrintWindow could not capture Setup")
	}
	header := struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Colors, Important      uint32
	}{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, Bits: 32}
	pixels := make([]byte, w*h*4)
	rows, _, _ := guiGDI.NewProc("GetDIBits").Call(dc, bitmap, 0, uintptr(h), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0)
	if rows != uintptr(h) {
		return fmt.Errorf("incomplete GUI preview bitmap")
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2], pixels[i+3] = pixels[i+2], pixels[i], 255
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)})
	closed := f.Close()
	if err != nil {
		return err
	}
	return closed
}
