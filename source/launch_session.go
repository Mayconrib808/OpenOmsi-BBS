package main

import (
 "crypto/sha256"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
)
var errBridgeBusy = errors.New("a bridge session is already running for this OMSI folder")
type launchSessionState struct { PID int; Trip TripInfo }
func launchSessionPath(root, packageDir string) string {
 sum:=sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
 return filepath.Join(managedDataDir(packageDir),"Sessions",fmt.Sprintf("%x.json",sum[:16]))
}
func sameLaunchTrip(a,b TripInfo) bool {
 eq:=func(x,y string)bool{return strings.EqualFold(strings.TrimSpace(x),strings.TrimSpace(y))}
 return validLaunchShiftID(a.ShiftID) && a.ShiftID==b.ShiftID && eq(a.MapName,b.MapName) && eq(a.Line,b.Line) && strings.TrimSpace(a.Tour)==strings.TrimSpace(b.Tour) && a.TripStart==b.TripStart && a.TripEnd==b.TripEnd && strings.TrimSpace(a.RouteText)==strings.TrimSpace(b.RouteText)
}
func validLaunchShiftID(id string) bool {
 if id=="" { return false }
 for _,r:=range id {if r<'0'||r>'9'{return false}}
 return true
}
func readLaunchSession(path string)(launchSessionState,error){
 var s launchSessionState
 b,err:=os.ReadFile(path); if err!=nil{return s,err}
 if len(b)>32768 { return s,fmt.Errorf("launch metadata too large") }
 if err=json.Unmarshal(b,&s);err!=nil{return s,err}
 if s.PID<=0||!validLaunchShiftID(s.Trip.ShiftID){return s,fmt.Errorf("invalid launch metadata")}
 return s,nil
}
func writeLaunchSession(path string,trip TripInfo)error{
 if !validLaunchShiftID(trip.ShiftID){return fmt.Errorf("missing numeric shift ID")}
 b,err:=json.Marshal(launchSessionState{os.Getpid(),trip});if err!=nil{return err}
 return writeManagedFileAtomic(path,b)
}
func clearLaunchSession(path string){
 s,err:=readLaunchSession(path)
 if err==nil&&s.PID==os.Getpid(){_ = os.Remove(path)}
}
