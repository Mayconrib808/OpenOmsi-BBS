package main

import (
	"bytes"
 "errors"
 "os"
 "path/filepath"
	"io"
	"testing"
)

func TestRealSpawnOutputAlsoReachesBCSGameLog(t *testing.T) {
	var diagnostics, bcs bytes.Buffer
	out := gameLogOutput(&diagnostics, &bcs)
	partial := "[2026-10-12T04:07:06Z INFO openomsi_game::spawn] spawned "
	io.WriteString(out, partial)
	if bytes.Contains(bcs.Bytes(), []byte("] spawned at")) {
		t.Fatal("premature readiness")
	}
	io.WriteString(out, "at entry point 7\n")
	if !bytes.Equal(diagnostics.Bytes(), bcs.Bytes()) || !bytes.Contains(bcs.Bytes(), []byte("] spawned at")) {
		t.Fatal("BCS cannot observe real spawn")
	}
}

type invalidGameOutput struct { calls int }
func (w *invalidGameOutput)Write([]byte)(int,error){w.calls++;return 0,errors.New("write /dev/stdout: The handle is invalid.")}
func TestBCSGameLogContinuesAfterInvalidInheritedHandle(t *testing.T) {
 var diagnostics,game bytes.Buffer
 invalid:=&invalidGameOutput{}
 out:=gameLogOutput(&diagnostics,&game,invalid)
 first:="[INFO] openOMSI 0.2.37\n"
 ready:="[INFO plugin] [BBS bridge] spawned at multiplayer player vehicle ready\n"
 for _,line:=range []string{first,ready}{
  if n,err:=io.WriteString(out,line);err!=nil||n!=len(line){t.Fatal("valid disk logs stopped with inherited stdout",n,err)}
 }
 if diagnostics.String()!=first+ready || game.String()!=first+ready {t.Fatal("later readiness log was lost")}
 if invalid.calls!=1 {t.Fatal("invalid inherited handle repeatedly used",invalid.calls)}
}
func TestBCSGameLogSurvivesInvalidPrimaryWhenDiskMirrorIsValid(t *testing.T) {
 var game bytes.Buffer
 out:=gameLogOutput(&invalidGameOutput{},&game)
 if _,err:=io.WriteString(out,"real output");err!=nil||game.String()!="real output"{t.Fatal(game.String(),err)}
}
func TestBCSGameLogRefusesOldReadinessAtNewLaunch(t *testing.T) {
 path:=filepath.Join(t.TempDir(),".openomsi","game.log")
 _ = os.MkdirAll(filepath.Dir(path),0755)
 _ = os.WriteFile(path,[]byte("[old trip] spawned at old bus\n"),0600)
 f,err:=openBCSGameLog(path);if err!=nil{t.Fatal(err)}
 _ = f.Close()
 if b,err:=os.ReadFile(path);err!=nil||len(b)!=0{t.Fatal("old trip readiness was reused",string(b),err)}
}
