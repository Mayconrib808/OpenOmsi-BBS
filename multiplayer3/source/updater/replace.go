package main

import (
 "bytes"
 "fmt"
 "os"
 "path/filepath"
)

// Retire the old file before installing its replacement. Windows can rename
// a loaded executable even when replacing its mapped image is denied.
// An install failure restores the original path before reporting an error.
func retireAndReplace(temp,target string,expected []byte,rename func(string,string)error) error {
 current,err:=os.ReadFile(target)
 if err!=nil{return err}
 if !bytes.Equal(current,expected){return fmt.Errorf("installation changed during update; preserved")}
 f,err:=os.CreateTemp(filepath.Dir(target),".update-retired-*.bak")
 if err!=nil{return err}
 retired:=f.Name()
 if err=f.Close();err!=nil{_ = os.Remove(retired);return err}
 if err=os.Remove(retired);err!=nil{return err}
 if err=rename(target,retired);err!=nil{return err}
 if err=rename(temp,target);err!=nil {
  if _,statErr:=os.Lstat(target);!os.IsNotExist(statErr) {
   return fmt.Errorf("%w; original preserved at %s; concurrent target preserved",err,retired)
  }
  if restoreErr:=rename(retired,target);restoreErr!=nil {
   return fmt.Errorf("%w; original preserved at %s; restoration failed: %v",err,retired,restoreErr)
  }
  return err
 }
 // A running image can remain undeletable. Keep it as a recovery file.
 _ = os.Remove(retired)
 return nil
}
