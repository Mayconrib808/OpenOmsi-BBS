//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"syscall"
)

func TestUpdateHandoffProcessChild(t *testing.T) {
    mode:=os.Getenv("BRIDGE_UPDATE_HANDOFF_CHILD")
    if mode==""{return}
    ready:=os.Args[len(os.Args)-1]
    marker:=os.Getenv("BRIDGE_UPDATE_HANDOFF_RESULT")
    var preflight error
    if mode=="error"{preflight=errors.New("fixture: BCS is still open")}
    if err:=announceBridgeUpdateHandoff(ready,preflight);err!=nil{os.Exit(2)}
    if preflight!=nil{os.Exit(1)}
    if err:=awaitBridgeUpdateAcceptance(ready,5*time.Second);err!=nil{os.Exit(3)}
    cleanBridgeUpdateHandoff(ready)
    if err:=os.WriteFile(marker,[]byte("accepted"),0600);err!=nil{os.Exit(4)}
    os.Exit(0)
}

func TestUpdateHandoffStartsOnceAndRequiresBothSides(t *testing.T) {
    source,err:=os.Executable();if err!=nil{t.Fatal(err)}
    for _,mode:=range []string{"ready","error"}{t.Run(mode,func(t *testing.T){
        cache:=t.TempDir();t.Setenv("LOCALAPPDATA",cache)
        result:=filepath.Join(cache,"install-started.txt")
        t.Setenv("BRIDGE_UPDATE_HANDOFF_RESULT",result)
        t.Setenv("BRIDGE_UPDATE_HANDOFF_CHILD",mode)
        err:=startBridgeUpdateWithAcknowledgment(source,filepath.Dir(source),"-test.run=^TestUpdateHandoffProcessChild$","--")
        if mode=="error" {
            if err==nil{t.Fatal("failed preflight closed Setup")}
            if _,e:=os.Stat(result);!os.IsNotExist(e){t.Fatal("rejected updater installed anyway")}
            return
        }
        if err!=nil{t.Fatal(err)}
        deadline:=time.Now().Add(5*time.Second)
        for time.Now().Before(deadline){
            if b,e:=os.ReadFile(result);e==nil&&string(b)=="accepted"{return}
            time.Sleep(20*time.Millisecond)
        }
        t.Fatal("first updater launch did not receive Setup acceptance")
    })}
}

func TestAtomicSetupReplacementWaitsForWindowsSharingRelease(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"Setup.exe")
    if err:=os.WriteFile(path,[]byte("old setup"),0600);err!=nil{t.Fatal(err)}
    name,err:=syscall.UTF16PtrFromString(path);if err!=nil{t.Fatal(err)}
    handle,err:=syscall.CreateFile(name,syscall.GENERIC_READ,syscall.FILE_SHARE_READ,nil,syscall.OPEN_EXISTING,syscall.FILE_ATTRIBUTE_NORMAL,0)
    if err!=nil{t.Fatal(err)}
    released:=make(chan struct{})
    go func(){time.Sleep(250*time.Millisecond);syscall.CloseHandle(handle);close(released)}()
    // One update attempt must replace the file after its temporary lock is
    // released. The user must not have to click Download/Update again.
    err=writeSetupAtomic(path,[]byte("new verified setup"));<-released
    if err!=nil{t.Fatal(err)}
    b,err:=os.ReadFile(path);if err!=nil||string(b)!="new verified setup"{t.Fatalf("replacement failed: %q %v",b,err)}
}

func TestUpdateProcessChild(t *testing.T) {
	if os.Getenv("BRIDGE_UPDATE_LAUNCH_CHILD") != "1" {
		return
	}
	args := os.Args
	marker := args[len(args)-1]
	if err := os.WriteFile(marker, []byte("started"), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestUpdateProcessStartsWithoutConsoleHandles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder with spaces")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "update child.exe")
	if err = os.WriteFile(executable, data, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "launch result.txt")
	t.Setenv("BRIDGE_UPDATE_LAUNCH_CHILD", "1")
	if err = startBridgeUpdateProcess(executable, dir, "-test.run=^TestUpdateProcessChild$", "--", marker); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if result, err := os.ReadFile(marker); err == nil && string(result) == "started" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("update process did not receive its argument or start")
}
