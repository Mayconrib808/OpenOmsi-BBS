//go:build windows
package main
import (
 "errors"
 "fmt"
 "os"
 "syscall"
 "time"
)
func transientReplacementError(err error)bool {
 return errors.Is(err,syscall.Errno(5)) || errors.Is(err,syscall.Errno(32)) || errors.Is(err,syscall.Errno(33))
}
func replacePreparedFile(temp,target string)error {
 original,readErr:=os.ReadFile(target)
 if readErr!=nil && !os.IsNotExist(readErr){return readErr}
 deadline:=time.Now().Add(12*time.Second)
 var last error
 for {
  last=os.Rename(temp,target)
  if last==nil{return nil}
  if !transientReplacementError(last){return last}
  if readErr==nil {
   // Do not repeat this operation after a partial replacement or failed restore.
   if err:=retireAndReplace(temp,target,original,os.Rename);err==nil{return nil
   }else if !transientReplacementError(err){return err
   }else{last=err}
  }
  if !time.Now().Before(deadline){break}
  time.Sleep(200*time.Millisecond)
 }
 return fmt.Errorf("Não foi possível substituir %s. Feche outras janelas do Setup.exe e tente novamente. Se o bloqueio persistir, confira a permissão de gravação e o histórico de proteção do Windows. Nenhuma proteção foi desativada. Detalhe: %w",target,last)
}
