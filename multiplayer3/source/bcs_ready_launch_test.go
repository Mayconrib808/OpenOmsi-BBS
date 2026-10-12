package main

import (
 "bytes"
 "crypto/sha256"
 "fmt"
 "os"
 "path/filepath"
 "testing"
)

func TestBCSReadyLaunchChild(t *testing.T) {
 if os.Getenv("BBS_READY_LAUNCH_TEST") != "1" { return }
 folder := filepath.Dir(os.Args[0])
 companion,err:=os.ReadFile(filepath.Join(folder,"Plugins",bcsReadyPluginName))
 if os.Getenv("BBS_READY_EXPECT_PLUGIN")=="yes" {
  if err!=nil || !bytes.Equal(companion,[]byte(bcsReadyPluginSource)) {t.Fatal("readiness plugin missing when the game starts",err)}
  bbs,err:=os.ReadFile(filepath.Join(folder,"Plugins","bbs.lua"))
  if err!=nil || string(bbs)!="original BCS plugin" {t.Fatal("BCS plugin was modified",err)}
 }else if !os.IsNotExist(err) {t.Fatal("non-BCS launch installed a readiness plugin",err)}
}
func TestBCSReadyPluginInstalledByActualLaunchBeforeGameStarts(t *testing.T) {
 testReadyLaunch(t,true)
}
func TestBCSReadyPluginNotInstalledForOrdinaryGameLaunch(t *testing.T) {
 testReadyLaunch(t,false)
}
func testReadyLaunch(t *testing.T,scheduled bool) {
 t.Helper()
 base:=t.TempDir()
 root:=filepath.Join(base,"OMSI 2")
 dir:=filepath.Join(base,"integration","OpenOMSI-BBS")
 client:=filepath.Join(base,"client")
 for _,folder:=range []string{root,client,filepath.Join(dir,"compat"),filepath.Join(dir,"PeDePeAdapter","plugins")} {
  if err:=os.MkdirAll(folder,0755);err!=nil{t.Fatal(err)}
 }
 exe,err:=os.Executable();if err!=nil{t.Fatal(err)}
 data,err:=os.ReadFile(exe);if err!=nil{t.Fatal(err)}
 game:=filepath.Join(client,"openomsi.exe")
 if err=os.WriteFile(game,data,0755);err!=nil{t.Fatal(err)}
 helper:=[]byte("MZ test-only BCS helper")
 if err=os.WriteFile(filepath.Join(dir,"compat","omsi-plugin-host32.exe"),helper,0600);err!=nil{t.Fatal(err)}
 oldHash:=bundledPluginHostSHA256
 bundledPluginHostSHA256=fmt.Sprintf("%x",sha256.Sum256(helper))
 defer func(){bundledPluginHostSHA256=oldHash}()
 if err=ensureLayout(dir);err!=nil{t.Fatal(err)}
 if err=os.WriteFile(filepath.Join(dir,"PeDePeAdapter","plugins","bbs.lua"),[]byte("original BCS plugin"),0600);err!=nil{t.Fatal(err)}
 t.Setenv("BBS_READY_LAUNCH_TEST","1")
 expected:="no";if scheduled{expected="yes"};t.Setenv("BBS_READY_EXPECT_PLUGIN",expected)
 args:=[]string{"-test.run=^TestBCSReadyLaunchChild$","--","--root",root}
 if scheduled{args=append(args,"--schedule","--line","178","--tour","5 (Pow)","--trip","05:52")}
 process,err:=launchNative(localConfig{Root:root,Game:game},args,dir)
 if err!=nil{t.Fatal(err)}
 if err=process.Wait();err!=nil{t.Fatal("real launch did not prepare BCS readiness",err)}
}
