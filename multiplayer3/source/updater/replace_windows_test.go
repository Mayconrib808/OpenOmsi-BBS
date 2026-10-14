//go:build windows
package main
import (
 "bytes"
 "io"
 "os"
 "os/exec"
 "path/filepath"
 "syscall"
 "testing"
 "time"
)
func TestLoadedExecutableHelper(t *testing.T) {
 if os.Getenv("BBS_UPDATER_TEST_HOLD")!="1"{return}
 dir:=os.Getenv("BBS_UPDATER_TEST_DIR")
 _ = os.WriteFile(filepath.Join(dir,"ready"),[]byte("ready"),0600)
 for n:=0;n<300;n++{if _,err:=os.Stat(filepath.Join(dir,"stop"));err==nil{return};time.Sleep(100*time.Millisecond)}
}
func TestAtomicWriteReplacesRunningWindowsExecutable(t *testing.T) {
 dir:=t.TempDir();original,err:=os.Executable();if err!=nil{t.Fatal(err)}
 data,err:=os.ReadFile(original);if err!=nil{t.Fatal(err)}
 target:=filepath.Join(dir,"Setup.exe")
 if err=os.WriteFile(target,data,0755);err!=nil{t.Fatal(err)}
 cmd:=exec.Command(target,"-test.run=^TestLoadedExecutableHelper$")
 cmd.Env=append(os.Environ(),"BBS_UPDATER_TEST_HOLD=1","BBS_UPDATER_TEST_DIR="+dir)
 cmd.Stdout=io.Discard;cmd.Stderr=io.Discard
 if err=cmd.Start();err!=nil{t.Fatal(err)}
 defer func(){_ = os.WriteFile(filepath.Join(dir,"stop"),[]byte("stop"),0600);_ = cmd.Wait()}()
 ready:=false
 for n:=0;n<100;n++{if _,err=os.Stat(filepath.Join(dir,"ready"));err==nil{ready=true;break};time.Sleep(50*time.Millisecond)}
 if !ready{t.Fatal("test executable did not start")}
 replacement:=[]byte("MZ replacement executable")
 if err=atomicWrite(target,replacement);err!=nil{t.Fatal("loaded Setup.exe replacement failed",err)}
 if got,err:=os.ReadFile(target);err!=nil||!bytes.Equal(got,replacement){t.Fatal("replacement not installed",err)}
}
func TestAtomicWriteRetriesTemporaryWindowsFileLock(t *testing.T) {
 dir:=t.TempDir();target:=filepath.Join(dir,"Setup.exe");_ = os.WriteFile(target,[]byte("old"),0600)
 name,err:=syscall.UTF16PtrFromString(target);if err!=nil{t.Fatal(err)}
 handle,err:=syscall.CreateFile(name,syscall.GENERIC_READ,syscall.FILE_SHARE_READ,nil,syscall.OPEN_EXISTING,syscall.FILE_ATTRIBUTE_NORMAL,0)
 if err!=nil{t.Fatal(err)}
 unlocked:=make(chan struct{})
 go func(){time.Sleep(500*time.Millisecond);_ = syscall.CloseHandle(handle);close(unlocked)}()
 err=atomicWrite(target,[]byte("new"));<-unlocked
 if err!=nil{t.Fatal("temporary file lock was not retried",err)}
 if got,_:=os.ReadFile(target);string(got)!="new"{t.Fatal("replacement missing")}
}
