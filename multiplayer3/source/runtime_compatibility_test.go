package main
import "testing"
func TestProtocol7EraRejectsSkippedAndOlderVersions(t *testing.T){
 for _,v:=range []string{"0.2.20-bbs-free2","openOMSI 0.2.38","0.2.47","unknown"}{if protocol7EraVersion(v){t.Fatal("incompatible client accepted",v)}}
 for _,v:=range []string{"openOMSI 0.2.48","openOMSI 0.2.50 (build)","0.3.0"}{if !protocol7EraVersion(v){t.Fatal("current version rejected",v)}}
 s:=serverStatus{Map:"maps/grundorf/global.cfg",Protocol:6,Free:true,Vehicles:[]byte(`[]`)}
 if validStatus(s,s.Map)==nil{t.Fatal("server protocol 6 accepted")}
 s.Protocol=8;if validStatus(s,s.Map)==nil{t.Fatal("unknown protocol accepted")}
 s.Protocol=7;if validStatus(s,"maps/carrao city/global.cfg")==nil{t.Fatal("another map accepted")}
}
