package main
import("os";"path/filepath";"testing")
func TestSameLaunchTripOnlySuppressesIdenticalRetry(t *testing.T){
 a:=TripInfo{ShiftID:"4930868",MapName:"Map",Line:"522",Tour:"1 (ni-sr)",TripStart:"01:40",TripEnd:"01:48",RouteText:"Kormoranow - Kollataja"}
 b:=a;b.BlockTime="later";if !sameLaunchTrip(a,b){t.Fatal("retry must match despite timestamp")}
 for _,change:=range []func(*TripInfo){func(x *TripInfo){x.ShiftID="4930869"},func(x *TripInfo){x.MapName="other"},func(x *TripInfo){x.Line="other"},func(x *TripInfo){x.Tour="2"},func(x *TripInfo){x.TripStart="02:40"},func(x *TripInfo){x.TripEnd="02:48"},func(x *TripInfo){x.RouteText="other"},func(x *TripInfo){x.ShiftID=""}} {
  b=a;change(&b);if sameLaunchTrip(a,b){t.Fatal("different or incomplete trip suppressed",b)}
 }
}
func TestLaunchSessionStateOwnedCleanup(t *testing.T){
 path:=filepath.Join(t.TempDir(),"sessions","current.json")
 a:=TripInfo{ShiftID:"4930868"}
 if err:=writeLaunchSession(path,a);err!=nil{t.Fatal(err)}
 s,err:=readLaunchSession(path);if err!=nil||s.PID!=os.Getpid()||s.Trip.ShiftID!=a.ShiftID{t.Fatal(s,err)}
 if err:=writeManagedFileAtomic(path,[]byte("{\"PID\":1,\"Trip\":{\"ShiftID\":\"4930869\"}}"));err!=nil{t.Fatal(err)}
 clearLaunchSession(path);if _,err:=os.Stat(path);err!=nil{t.Fatal("other owner removed")}
 if err:=writeLaunchSession(path,a);err!=nil{t.Fatal(err)}
 clearLaunchSession(path);if _,err:=os.Stat(path);!os.IsNotExist(err){t.Fatal("own state not removed",err)}
}
