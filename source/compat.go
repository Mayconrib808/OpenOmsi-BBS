//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	pOpenProcess         = kernel32.NewProc("OpenProcess")
	pWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	pCloseHandle         = kernel32.NewProc("CloseHandle")
	pVirtualAlloc        = kernel32.NewProc("VirtualAlloc")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pShowWindow               = user32.NewProc("ShowWindow")
	pPostMessageW             = user32.NewProc("PostMessageW")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pEnumWindows              = user32.NewProc("EnumWindows")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pSetWindowTextW           = user32.NewProc("SetWindowTextW")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
)

const (
	synchronize = 0x00100000
	infinite    = 0xFFFFFFFF

	wsPopup   = 0x80000000
	wsVisible = 0x10000000
	wsCaption = 0x00C00000
	wsSysMenu = 0x00080000
	wsChild   = 0x40000000

	wsExToolWindow = 0x00000080
	wsExNoActivate = 0x08000000

	swHide           = 0
	swShowNoActivate = 4

	swpNoActivate = 0x0010
	swpNoZOrder   = 0x0004

	wmNull          = 0x0000
	wmDestroy       = 0x0002
	wmShowWindow    = 0x0018
	wmClose         = 0x0010
	wmCommand       = 0x0111
	wmHScroll       = 0x0114
	wmKeyDown       = 0x0100
	wmKeyUp         = 0x0101
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmCompatAdvance = 0x8001 // private WM_APP message: run phase transition on GUI thread

	bmClick        = 245
	cbGetCount     = 326
	cbGetLBText    = 328
	cbGetLBTextLen = 329
	cbSetCurSel    = 334
	tbmGetPos      = 1024
	tbmGetRangeMax = 1026
	tbmSetPos      = 1029

	memCommit     = 0x1000
	memReserve    = 0x2000
	pageReadWrite = 0x04

	// Exact RVA used by this BCS build's OmsiSpeicher.java.
	omsiRootPointerRVA   = 4592896 // 0x461500
	omsiDriverPointerRVA = 4592888 // 0x4614F8
	offScheduleActive    = 1628
	offLine              = 1632
	offTour              = 1636
	offTrip              = 1640
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type comboState struct {
	items    []string
	selected int
}

// legacyRvaBacking deliberately enlarges the writable PE .data/BSS image.
// The BCS build reads a pointer at moduleBase + 0x461500, which in the
// original OMSI lives inside the image's writable .bss. Keeping this RVA
// inside our own image is stronger than trying to VirtualAlloc an arbitrary
// absolute address after process startup: ASLR may move the image, but the
// RVA moves with it and remains mapped/writable.
var legacyRvaBacking [4 << 20]byte

var (
	logMu       sync.Mutex
	legacyLogMu sync.Mutex
	logPath     string
	winNameMu   sync.RWMutex
	winNames    = map[uintptr]string{}
	comboMu     sync.Mutex
	combos      = map[uintptr]*comboState{}
	trackMu     sync.Mutex
	trackPos    int
	trackMax    = 255

	mainHwnd    uintptr
	startHwnd   uintptr
	setTTHwnd   uintptr
	okHwnd      uintptr
	cancelHwnd  uintptr
	trackHwnd   uintptr
	menuHwnd    uintptr
	confirmHwnd uintptr

	openOmsiPID     uint32
	omsiRoot        string
	mapRel          string
	wantedLine      string
	wantedTour      string
	bcsLogPath      string
	currentShiftID  string
	bcsLogStartSize int64
	omsiMem         uintptr
	driverSlot      uintptr
	driverPage      uintptr
	driverBank      int
	driverState     *driverSync
	markerOnce      sync.Once
	mapFinishOnce   sync.Once
	stateMu         sync.Mutex
	openOMSIReady   bool
	bcsStartMenuAck bool
)

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func logf(format string, a ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, time.Now().Format("2006-01-02 15:04:05.000 ")+format+"\r\n", a...)
}

func nameOf(hwnd uintptr) string {
	winNameMu.RLock()
	defer winNameMu.RUnlock()
	if n, ok := winNames[hwnd]; ok {
		return n
	}
	return fmt.Sprintf("HWND=0x%X", hwnd)
}

func putInt(addr uintptr, v int32)   { *(*int32)(unsafe.Pointer(addr)) = v }
func putUint(addr uintptr, v uint32) { *(*uint32)(unsafe.Pointer(addr)) = v }

func setScheduleActive(active bool) {
	if omsiMem == 0 {
		return
	}
	if active {
		putInt(omsiMem+offScheduleActive, 1)
	} else {
		putInt(omsiMem+offScheduleActive, 0)
	}
	logf("OmsiSpeicher shim: schedule active=%v", active)
}

func initOmsiMemoryShim() bool {
	// Keep the large backing object live and verify that the exact legacy RVA
	// falls inside it. The linker therefore maps moduleBase+0x461500 as part of
	// the executable itself, just like the original OMSI .bss.
	legacyRvaBacking[0] = 0
	hInst, _, _ := pGetModuleHandleW.Call(0)
	rootSlot := hInst + omsiRootPointerRVA
	menuSlot := hInst + omsiMenuPointerRVA
	driverSlot = hInst + omsiDriverPointerRVA
	backingStart := uintptr(unsafe.Pointer(&legacyRvaBacking[0]))
	backingEnd := backingStart + uintptr(len(legacyRvaBacking))
	if rootSlot < backingStart || rootSlot+4 > backingEnd || driverSlot < backingStart || driverSlot+4 > backingEnd || menuSlot < backingStart || menuSlot+4 > backingEnd {
		logf("ERROR OmsiSpeicher PE-backed RVA check: moduleBase=0x%X rootSlot=0x%X backing=[0x%X,0x%X)", hInst, rootSlot, backingStart, backingEnd)
		return false
	}

	block, _, e := pVirtualAlloc.Call(0, 4096, memCommit|memReserve, pageReadWrite)
	if block == 0 {
		logf("ERROR VirtualAlloc OmsiSpeicher block: %v", e)
		return false
	}
	omsiMem = block
	putInt(omsiMem+offScheduleActive, 0)
	putInt(omsiMem+offLine, 0)
	putInt(omsiMem+offTour, 0)
	putInt(omsiMem+offTrip, 0)

	putUint(rootSlot, uint32(omsiMem))
	menuPage, _, e := pVirtualAlloc.Call(0, 4096, memCommit|memReserve, pageReadWrite)
	buttonPage, _, e2 := pVirtualAlloc.Call(0, 4096, memCommit|memReserve, pageReadWrite)
	if menuPage == 0 || buttonPage == 0 {
		logf("ERROR menu memory allocation: %v / %v", e, e2)
		return false
	}
	menu, button := menuMemoryBlocks(uint32(buttonPage))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(menuPage)), len(menu)), menu[:])
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buttonPage)), len(button)), button[:])
	putUint(menuSlot, uint32(menuPage))
	logf("BCS timetable retry button ready: menu RVA=0x%X button id=%d rectangle=%v", omsiMenuPointerRVA, scheduleButtonID, scheduleButtonRect)
	logf("OmsiSpeicher shim ready (PE-backed RVA): moduleBase=0x%X rootSlot=0x%X backing=[0x%X,0x%X) -> block=0x%X", hInst, rootSlot, backingStart, backingEnd, omsiMem)
	driverPage, _, e = pVirtualAlloc.Call(0, 4096, memCommit|memReserve, pageReadWrite)
	if driverPage == 0 {
		logf("ERROR VirtualAlloc driver block: %v", e)
		return false
	}
	publishDriver(driverState.Last)
	logf("BCS driver pointer ready: RVA=0x%X slot=0x%X -> page=0x%X; real baseline stops=%d", omsiDriverPointerRVA, driverSlot, driverPage, driverState.Last.Stops[0])
	return true
}

func publishDriver(d driverRecord) {
	// Fill an inactive record, then publish its 32-bit pointer. Neither the
	// six-decimal double penalty nor any counter is exposed half-written.
	driverBank ^= 1
	addr := driverPage + uintptr(driverBank*128)
	b := d.memoryBlock()
	copy(unsafe.Slice((*byte)(unsafe.Pointer(addr)), len(b)), b[:])
	atomic.StoreUint32((*uint32)(unsafe.Pointer(driverSlot)), uint32(addr))
}

func watchDriver() {
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		lastError := ""
		for range t.C {
			freezePath := filepath.Join(filepath.Dir(logPath), "facade-evaluation-complete.flag")
			if evaluationFreezeRequested(freezePath, currentShiftID) {
				if err := driverState.freezeEvaluation(); err != nil {
					logf("WARN retaining completed shift driver state: %v", err)
				}
				logf("DRIVER_EVALUATION_COMPLETE: shift %s; real saved counters retained while BCS results/next-trip screen is open", currentShiftID)
				return
			}
			changed, err := driverState.poll()
			if changed {
				publishDriver(driverState.Last)
				logf("DRIVER_SAVED: stops=%d late=%d early=%d hectom=%d tickets=%d; expected Auswertungsdaten: %s", driverState.Last.Stops[0], driverState.Last.Stops[1], driverState.Last.Stops[2], driverState.Last.Hectom, driverState.Last.Tickets, driverState.Last.evaluation())
			}
			if err != nil {
				if err.Error() != lastError {
					logf("WARN driver sync: %v (last valid memory retained)", err)
					lastError = err.Error()
				}
			} else {
				lastError = ""
			}
		}
	}()
}

func legacyLogPath() string {
	if omsiRoot == "" {
		return ""
	}
	return filepath.Join(omsiRoot, "logfile.txt")
}

func appendLegacyLogLine(line string) {
	path := legacyLogPath()
	if path == "" {
		return
	}
	legacyLogMu.Lock()
	defer legacyLogMu.Unlock()

	// os.O_APPEND starts exactly at EOF. The previous implementation rewrote
	// logfile.txt without a trailing newline, so the next compatibility marker
	// could be glued to the Spline-Helper line. BCS reads this file line-by-line;
	// preserve real OMSI semantics by forcing a line boundary first.
	needSep := false
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		if rf, err := os.Open(path); err == nil {
			if _, err := rf.Seek(-1, 2); err == nil {
				one := []byte{0}
				if _, err := rf.Read(one); err == nil && one[0] != '\n' {
					needSep = true
				}
			}
			_ = rf.Close()
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logf("ERROR legacy logfile append: %v", err)
		return
	}
	defer f.Close()
	if needSep {
		_, _ = f.WriteString("\r\n")
	}
	_, _ = f.WriteString(line + "\r\n")
	logf("legacy logfile <= %s", line)
}

func mapFolderForLegacyLog() string {
	rel := strings.ReplaceAll(mapRel, "/", "\\")
	rel = strings.TrimSpace(rel)
	low := strings.ToLower(rel)
	if strings.HasPrefix(low, "maps\\") {
		rel = rel[len("maps\\"):]
	}
	low = strings.ToLower(rel)
	if strings.HasSuffix(low, "\\global.cfg") {
		rel = rel[:len(rel)-len("\\global.cfg")]
	}
	return strings.Trim(rel, "\\")
}

func prepareLegacyLogfile() {
	path := legacyLogPath()
	if path == "" {
		return
	}
	legacyLogMu.Lock()
	defer legacyLogMu.Unlock()
	b, _ := os.ReadFile(path)
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+2)
	removed := 0
	for _, line := range lines {
		// OMSI recreates its logfile on startup. openOMSI does not write the same
		// legacy markers, so stale entries from an older OMSI session confuse the
		// BCS 5.0.0.1 automation (it scans the entire file, not only new lines).
		if strings.Contains(line, "Loading Situation maps\\") ||
			strings.Contains(line, "Spline-Helper initialisieren") ||
			strings.Contains(line, "[OpenOMSI_BCS_Bridge") {
			removed++
			continue
		}
		out = append(out, line)
	}
	// Preserve all unrelated OMSI diagnostics, but remove stale automation
	// sentinels and publish the exact startup sentinel StartController waits for.
	out = append(out, "Spline-Helper initialisieren [OpenOMSI_BCS_Bridge v1.1.3]")
	if err := os.WriteFile(path, []byte(strings.Join(out, "\r\n")+"\r\n"), 0644); err != nil {
		logf("ERROR prepare legacy logfile: %v", err)
		return
	}
	logf("legacy logfile prepared: removed %d stale automation marker(s); injected Spline-Helper initialisieren", removed)
}

func closecheckPath() string {
	if omsiRoot == "" {
		return ""
	}
	return filepath.Join(omsiRoot, "closecheck")
}

func ensureBCSClosecheckGate() {
	path := closecheckPath()
	if path == "" {
		return
	}

	create := func(reason string) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		// StartController.java only checks File.exists() here. Keep contents
		// deliberately inert; this is a compatibility sentinel, not telemetry.
		if err := os.WriteFile(path, []byte("OpenOMSI_BCS_Bridge v1.1.3\r\n"), 0644); err != nil {
			logf("ERROR BCS closecheck compatibility gate: %v", err)
			return
		}
		logf("BCS startup gate satisfied: created closecheck sentinel (%s): %s", reason, path)
	}

	create("initial")
	go func() {
		deadline := time.Now().Add(startupTimeout)
		for time.Now().Before(deadline) {
			stateMu.Lock()
			acked := bcsStartMenuAck
			stateMu.Unlock()
			if acked {
				return
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				create("watchdog")
			}
			time.Sleep(250 * time.Millisecond)
		}
		logf("WARN closecheck watchdog timeout before BCS start-menu acknowledgment")
	}()
}

func ensureLegacyStartupMarkers() {
	path := legacyLogPath()
	if path == "" {
		return
	}
	folder := mapFolderForLegacyLog()
	if folder == "" {
		folder = "unknown"
	}
	startupNeedle := "Spline-Helper initialisieren"
	loadingNeedle := "Loading Situation maps\\" + folder + "\\global.cfg"
	loadingLine := loadingNeedle + " [OpenOMSI_BCS_Bridge v1.1.3]"

	go func() {
		deadline := time.Now().Add(startupTimeout)
		for time.Now().Before(deadline) {
			stateMu.Lock()
			acked := bcsStartMenuAck
			stateMu.Unlock()
			if acked {
				return
			}
			b, _ := os.ReadFile(path)
			text := string(b)
			if !strings.Contains(text, startupNeedle) {
				appendLegacyLogLine("Spline-Helper initialisieren [OpenOMSI_BCS_Bridge v1.1.3]")
				logf("startup watchdog: re-injected Spline-Helper sentinel after logfile replacement")
			}
			if !strings.Contains(text, loadingNeedle) {
				appendLegacyLogLine(loadingLine)
				logf("startup watchdog: re-injected map-loading marker after logfile replacement")
			}
			time.Sleep(250 * time.Millisecond)
		}
		logf("WARN startup marker watchdog timeout")
	}()
}

func signalMapLoadingStarted() {
	folder := mapFolderForLegacyLog()
	if folder == "" {
		folder = "unknown"
	}
	// OmsiStartmenue.java only checks for the substring "Loading Situation maps\\".
	// Do NOT use laststn.osn here: BCS has a second observer that treats that
	// suffix as "map finished loading".
	appendLegacyLogLine(fmt.Sprintf("Loading Situation maps\\%s\\global.cfg [OpenOMSI_BCS_Bridge v1.1.3]", folder))
}

func signalMapLoadingFinished() {
	folder := mapFolderForLegacyLog()
	if folder == "" {
		folder = "unknown"
	}
	if startHwnd != 0 {
		pShowWindow.Call(startHwnd, swHide)
		logf("Tform_start hidden: BCS acknowledged start menu AND openOMSI is ready")
	}
	// Only publish the map-finished marker after BOTH sides have reached the
	// same phase: openOMSI is actually ready and BCS has acknowledged its
	// synthetic start-menu result. v0.4.6.7 published this too early and hid
	// Tform_start before StartController necessarily began polling for it.
	if setTTHwnd != 0 {
		pShowWindow.Call(setTTHwnd, swShowNoActivate)
		logf("Tform_settt shown: start-menu handshake complete; BCS may configure timetable")
	}
	appendLegacyLogLine(fmt.Sprintf("Loading Situation maps\\%s\\laststn.osn...", folder))
	logf("state transition WAIT_BCS_START_ACK+WAIT_OPENOMSI_READY -> WAIT_TIMETABLE")
}

func tryAdvanceToTimetable() {
	stateMu.Lock()
	ready := openOMSIReady
	acked := bcsStartMenuAck
	stateMu.Unlock()
	if ready && acked {
		// IMPORTANT: this function is normally called by watcher goroutines.
		// ShowWindow on a window owned by the GUI thread can block when invoked
		// cross-thread. v0.4.6.9 proved both handshake booleans became true but
		// then stopped before the first ShowWindow returned. Marshal the actual
		// transition back to the window thread with a private WM_APP message.
		if mainHwnd != 0 {
			logf("handshake complete on worker; posting GUI-thread timetable transition")
			pPostMessageW.Call(mainHwnd, wmCompatAdvance, 0, 0)
			return
		}
		logf("WARN handshake complete but TForm_main hwnd is zero; using direct fallback")
		mapFinishOnce.Do(signalMapLoadingFinished)
	}
}

func markOpenOMSIReady() {
	stateMu.Lock()
	first := !openOMSIReady
	openOMSIReady = true
	stateMu.Unlock()
	if first {
		logf("handshake: openOMSI ready=true; waiting for BCS start-menu acknowledgment if not already seen")
	}
	tryAdvanceToTimetable()
}

func markBCSStartMenuAck(source string) {
	stateMu.Lock()
	first := !bcsStartMenuAck
	bcsStartMenuAck = true
	stateMu.Unlock()
	if first {
		logf("handshake: BCS start-menu acknowledged via %s", source)
	}
	tryAdvanceToTimetable()
}

func containsTextBytes(b []byte, text string) bool {
	if bytes.Contains(b, []byte(text)) {
		return true
	}
	// BCS log.txt is normally UTF-16LE. Search the ASCII phrase in that
	// representation too, without depending on the file's BOM or encoding.
	u16 := make([]byte, 0, len(text)*2)
	for i := 0; i < len(text); i++ {
		u16 = append(u16, text[i], 0)
	}
	return bytes.Contains(b, u16)
}

func watchBCSStartMenuAck() {
	if strings.TrimSpace(bcsLogPath) == "" {
		bcsLogPath = filepath.Join(omsiRoot, "Busbetrieb-Simulator", "log.txt")
	}
	if st, err := os.Stat(bcsLogPath); err == nil {
		bcsLogStartSize = st.Size()
	}
	logf("BCS start-menu watcher armed: path=%q offset=%d", bcsLogPath, bcsLogStartSize)
	go func() {
		deadline := time.Now().Add(2 * time.Minute)
		offset := bcsLogStartSize
		for time.Now().Before(deadline) {
			b, err := os.ReadFile(bcsLogPath)
			if err == nil {
				if int64(len(b)) < offset {
					offset = 0 // log was replaced/truncated
				}
				tail := b[offset:]
				if containsTextBytes(tail, "OMSI Startmenue: Karte laedt") {
					markBCSStartMenuAck("BCS log: OMSI Startmenue: Karte laedt")
					return
				}
				if containsTextBytes(tail, "OMSI Startmenue: Start nicht ausgeloest") {
					logf("WARN BCS reported start menu not triggered; keeping Tform_start visible and markers alive")
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		logf("WARN BCS start-menu acknowledgment timeout; Tform_start kept visible")
	}()
}

func waitForOpenOMSIReadyFlag() {
	readyFlag := filepath.Join(filepath.Dir(logPath), "openomsi-ready-v1.1.3.flag")
	go func() {
		deadline := time.Now().Add(startupTimeout)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(readyFlag); err == nil {
				logf("openOMSI readiness flag observed: %s", readyFlag)
				markOpenOMSIReady()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		logf("WARN openOMSI readiness flag timeout; keeping BCS in map-loading state")
	}()
}

func syntheticLoadingMarker() {
	markerOnce.Do(func() {
		// v1.1.3 publishes the observable result of the start-menu action
		// immediately. OmsiStartmenue checks logfile.txt for this marker before
		// it attempts any Win32 click, so startup no longer depends on synthetic
		// mouse routing. The actual "map ready" marker is still delayed until
		// openOMSI reports auto-start completion.
		signalMapLoadingStarted()
		logf("startup marker armed: map-loading result published; Tform_start will remain visible until BCS acknowledges Startmenue")
		waitForOpenOMSIReadyFlag()
	})
}

func copyWideString(dst uintptr, s string) uintptr {
	if dst == 0 {
		return 0
	}
	u, _ := syscall.UTF16FromString(s)
	out := unsafe.Slice((*uint16)(unsafe.Pointer(dst)), len(u))
	copy(out, u)
	if len(u) > 0 {
		return uintptr(len(u) - 1)
	}
	return 0
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	// BCS probes facade windows with SendMessageTimeout(WM_NULL). The Java/JNA
	// code checks the API call return value (success/non-timeout), not the
	// window-procedure result stored through lpdwResult. Returning 1 here is
	// harmless and keeps the facade explicitly responsive, but delivery itself
	// is the important part.
	if message == wmNull {
		logf("BCS responsiveness probe %s WM_NULL -> 1", nameOf(hwnd))
		return 1
	}
	if message == wmCompatAdvance {
		logf("GUI-thread timetable transition message received by %s", nameOf(hwnd))
		mapFinishOnce.Do(signalMapLoadingFinished)
		return 0
	}

	switch message {
	case wmLButtonDown, wmLButtonUp, wmCommand, wmHScroll, wmKeyDown, wmKeyUp, wmShowWindow, wmClose, bmClick,
		cbGetCount, cbGetLBText, cbGetLBTextLen, cbSetCurSel, tbmGetPos, tbmGetRangeMax, tbmSetPos:
		logf("message %s msg=0x%X w=0x%X l=0x%X", nameOf(hwnd), message, wParam, lParam)
	}

	if hwnd == menuHwnd && message == wmLButtonUp && isScheduleButtonClick(lParam) {
		stateMu.Lock()
		ready := openOMSIReady && bcsStartMenuAck
		stateMu.Unlock()
		if ready {
			active := *(*int32)(unsafe.Pointer(omsiMem + offScheduleActive)) != 0
			if active {
				pShowWindow.Call(confirmHwnd, swShowNoActivate)
				logf("BCS timetable retry: waiting for confirmation of previous facade schedule")
			} else {
				pShowWindow.Call(setTTHwnd, swShowNoActivate)
				logf("BCS timetable retry: dialog reopened")
			}
		}
		return 0
	}
	if hwnd == confirmHwnd && ((message == wmKeyUp && wParam == 13) || (message == wmCommand && wParam&0xffff == 1)) {
		setScheduleActive(false)
		pShowWindow.Call(confirmHwnd, swHide)
		pShowWindow.Call(setTTHwnd, swShowNoActivate)
		logf("BCS timetable retry: confirmed; same-trip dialog reopened")
		return 0
	}

	if hwnd == startHwnd {
		switch message {
		case wmLButtonDown:
			// BCS OmsiStartmenue.java posts WM_LBUTTONDOWN first. Keep the form
			// visible until the corresponding UP so the legacy click completes.
			logf("state=WAIT_START_CLICK; observed BCS WM_LBUTTONDOWN on Tform_start")
			return 0
		case wmLButtonUp, bmClick:
			logf("BCS start-menu click observed; trigger=0x%X", message)
			syntheticLoadingMarker() // idempotent; startup may already be armed
			markBCSStartMenuAck("Tform_start click")
			return 0
		}
	}

	comboMu.Lock()
	cs := combos[hwnd]
	comboMu.Unlock()
	if cs != nil {
		switch message {
		case cbGetCount:
			return uintptr(len(cs.items))
		case cbGetLBTextLen:
			i := int(wParam)
			if i < 0 || i >= len(cs.items) {
				return ^uintptr(0)
			}
			u, _ := syscall.UTF16FromString(cs.items[i])
			return uintptr(len(u) - 1)
		case cbGetLBText:
			i := int(wParam)
			if i < 0 || i >= len(cs.items) {
				return ^uintptr(0)
			}
			return copyWideString(lParam, cs.items[i])
		case cbSetCurSel:
			i := int(wParam)
			if i < 0 || i >= len(cs.items) {
				return ^uintptr(0)
			}
			comboMu.Lock()
			cs.selected = i
			comboMu.Unlock()
			return uintptr(i)
		}
	}

	if hwnd == trackHwnd {
		switch message {
		case tbmGetRangeMax:
			return uintptr(trackMax)
		case tbmSetPos:
			trackMu.Lock()
			trackPos = int(lParam)
			trackMu.Unlock()
			return 0
		case tbmGetPos:
			trackMu.Lock()
			p := trackPos
			trackMu.Unlock()
			return uintptr(p)
		}
	}

	if hwnd == okHwnd && (message == bmClick || message == wmLButtonUp) {
		// OmsiFahrplan.java posts BM_CLICK (245) to the OK TButton. Only now
		// expose the active Fahrplan in the OmsiSpeicher shim and close dialog.
		setScheduleActive(true)
		if setTTHwnd != 0 {
			pShowWindow.Call(setTTHwnd, swHide)
		}
		logf("state transition WAIT_TIMETABLE -> SCHEDULE_ACTIVE; BCS clicked timetable OK; dialog hidden and schedule flag set")
		return 0
	}
	if hwnd == cancelHwnd && (message == bmClick || message == wmLButtonUp) {
		setScheduleActive(false)
		if setTTHwnd != 0 {
			pShowWindow.Call(setTTHwnd, swHide)
		}
		logf("state transition WAIT_TIMETABLE -> CANCELLED; BCS clicked timetable Cancel")
		return 0
	}
	// Native Delphi buttons also notify the parent with WM_COMMAND/BN_CLICKED.
	// Accept that form as a fallback without advancing on any unrelated command.
	if hwnd == setTTHwnd && message == wmCommand {
		id := int(wParam & 0xFFFF)
		if id == 1 {
			setScheduleActive(true)
			pShowWindow.Call(setTTHwnd, swHide)
			logf("state transition WAIT_TIMETABLE -> SCHEDULE_ACTIVE; parent WM_COMMAND id=1")
			return 0
		}
		if id == 2 {
			setScheduleActive(false)
			pShowWindow.Call(setTTHwnd, swHide)
			logf("state transition WAIT_TIMETABLE -> CANCELLED; parent WM_COMMAND id=2")
			return 0
		}
	}

	// Ignore close requests: facade lives until openOMSI exits.
	if message == wmClose || message == wmDestroy {
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func registerClass(class string, hInst uintptr, cb uintptr) error {
	cn := u16(class)
	wc := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: cb, hInstance: hInst, lpszClassName: cn}
	r, _, e := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno == 1410 {
			return nil
		}
		return fmt.Errorf("RegisterClassExW %s: %v", class, e)
	}
	return nil
}

func coord(v int32) uintptr { return uintptr(uint32(v)) }

func createFake(class, title string, visible bool, owner, hInst, cb uintptr) uintptr {
	if err := registerClass(class, hInst, cb); err != nil {
		logf("ERROR %v", err)
		return 0
	}
	style := uintptr(wsPopup | wsCaption | wsSysMenu)
	if visible {
		style |= wsVisible
	}
	ex := uintptr(wsExToolWindow | wsExNoActivate)
	hwnd, _, e := pCreateWindowExW.Call(ex, uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(title))), style,
		coord(-30000), coord(-30000), 640, 480, owner, 0, hInst, 0)
	if hwnd == 0 {
		logf("ERROR CreateWindowEx %s/%s: %v", class, title, e)
		return 0
	}
	winNameMu.Lock()
	winNames[hwnd] = fmt.Sprintf("%s [%s]", class, title)
	winNameMu.Unlock()
	pSetWindowPos.Call(hwnd, 0, coord(-30000), coord(-30000), 640, 480, swpNoActivate|swpNoZOrder)
	if visible {
		pShowWindow.Call(hwnd, swShowNoActivate)
	} else {
		pShowWindow.Call(hwnd, swHide)
	}
	logf("created %s title=%q hwnd=0x%X visible=%v owner=0x%X", class, title, hwnd, visible, owner)
	return hwnd
}

func createChild(class, title string, parent, hInst, cb uintptr, id int, x, y, w, h int32) uintptr {
	if err := registerClass(class, hInst, cb); err != nil {
		logf("ERROR %v", err)
		return 0
	}
	style := uintptr(wsChild | wsVisible)
	hwnd, _, e := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(title))), style,
		coord(x), coord(y), uintptr(w), uintptr(h), parent, uintptr(id), hInst, 0)
	if hwnd == 0 {
		logf("ERROR child %s: %v", class, e)
		return 0
	}
	winNameMu.Lock()
	winNames[hwnd] = fmt.Sprintf("%s#%d [%s]", class, id, title)
	winNameMu.Unlock()
	logf("created child %s id=%d x=%d y=%d hwnd=0x%X", class, id, x, y, hwnd)
	return hwnd
}

func addCombo(hwnd uintptr, items []string) {
	comboMu.Lock()
	defer comboMu.Unlock()
	combos[hwnd] = &comboState{items: items, selected: -1}
}

func watchOpenOMSIPIDFile(dir string) {
	pidPath := filepath.Join(dir, "openomsi.pid")
	go func() {
		for i := 0; i < 600; i++ { // up to ~60 s for openOMSI to be spawned
			b, err := os.ReadFile(pidPath)
			if err == nil {
				if v, e := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 32); e == nil && v != 0 {
					atomic.StoreUint32(&openOmsiPID, uint32(v))
					logf("openOMSI PID rendezvous: %d", atomic.LoadUint32(&openOmsiPID))
					retitleOpenOMSI()
					watchSimulatorExit(uint32(v))
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		logf("WARN openOMSI PID rendezvous timeout")
		os.Exit(1) // Do not leave a phantom Omsi.exe/window for the next BCS launch.
	}()
}

var simulatorWatchOnce sync.Once

func watchSimulatorExit(pid uint32) {
	simulatorWatchOnce.Do(func() {
		h, _, e := pOpenProcess.Call(synchronize, 0, uintptr(pid))
		if h == 0 {
			logf("ERROR watching simulator process %d: %v", pid, e)
			os.Exit(1)
		}
		go func() {
			pWaitForSingleObject.Call(h, infinite)
			pCloseHandle.Call(h)
			logf("openOMSI exited; facade terminating")
			os.Exit(0)
		}()
	})
}

var retitleOnce sync.Once
var retitleCallback uintptr

func retitleOpenOMSI() {
	retitleOnce.Do(func() {
		retitleCallback = syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
			var pid uint32
			pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid != atomic.LoadUint32(&openOmsiPID) {
				return 1
			}
			vis, _, _ := pIsWindowVisible.Call(hwnd)
			if vis == 0 {
				return 1
			}
			pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(u16("OMSI 2.3.004"))))
			return 1
		})
	})
	pEnumWindows.Call(retitleCallback, 0)
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	exe, _ := os.Executable()
	logPath = filepath.Join(filepath.Dir(exe), "compat-facade-v1.1.3.log")
	_ = os.Remove(logPath)
	logf("OpenOMSI BCS compatibility facade v1.1.3 - by %s starting", bridgeAuthor)

	if len(os.Args) >= 2 {
		if v, err := strconv.ParseUint(os.Args[1], 10, 32); err == nil {
			atomic.StoreUint32(&openOmsiPID, uint32(v))
		}
	}
	if len(os.Args) >= 3 {
		omsiRoot = os.Args[2]
	}
	if len(os.Args) >= 4 {
		mapRel = os.Args[3]
	}
	if len(os.Args) >= 5 {
		wantedLine = os.Args[4]
	}
	if len(os.Args) >= 6 {
		wantedTour = os.Args[5]
	}
	if len(os.Args) >= 7 {
		bcsLogPath = os.Args[6]
	}
	if strings.TrimSpace(bcsLogPath) == "" {
		bcsLogPath = filepath.Join(omsiRoot, "Busbetrieb-Simulator", "log.txt")
	}
	logf("openOMSI PID=%d root=%q map=%q line=%q tour=%q bcslog=%q", atomic.LoadUint32(&openOmsiPID), omsiRoot, mapRel, wantedLine, wantedTour, bcsLogPath)
	compatDir := filepath.Dir(exe)
	readyPath := filepath.Join(compatDir, "facade-ready-v1.1.3.flag")
	_ = os.Remove(readyPath)
	openDriver := filepath.Join(compatDir, "bcs-driver-openomsi-v1.1.3.odr")
	nativeDriver := filepath.Join(omsiRoot, "Drivers", "bbs.odr")
	if len(os.Args) >= 8 {
		openDriver = os.Args[7]
	}
	if len(os.Args) >= 9 {
		nativeDriver = os.Args[8]
	}
	if len(os.Args) >= 10 {
		currentShiftID = strings.TrimSpace(os.Args[9])
	}
	var err error
	driverState, err = loadDriverSync(openDriver, nativeDriver, compatDir)
	if err != nil {
		logf("ERROR driver initialization: %v", err)
		return
	}
	if !initOmsiMemoryShim() {
		return
	}
	logf("Driver files: openOMSI=%q native BCS=%q", openDriver, nativeDriver)
	logf("Driver baseline Auswertungsdaten: %s", driverState.Last.evaluation())
	watchDriver()

	hInst, _, _ := pGetModuleHandleW.Call(0)
	cb := syscall.NewCallback(wndProc)

	// Exact identities this BCS build searches for.
	app := createFake("TApplication", "Omsi 2", true, 0, hInst, cb)
	mainHwnd = createFake("TForm_main", "OMSI 2.3.004", true, app, hInst, cb)
	menuHwnd = createFake("Tform_menu3", "form_menu3", true, app, hInst, cb)

	// OmsiStartmenue.java explicitly requires this form to be VISIBLE and in
	// the same PID as TForm_main. v0.4.5 incorrectly created it hidden.
	startHwnd = createFake("Tform_start", "<Start...>", true, 0, hInst, cb)
	// The BCS posts a synthetic click at client coordinate (123,751). PostMessage
	// does not require the point to be inside the client area, but making the form
	// tall enough removes that ambiguity and more closely resembles the legacy
	// Delphi start form. The window remains off-screen/no-activate.
	if startHwnd != 0 {
		pSetWindowPos.Call(startHwnd, 0, coord(-30000), coord(-30000), 1024, 800, swpNoActivate|swpNoZOrder)
		logf("Tform_start compatibility geometry set to 1024x800; BCS click (123,751) is inside client bounds")
	}

	// Functional timetable dialog expected by OmsiFahrplan.java. It is initially
	// hidden, then appears immediately after BCS clicks Tform_start.
	setTTHwnd = createFake("Tform_settt", "Set Time Table", false, app, hInst, cb)
	lineCombo := createChild("TComboBox", "", setTTHwnd, hInst, cb, 101, 128, 8, 240, 24)
	tourCombo := createChild("TComboBox", "", setTTHwnd, hInst, cb, 102, 128, 40, 240, 24)
	haltCombo := createChild("TComboBox", "", setTTHwnd, hInst, cb, 103, 128, 187, 240, 24)
	trackHwnd = createChild("TTrackBar", "", setTTHwnd, hInst, cb, 104, 120, 64, 300, 32)
	okHwnd = createChild("TButton", "OK", setTTHwnd, hInst, cb, 1, 8, 248, 72, 28)
	cancelHwnd = createChild("TButton", "Cancel", setTTHwnd, hInst, cb, 2, 88, 248, 72, 28)
	addCombo(lineCombo, []string{wantedLine})
	addCombo(tourCombo, []string{wantedTour})
	addCombo(haltCombo, []string{""})

	// Other legacy forms that BCS may probe.
	_ = createFake("Tform_menu", "form_menu", false, app, hInst, cb)
	_ = createFake("Tform_menu2", "form_menu2", false, app, hInst, cb)
	_ = createFake("Tform_submenu", "form_submenu", false, app, hInst, cb)
	_ = createFake("Tform_splashscreen", "form_splashscreen", false, app, hInst, cb)
	_ = createFake("Tform_timetable_running", "Time Table", false, 0, hInst, cb)
	_ = createFake("Tform_timetable", "Time Table", false, 0, hInst, cb)
	_ = createFake("Tform_setline", "Set Line and Terminus...", false, app, hInst, cb)
	_ = createFake("Tform_selectVeh", "Select Vehicle...", false, 0, hInst, cb)
	_ = createFake("Tform_daytime", "Select time and date...", false, 0, hInst, cb)
	confirmHwnd = createFake("Tform_messagedlg", "form_messagedlg", false, app, hInst, cb)
	for _, hwnd := range []uintptr{app, mainHwnd, menuHwnd, startHwnd, setTTHwnd, lineCombo, tourCombo, haltCombo, trackHwnd, okHwnd, cancelHwnd, confirmHwnd} {
		if hwnd == 0 {
			logf("ERROR: an essential BCS compatibility window failed; readiness will not be published")
			return
		}
	}

	// BCS 5.0.0.1 does not begin OmsiStartmenue automation immediately.
	// StartController first waits until logfile.txt contains the exact legacy
	// startup sentinel "Spline-Helper initialisieren". openOMSI never writes
	// that OMSI 2 line, which is why v0.4.6.5 saw no WM_NULL/click at all.
	prepareLegacyLogfile()
	// BCS 5.0.0.1 StartController waits for <OMSI root>\closecheck to
	// exist before it even starts scanning logfile.txt for Spline-Helper or
	// looking for Tform_start. Original OMSI/bbs.dll supplies that gate; the
	// external openOMSI plugin host does not reliably create it.
	ensureBCSClosecheckGate()
	watchBCSStartMenuAck()

	// Publish the result OmsiStartmenue polls for immediately. Keep Tform_start
	// visible for FindWindow/IsWindowVisible compatibility, but do not require a
	// synthetic mouse click to advance. A watchdog restores these markers if
	// logfile.txt is recreated during openOMSI startup.
	syntheticLoadingMarker()
	ensureLegacyStartupMarkers()
	logf("state=WAIT_BCS_START_ACK+WAIT_OPENOMSI_READY; keeping Tform_start visible until both sides are ready")
	if err := os.WriteFile(readyPath, []byte("ready\r\n"), 0644); err != nil {
		logf("WARN could not write facade ready flag: %v", err)
	} else {
		logf("facade ready flag written: %s", readyPath)
	}

	if atomic.LoadUint32(&openOmsiPID) == 0 {
		watchOpenOMSIPIDFile(compatDir)
	} else {
		retitleOpenOMSI()
		watchSimulatorExit(atomic.LoadUint32(&openOmsiPID))
	}
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			if atomic.LoadUint32(&openOmsiPID) != 0 {
				retitleOpenOMSI()
			}
		}
	}()

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
