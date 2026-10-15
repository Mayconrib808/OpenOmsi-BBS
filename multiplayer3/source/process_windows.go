//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

func detachHostAgent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x00000008, HideWindow: true}
}

// A separate process group leaves Ctrl+C handling with the supervisor. Closing
// its Job Object also terminates the server's tunnel, including when the console
// is closed or the supervisor exits unexpectedly.
func prepareCompanyHostProcess(child *exec.Cmd) {
	if child.SysProcAttr == nil {
		child.SysProcAttr = &syscall.SysProcAttr{}
	}
	// The native server has tunnel=0 and cloudflared has --no-autoupdate:
	// neither is allowed to spawn the other. Start normally, then attach the
	// process to its lifetime job before the supervisor starts the next child.
	// Never guess a primary thread from an unordered system-wide snapshot.
	child.SysProcAttr.CreationFlags |= 0x00000200 | 0x08000000
	child.SysProcAttr.HideWindow = true
}

func ownCompanyHostProcess(child *exec.Cmd) (func(), error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	createJob := kernel.NewProc("CreateJobObjectW")
	setJob := kernel.NewProc("SetInformationJobObject")
	assignJob := kernel.NewProc("AssignProcessToJobObject")
	job, _, err := createJob.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("cannot own owned child process: %w", err)
	}
	closeJob := func() { _ = syscall.CloseHandle(syscall.Handle(job)) }
	size := 112
	if unsafe.Sizeof(uintptr(0)) == 8 {
		size = 144
	}
	// JOBOBJECT_EXTENDED_LIMIT_INFORMATION: LimitFlags is at offset 16 on
	// both Windows architectures. JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE=0x2000.
	limits := make([]byte, size)
	binary.LittleEndian.PutUint32(limits[16:20], 0x00002000)
	ok, _, err := setJob.Call(job, 9, uintptr(unsafe.Pointer(&limits[0])), uintptr(len(limits)))
	if ok == 0 {
		closeJob()
		return nil, fmt.Errorf("cannot configure child process ownership: %w", err)
	}
	// AssignProcessToJobObject requires PROCESS_SET_QUOTA | PROCESS_TERMINATE.
	process, err := syscall.OpenProcess(0x0100|0x0001, false, uint32(child.Process.Pid))
	if err != nil {
		closeJob()
		return nil, err
	}
	defer syscall.CloseHandle(process)
	ok, _, err = assignJob.Call(job, uintptr(process))
	if ok == 0 {
		closeJob()
		return nil, fmt.Errorf("cannot assign owned child process: %w", err)
	}
	var once sync.Once
	return func() { once.Do(closeJob) }, nil
}

