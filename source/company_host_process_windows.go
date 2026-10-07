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

// A separate process group leaves Ctrl+C handling with the supervisor. Closing
// its Job Object also terminates the server's tunnel, including when the console
// is closed or the supervisor exits unexpectedly.
func prepareCompanyHostProcess(child *exec.Cmd) {
	child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x00000004}
}

func ownCompanyHostProcess(child *exec.Cmd) (func(), error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	createJob := kernel.NewProc("CreateJobObjectW")
	setJob := kernel.NewProc("SetInformationJobObject")
	assignJob := kernel.NewProc("AssignProcessToJobObject")
	job, _, err := createJob.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("cannot own dedicated server process: %w", err)
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
		return nil, fmt.Errorf("cannot configure server process ownership: %w", err)
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
		return nil, fmt.Errorf("cannot assign dedicated server process: %w", err)
	}
	// The primary thread was created suspended. Assign the job before it can
	// start cloudflared, then resume that thread through documented Win32 APIs.
	if err := resumeCompanyHostProcess(uint32(child.Process.Pid)); err != nil {
		closeJob()
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(closeJob) }, nil
}

type companyHostThreadEntry struct {
	Size, Usage, ThreadID, OwnerPID uint32
	BasePriority, DeltaPriority     int32
	Flags                           uint32
}

func resumeCompanyHostProcess(pid uint32) error {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	snapshotProc := kernel.NewProc("CreateToolhelp32Snapshot")
	first := kernel.NewProc("Thread32First")
	next := kernel.NewProc("Thread32Next")
	open := kernel.NewProc("OpenThread")
	resume := kernel.NewProc("ResumeThread")
	snapshot, _, err := snapshotProc.Call(0x00000004, 0)
	if snapshot == ^uintptr(0) {
		return fmt.Errorf("cannot inspect suspended server thread: %w", err)
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))
	entry := companyHostThreadEntry{Size: uint32(unsafe.Sizeof(companyHostThreadEntry{}))}
	ok, _, _ := first.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if entry.OwnerPID == pid {
			thread, _, err := open.Call(0x0002, 0, uintptr(entry.ThreadID))
			if thread == 0 {
				return fmt.Errorf("cannot open suspended server thread: %w", err)
			}
			count, _, err := resume.Call(thread)
			_ = syscall.CloseHandle(syscall.Handle(thread))
			if uint32(count) == 0xffffffff {
				return fmt.Errorf("cannot resume dedicated server: %w", err)
			}
			return nil
		}
		entry.Size = uint32(unsafe.Sizeof(companyHostThreadEntry{}))
		ok, _, _ = next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return fmt.Errorf("dedicated server's suspended thread was not found")
}
