package main

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

type latestClientTransport struct { t *testing.T; called bool }
func (r *latestClientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
 r.called = true
 if req.URL.String() != "https://api.github.com/repos/openOMSI-org/openOMSI/releases/latest" {
  r.t.Fatalf("client still requests a fixed release: %s", req.URL)
 }
 release := serverRelease{Tag:"v0.2.37", Assets:[]releaseAsset{{Name:"openOMSI-0.2.37-windows-x64.zip", URL:"https://github.com/openOMSI-org/openOMSI/releases/download/v0.2.37/openOMSI-0.2.37-windows-x64.zip", Digest:"sha256:"+strings.Repeat("a",64), Size:123}}}
 body, _ := json.Marshal(release)
 return &http.Response{StatusCode:200, Header:make(http.Header), Body:io.NopCloser(strings.NewReader(string(body))), Request:req},nil
}
func TestAutomaticClientUsesLatestOfficialReleaseAndVerifiedCache(t *testing.T) {
 dir := t.TempDir()
 for _, version := range []string{"0.2.29", "0.2.37"} {
  archive := makeRuntimeZip(t,map[string]string{"pkg/openomsi.exe":"client "+version,"pkg/steam_api64.dll":"steam","pkg/assets/font.bin":"resource"})
  dest := filepath.Join(dir,"client",version)
  receipt,err := unpackClient(archive,dest,version,strings.Repeat("a",64))
  if err!=nil { t.Fatal(err) }
  data,_:=json.Marshal(receipt)
  if err=os.WriteFile(filepath.Join(dest,"verified.json"),data,0600);err!=nil {t.Fatal(err)}
 }
 transport := &latestClientTransport{t:t}
 old := http.DefaultTransport
 http.DefaultTransport = transport
 defer func(){http.DefaultTransport=old}()
 exe,err:=resolveClient(context.Background(),dir)
 if err!=nil || !transport.called || exe!=filepath.Join(dir,"client","0.2.37","openomsi.exe") {
  t.Fatal("latest client not selected",exe,err,transport.called)
 }
 if got:=detectClient(t.TempDir(),dir);got!="" {
  t.Fatal("old cached version silently prefilled the optional field",got)
 }
}
func TestLatestClientRejectsPreviewAndUnverifiedDownloads(t *testing.T) {
 r:=serverRelease{Tag:"v0.2.37",Assets:[]releaseAsset{{Name:"openOMSI-0.2.37-windows-x64.zip",URL:"https://github.com/openOMSI-org/openOMSI/releases/download/v0.2.37/openOMSI-0.2.37-windows-x64.zip",Digest:"sha256:"+strings.Repeat("a",64),Size:123}}}
 if v,_,err:=runtimeReleaseSelection(r,"",true);err!=nil||v!="0.2.37" {t.Fatal(v,err)}
 r.Prerelease=true
 if _,_,err:=runtimeReleaseSelection(r,"",true);err==nil {t.Fatal("preview accepted")}
 r.Prerelease=false;r.Assets[0].Digest=""
 if _,_,err:=runtimeReleaseSelection(r,"",true);err==nil {t.Fatal("unverified download accepted")}
}
