package main

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// The Setup must not exit simply because CreateProcess succeeded. The
// downloaded updater acknowledges its own startup and preflight first.
func newBridgeUpdateHandoffPath() (string,error) {
    cache,err:=os.UserCacheDir()
    if err!=nil{return "",err}
    dir:=filepath.Join(cache,"OpenOmsi-BBS","Updates")
    if err=os.MkdirAll(dir,0700);err!=nil{return "",err}
    f,err:=os.CreateTemp(dir,".bridge-handoff-*.signal")
    if err!=nil{return "",err}
    path:=f.Name()
    if e:=f.Close();e!=nil{os.Remove(path);return "",e}
    if e:=os.Remove(path);e!=nil{return "",e}
    return path,nil
}

// The helper may create only a new handshake in the user's updater cache;
// never accept an arbitrary file path from command-line arguments.
func validateBridgeHandoffPath(path string) error {
    cache,err:=os.UserCacheDir()
    if err!=nil{return err}
    dir:=filepath.Join(cache,"OpenOmsi-BBS","Updates")
    if !filepath.IsAbs(path)||!strings.EqualFold(filepath.Clean(filepath.Dir(path)),filepath.Clean(dir))||
       !strings.HasPrefix(filepath.Base(path),".bridge-handoff-")||!strings.HasSuffix(path,".signal"){
        return fmt.Errorf("invalid updater handoff path")
    }
    return nil
}
func announceBridgeUpdateHandoff(path string,preflight error) error {
    if path==""{return nil}
    if err:=validateBridgeHandoffPath(path);err!=nil{return err}
    if _,err:=os.Stat(path+".canceled");err==nil{return fmt.Errorf("updater startup was canceled by Setup")}
    content:="READY\n"
    if preflight!=nil{content="ERROR: "+preflight.Error()+"\n"}
    dir:=filepath.Dir(path)
    f,err:=os.CreateTemp(dir,".handoff-pending-*.tmp")
    if err!=nil{return err}
    temp:=f.Name()
    defer os.Remove(temp)
    if _,err=f.WriteString(content);err!=nil{f.Close();return err}
    if err=f.Sync();err!=nil{f.Close();return err}
    if err=f.Close();err!=nil{return err}
    // The parent observes either the complete READY/ERROR or nothing, never
    // a partially written handoff.
    return os.Rename(temp,path)
}
// A slow antivirus scan can outlast Setup's startup timeout. The child must
// never close Setup or install later unless that exact attempt was accepted.
func acceptBridgeUpdateHandoff(path string) error {
    if err:=validateBridgeHandoffPath(path);err!=nil{return err}
    return os.WriteFile(path+".accepted",[]byte("ACCEPTED\n"),0600)
}
func awaitBridgeUpdateAcceptance(path string,timeout time.Duration) error {
    deadline:=time.Now().Add(timeout)
    for {
        if _,err:=os.Stat(path+".canceled");err==nil{return fmt.Errorf("Setup canceled this updater attempt")}
        raw,err:=os.ReadFile(path+".accepted")
        if err==nil && string(raw)=="ACCEPTED\n"{return nil}
        if err!=nil && !os.IsNotExist(err){return err}
        if time.Now().After(deadline){return fmt.Errorf("Setup did not accept this updater attempt; no files were changed")}
        time.Sleep(50*time.Millisecond)
    }
}
func cleanBridgeUpdateHandoff(path string) {
    if path==""{return}
    os.Remove(path);os.Remove(path+".accepted");os.Remove(path+".canceled")
}
func awaitBridgeUpdateHandoff(path string,timeout time.Duration) error {
    deadline:=time.Now().Add(timeout)
    for {
        raw,err:=os.ReadFile(path)
        if err==nil {
            message:=strings.TrimSpace(string(raw))
            switch {
            case message=="READY":return nil
            case strings.HasPrefix(message,"ERROR: "):return errors.New(strings.TrimPrefix(message,"ERROR: "))
            default:return fmt.Errorf("updater returned an invalid startup confirmation")
            }
        }
        if !os.IsNotExist(err){return err}
        if time.Now().After(deadline){return fmt.Errorf("o atualizador baixado não confirmou a inicialização; o Setup foi mantido aberto")}
        time.Sleep(50*time.Millisecond)
    }
}
