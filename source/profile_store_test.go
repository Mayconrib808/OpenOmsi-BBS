package main
import ("context";"encoding/json";"os";"path/filepath";"testing";"reflect")
func TestInstalledSettingsSurvivePackageUpdate(t *testing.T){
 t.Setenv("LOCALAPPDATA", t.TempDir())
 old, next := t.TempDir(), t.TempDir()
 c:=defaultConfig();c.Language="pt";c.Root=filepath.Join(t.TempDir(),"omsi");c.OpenOMSI=filepath.Join(t.TempDir(),"openomsi.exe")
 c.Multiplayer=true;c.CompanyID="company-a";c.CompanyProfile="saved.json";c.PlayerName="Maycon";c.CompanyProfileSource="original.json"
 if e:=saveInstalledConfig(old,c);e!=nil{t.Fatal(e)}
 if got:=readInstalledConfig(next);!reflect.DeepEqual(got,c){t.Fatalf("new package forgot settings: %+v",got)}
 other:=c;other.PlayerName="Second";if e:=writeManagedFileAtomic(configPath(next),encodeConfig(other));e!=nil{t.Fatal(e)}
 if got:=readInstalledConfig(next);got.PlayerName!="Second"{t.Fatal("explicit package settings lost")}
}
func TestManagedProfileSurvivesMissingOriginalAndRefreshesKnownSource(t *testing.T){
 t.Setenv("LOCALAPPDATA",t.TempDir());c,p,_,dir:=companyFixture(t);original:=saveTestCompany(t,dir,p)
 enrolled,_,err:=enrollCompany(context.Background(),dir,original,"Maycon",c);if err!=nil{t.Fatal(err)}
 if enrolled.CompanyProfile==original||enrolled.CompanyProfileSource!=original{t.Fatal("profile not imported")}
 p.Sessions[0].ServerURL="https://new-server.example.invalid"
 b,_:=json.Marshal(p);if err=os.WriteFile(original,b,0600);err!=nil{t.Fatal(err)}
 updated,err:=loadInstalledCompanyProfile(context.Background(),enrolled,dir,nil)
 if err!=nil||updated.Sessions[0].ServerURL!=p.Sessions[0].ServerURL{t.Fatalf("known source not refreshed: %v",err)}
 if err=os.Remove(original);err!=nil{t.Fatal(err)}
 cached,err:=loadInstalledCompanyProfile(context.Background(),enrolled,t.TempDir(),nil)
 if err!=nil||cached.Sessions[0].ServerURL!=p.Sessions[0].ServerURL{t.Fatalf("moved package lost profile: %v",err)}
}
func TestManagedProfileCannotAdoptChangedIdentityOrPlayerAssets(t *testing.T){
 t.Setenv("LOCALAPPDATA",t.TempDir());c,p,trip,dir:=companyFixture(t);original:=saveTestCompany(t,dir,p)
 enrolled,_,err:=enrollCompany(context.Background(),dir,original,"Maycon",c);if err!=nil{t.Fatal(err)}
 p.CompanyID="other-company";b,_:=json.Marshal(p);os.WriteFile(original,b,0600)
 if _,err=loadInstalledCompanyProfile(context.Background(),enrolled,dir,nil);err==nil{t.Fatal("changed identity accepted")}
 os.Remove(original);cached,err:=loadInstalledCompanyProfile(context.Background(),enrolled,dir,nil);if err!=nil{t.Fatal(err)}
 os.WriteFile(filepath.Join(c.Root,"Vehicles","A","script","main.osc"),[]byte("different script"),0600)
 if got:=checkCompanyPackages(c.Root,cached,cached.Sessions[0]);len(got)==0{t.Fatalf("import changed asset trust: %+v",trip)}
}

func TestManagedCompanyCanBeReactivatedAfterOriginalRemoval(t *testing.T) {
 packageDir:=t.TempDir()
 t.Setenv("LOCALAPPDATA",t.TempDir())
 source:=filepath.Join(packageDir,"company.json")
 _,p,_,_:=companyFixture(t)
 b,err:=json.Marshal(p);if err!=nil{t.Fatal(err)}
 if err=os.WriteFile(source,b,0600);err!=nil{t.Fatal(err)}
 c,_,err:=enrollCompany(context.Background(),packageDir,source,"First",Config{})
 if err!=nil{t.Fatal(err)}
 if err=os.Remove(source);err!=nil{t.Fatal(err)}
 c,got,err:=enrollCompany(context.Background(),packageDir,source,"Second",c)
 if err!=nil||got==nil||c.PlayerName!="Second"||c.CompanyID!=p.CompanyID{t.Fatal(c,got,err)}
 _,_,err=enrollCompany(context.Background(),packageDir,filepath.Join(packageDir,"different-missing.json"),"Third",c)
 if err==nil{t.Fatal("unknown missing administrator profile silently accepted")}
}
