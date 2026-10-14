package main

import (
 "fmt"
 "io"
 "log"
 "os"
 "path/filepath"
 "sync"
)

var bcsGameLogPath = defaultBCSGameLogPath

func defaultBCSGameLogPath() (string,error) {
 home,err:=os.UserHomeDir()
 if err!=nil{return "",err}
 if !filepath.IsAbs(home){return "",fmt.Errorf("BCS game log needs an absolute user profile path")}
 return filepath.Join(home,".openomsi","game.log"),nil
}
func openBCSGameLog(path string)(*os.File,error) {
 if !filepath.IsAbs(path){return nil,fmt.Errorf("BCS game log needs an absolute path")}
 if err:=os.MkdirAll(filepath.Dir(path),0755);err!=nil{return nil,err}
 // A new launch must not inherit an old trip's readiness marker.
 return os.OpenFile(path,os.O_CREATE|os.O_WRONLY|os.O_TRUNC,0600)
}
type resilientGameLogWriter struct {
 mu sync.Mutex
 outputs []io.Writer
 failed []bool
}
func gameLogOutput(diagnostics io.Writer, mirrors ...io.Writer)io.Writer {
 outputs:=append([]io.Writer{diagnostics},mirrors...)
 return &resilientGameLogWriter{outputs:outputs,failed:make([]bool,len(outputs))}
}
func (w *resilientGameLogWriter)Write(p []byte)(int,error) {
 w.mu.Lock()
 success:=false
 var failures []string
 var firstErr error
 for i,out:=range w.outputs {
  if out==nil || w.failed[i]{continue}
  n,err:=out.Write(p)
  if err==nil && n!=len(p){err=io.ErrShortWrite}
  if err!=nil {
   w.failed[i]=true
   if firstErr==nil{firstErr=err}
   failures=append(failures,fmt.Sprintf("game output destination %d unavailable; continuing other log destinations: %v",i,err))
  }else{success=true}
 }
 w.mu.Unlock()
 // Diagnose once without closing the child pipes or losing later log lines.
 for _,failure:=range failures{log.Print(failure)}
 if success{return len(p),nil}
 if firstErr==nil{firstErr=io.ErrClosedPipe}
 return 0,fmt.Errorf("no game log output destination available: %w",firstErr)
}
