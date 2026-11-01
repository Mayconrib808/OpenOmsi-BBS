package main

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"
)

func TestUpdaterHandoffOnlyInUserCache(t *testing.T){
    root:=t.TempDir()
    t.Setenv("LOCALAPPDATA",root)
    t.Setenv("XDG_CACHE_HOME",root)
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{t.Fatal(err)}
    if _,err:=os.Stat(ready);!os.IsNotExist(err){t.Fatalf("handoff must not exist before child starts: %v",err)}
    if err:=validateBridgeHandoffPath(ready);err!=nil{t.Fatal(err)}
    if err:=validateBridgeHandoffPath(filepath.Join(root,"outside.signal"));err==nil{t.Fatal("arbitrary path accepted")}
    if err:=announceBridgeUpdateHandoff(ready,nil);err!=nil{t.Fatal(err)}
    if err:=awaitBridgeUpdateHandoff(ready,100*time.Millisecond);err!=nil{t.Fatal(err)}
    if err:=os.Remove(ready);err!=nil{t.Fatal(err)}
}
func TestUpdaterHandoffDisplaysPreflightFailureWithoutClosingSetup(t *testing.T){
    root:=t.TempDir()
    t.Setenv("LOCALAPPDATA",root)
    t.Setenv("XDG_CACHE_HOME",root)
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{t.Fatal(err)}
    want:=errors.New("feche o BCS antes de atualizar")
    if err:=announceBridgeUpdateHandoff(ready,want);err!=nil{t.Fatal(err)}
    got:=awaitBridgeUpdateHandoff(ready,100*time.Millisecond)
    if got==nil||!strings.Contains(got.Error(),"feche o BCS"){t.Fatalf("preflight error hidden: %v",got)}
    os.Remove(ready)
}
func TestUpdaterHandoffTimeoutKeepsSetupOpen(t *testing.T){
    root:=t.TempDir()
    t.Setenv("LOCALAPPDATA",root)
    t.Setenv("XDG_CACHE_HOME",root)
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{t.Fatal(err)}
    if err=awaitBridgeUpdateHandoff(ready,15*time.Millisecond);err==nil||!strings.Contains(err.Error(),"Setup foi mantido aberto"){
        t.Fatalf("missing explicit timeout: %v",err)
    }
}
func TestLateUpdaterCannotInstallAfterSetupCancels(t *testing.T){
    root:=t.TempDir();t.Setenv("LOCALAPPDATA",root);t.Setenv("XDG_CACHE_HOME",root)
    ready,err:=newBridgeUpdateHandoffPath();if err!=nil{t.Fatal(err)}
    defer cleanBridgeUpdateHandoff(ready)
    if err=os.WriteFile(ready+".canceled",[]byte("CANCELED\n"),0600);err!=nil{t.Fatal(err)}
    if err=announceBridgeUpdateHandoff(ready,nil);err==nil{t.Fatal("late child ignored canceled attempt")}
    if err=awaitBridgeUpdateAcceptance(ready,time.Millisecond);err==nil{t.Fatal("late child proceeded without acceptance")}
}
func TestUpdaterRejectsHandoffPathOutsideCacheOrWithoutWaitingSetup(t *testing.T){
    root:=t.TempDir()
    t.Setenv("LOCALAPPDATA",root)
    t.Setenv("XDG_CACHE_HOME",root)
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{t.Fatal(err)}
    args:=[]string{"--target-dir",root,"--wait-setup","12","--reopen","--handoff-ready",ready}
    got,err:=parseBridgeUpdateOptions(args)
    if err!=nil||got.handoff!=ready{t.Fatalf("valid handoff rejected: %v %#v",err,got)}
    for _,bad:=range [][]string{
        {"--target-dir",root,"--handoff-ready",ready},
        {"--target-dir",root,"--wait-setup","12","--handoff-ready",ready},
        {"--target-dir",root,"--wait-setup","12","--reopen","--handoff-ready",filepath.Join(root,"other.signal")},
    }{
        if _,err:=parseBridgeUpdateOptions(bad);err==nil{t.Fatalf("unsafe args accepted: %v",bad)}
    }
}
