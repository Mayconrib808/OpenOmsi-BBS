package main
import (
 "errors"
 "os"
 "path/filepath"
 "testing"
)
func TestRetirementRestoresOldFileWhenInstallFails(t *testing.T) {
 dir:=t.TempDir();target:=filepath.Join(dir,"Setup.exe");temp:=filepath.Join(dir,"update.tmp")
 _ = os.WriteFile(target,[]byte("old"),0600);_ = os.WriteFile(temp,[]byte("new"),0600)
 denied:=errors.New("install denied")
 rename:=func(from,to string)error{if from==temp{return denied};return os.Rename(from,to)}
 if err:=retireAndReplace(temp,target,[]byte("old"),rename);!errors.Is(err,denied){t.Fatal(err)}
 if b,_:=os.ReadFile(target);string(b)!="old"{t.Fatal("original was not restored",string(b))}
}
func TestRetirementPreservesConcurrentFileChange(t *testing.T) {
 dir:=t.TempDir();target:=filepath.Join(dir,"Setup.exe");temp:=filepath.Join(dir,"update.tmp")
 _ = os.WriteFile(target,[]byte("another change"),0600);_ = os.WriteFile(temp,[]byte("new"),0600)
 if err:=retireAndReplace(temp,target,[]byte("old"),os.Rename);err==nil{t.Fatal("concurrent change overwritten")}
 if b,_:=os.ReadFile(target);string(b)!="another change"{t.Fatal("changed file lost")}
}
