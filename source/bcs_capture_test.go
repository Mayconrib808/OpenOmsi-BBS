package main

import (
 "archive/zip"
 "bytes"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
)

func TestBCSCaptureWrapperVerifiesAndPreservesLogging(t *testing.T){
 java,err:=exec.LookPath("java");if err!=nil{t.Fatal("JDK required for bytecode verification")}
 folder:=t.TempDir();pkg:=filepath.Join(folder,"bbs");os.MkdirAll(pkg,0700)
 source:=`package bbs;
public class f {
 private static String bridgeNextTripDetails(String s){return s==null?null:s+" [OpenOmsi-BBS next-trip log v2]";}
 public static String invoke(String s){return bridgeNextTripDetails(s);}
}`
 os.WriteFile(filepath.Join(pkg,"f.java"),[]byte(source),0600)
 check:=`public class Check {
 public static void main(String[] args) throws Exception {
  if(!"unchanged [OpenOmsi-BBS next-trip log v2]".equals(bbs.f.invoke("unchanged"))) throw new AssertionError("logger changed");
  if(bbs.f.invoke(null)!=null) throw new AssertionError("null changed");
  if(args.length>0 && !java.nio.file.Files.isRegularFile(java.nio.file.Paths.get(args[0],"OpenOmsi-BBS","EventosBeta","bcs-events-"+java.lang.management.ManagementFactory.getRuntimeMXBean().getName().split("@")[0]+".json"))) throw new AssertionError("capture did not start normally");
  System.out.println("PASS logger and startup");
 }
}`
 os.WriteFile(filepath.Join(folder,"Check.java"),[]byte(check),0600)
 cmd:=exec.Command(java,"--module","jdk.compiler/com.sun.tools.javac.Main","--release","8","-d",folder,filepath.Join(pkg,"f.java"),filepath.Join(folder,"Check.java"))
 if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("compile fixture: %v %s",err,out)}
 original,_:=os.ReadFile(filepath.Join(pkg,"f.class"))
 wrapped,err:=bcsCaptureLogger(original);if err!=nil{t.Fatal(err)}
 code:=func(data []byte)[]byte{var result []byte;inspectBCSClass(data,func(method string,at,n int){if strings.HasPrefix(method,"bridgeNextTripDetails("){result=append([]byte(nil),data[at:at+n]...)}});return result}
 if !bytes.Equal(code(original),code(wrapped)){t.Fatal("original observer method body changed")}
 twice,err:=bcsCaptureLogger(wrapped);if err!=nil||!bytes.Equal(twice,wrapped){t.Fatal("capture wrapper not idempotent",err)}
 os.WriteFile(filepath.Join(pkg,"f.class"),wrapped,0600)
 // Even a missing observer cannot interrupt the original logger.
 cmd=exec.Command(java,"-Xverify:all","-cp",folder,"Check")
 if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("missing observer fallback: %v %s",err,out)}
 entries,err:=bcsCaptureEntries(wrapped);if err!=nil{t.Fatal(err)}
 for name,data:=range entries {p:=filepath.Join(folder,filepath.FromSlash(name));os.MkdirAll(filepath.Dir(p),0700);os.WriteFile(p,data,0600)}
 local:=filepath.Join(folder,"profile");cmd=exec.Command(java,"-Xverify:all","-cp",folder,"Check",local);cmd.Env=append(os.Environ(),"LOCALAPPDATA="+local)
 if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("normal embedded capture: %v %s",err,out)}
}

func TestBCSCaptureArchiveUpgradeRetainsAllUnrelatedEntries(t *testing.T){
 path,sha:=testBCSJar(t)
 replacements,err:=bcsCaptureEntries([]byte("new owned logger"));if err!=nil{t.Fatal(err)}
 changed,err:=patchBCSJarEntries(path,sha,replacements);if err!=nil||!changed{t.Fatal(changed,err)}
 before,err:=zip.OpenReader(path+".antes-proxima-viagem.bak");if err!=nil{t.Fatal(err)};defer before.Close()
 if err:=verifyPatchedBCSEntries(path,before,replacements);err!=nil{t.Fatal(err)}
 changed,err=patchBCSJarEntries(path,sha,replacements);if err!=nil||changed{t.Fatal("repeat changed archive",changed,err)}
 // A new observer revision must replace its own classes, keeping prior backups.
 replacements["org/openomsi/bridge/events/Revision.class"]=[]byte("owned future revision")
 current,_:=bcsJarSHA(path);changed,err=patchBCSJarEntries(path,current,replacements)
 if err!=nil||!changed{t.Fatal("capture revision upgrade",changed,err)}
}
