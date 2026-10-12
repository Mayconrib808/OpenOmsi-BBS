package main
import (
 "context"
 "fmt"
 "path/filepath"
 "testing"
)
func TestDefaultAutomaticClientAndServer(t *testing.T) {
 dir:=t.TempDir()
 v:=newSetupView(localConfig{},dir,"token")
 if v.ClientMode!="latest" || v.ServerMode!="latest" || v.Zone!="" || v.Offset!="" { t.Fatalf("unexpected setup defaults: %+v",v) }
 p,err:=normalizeServer(serverPreference{})
 if err!=nil||p.Mode!="latest" {t.Fatal(p,err)}
}
func TestAutomaticClientChecksBeforeEveryTripAndManualDoesNot(t *testing.T) {
 dir:=t.TempDir()
 c:=localConfig{ClientMode:"latest",Game:filepath.Join(dir,"client","0.2.29","openomsi.exe")}
 calls:=0
 resolver:=func(context.Context,string)(string,error){calls++;return filepath.Join(dir,"client","0.2.37","openomsi.exe"),nil}
 for n:=0;n<2;n++ {
  var err error;c,err=prepareConfiguredClient(context.Background(),c,dir,resolver)
  if err!=nil {t.Fatal(err)}
 }
 if calls!=2 || c.Game!=filepath.Join(dir,"client","0.2.37","openomsi.exe") {t.Fatal(c,calls)}
 c.ClientMode="manual"
 if _,err:=prepareConfiguredClient(context.Background(),c,dir,resolver);err!=nil||calls!=2 {t.Fatal(err,calls)}
}
func TestCompanyClockRequiresExplicitFieldsAndAllowsExplicitZero(t *testing.T) {
 for _,fields:=range [][2]string{{"",""},{"Europe/Berlin",""},{"","-8"},{"Europe/Berlin","NaN"}} {
  if _,err:=parseCompanyClockInput(fields[0],fields[1]);err==nil {t.Fatal("missing or invalid company clock accepted",fields)}
 }
 for _,hours:=range []string{"0","-8","-3"} {
  if _,err:=parseCompanyClockInput("Europe/Berlin",hours);err!=nil {t.Fatal(hours,err)}
 }
 c,err:=parseCompanyClockInput("Europe/Berlin","-8")
 if err!=nil||c.ShiftMinutes!=-480 {t.Fatal(c,err)}
}
func TestAutomaticServerFallbackIsLimitedToUnsupportedCapabilities(t *testing.T) {
 if !automaticServerFallback("latest",false,errServerCapabilities) {t.Fatal("automatic incompatible server cannot fall back")}
 for _,tc:=range []struct{mode string;tried bool;err error}{{"manual",false,errServerCapabilities},{"recommended",false,errServerCapabilities},{"latest",true,errServerCapabilities},{"latest",false,fmt.Errorf("wrong map")}} {
  if automaticServerFallback(tc.mode,tc.tried,tc.err) {t.Fatal("unsafe or repeated fallback",tc)}
 }
}
