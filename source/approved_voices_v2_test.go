package main

import (
    "crypto/sha256"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestApprovedBrazilianV2WAVsOnlyAndNoUnverifiedMutations(t *testing.T) {
    const prefix = "OpenOmsi Coop 0.3.41/programa/bcs-assets/PassengerVoices/pt-BR/"
    want := map[string]string{
        "ExitThanks_1.wav": "55d24033269764c01f34f5e584b737182c041dd37f3eb1e0bfc3c008e7c184e8",
        "ExitThanks_2.wav": "2481870d31818cfc47ed64be63e40b7085199d7b296db73348d47acd683728fb",
        "ExitThanks_3.wav": "7f5c625b5c7ef05c484e382e072d536dde59846be6b8d5b5366bfbbef3d4199b",
        "MissedStop_1.wav": "814a84c321db0aea89ac85c63eda68c808464941c69e7df37d33e2a869066890",
        "MissedStop_2.wav": "69b509b818714030e34b7f3cb9083d1b459cd19ca37961c154302700f83206b4",
        "OpenDoor_1.wav": "287ed8845e1d159e8659954bfcb5ec7c9e5488f2dcf864319d8efe266892628c",
        "OpenDoor_2.wav": "18e1e2afe67216a7859f69350c33f2d30db29431f21dffa180a1bd5c55eb4372",
        "OpenDoor_3.wav": "48aaf1808e202482cb8372800be8829235da7423aab82f9dd5c085d3101fb236",
    }
    if len(approvedPassengerVoiceV2Digests)!=len(want){t.Fatal("wrong approved V2 inventory")}
    for file,digest:=range want {
        name:=prefix+file
        if !approvedPassengerVoiceV2Digest(name,digest) {t.Fatal("rejected user-approved file",file)}
        if approvedPassengerVoiceV2Digest(name,strings.Repeat("0",64)) {t.Fatal("accepted modified WAV",file)}
        if approvedPassengerVoiceV2Digest(strings.Replace(name,"/pt-BR/","/en/",1),digest) {t.Fatal("wrong language accepted",file)}
        if approvedPassengerVoiceV2Digest(strings.Replace(name,"/programa/","/wrong/",1),digest) {t.Fatal("wrong folder accepted",file)}
        if approvedPassengerVoiceV2Digest(name+"/traversal",digest) {t.Fatal("nested path accepted",file)}
        if approvedPassengerVoiceV2Digests[file]!=digest {t.Fatal("unexpected approved WAV hash",file)}
    }
    if approvedPassengerVoiceV2Digest(prefix+"extra.wav",want["ExitThanks_1.wav"]) {t.Fatal("unexpected extra WAV accepted")}
    if approvedPassengerVoiceV2Digest("Setup.exe",want["ExitThanks_1.wav"]) {t.Fatal("binary hash bypass")}
}

func TestStrictManifestRejectsModifiedWAVAndIgnoresOnlyActivationAssets(t *testing.T) {
    dir:=t.TempDir()
    if err:=os.MkdirAll(filepath.Join(dir,"docs"),0755);err!=nil {t.Fatal(err)}
    files:=map[string][]byte{
        "Setup.exe":[]byte("MZ setup"),
        "app/OpenOMSI_BCS_Bridge.exe":[]byte("MZ bridge"),
        "app/compat/Omsi.exe":[]byte("MZ compat"),
        "app/compat/omsi-plugin-host32.exe":[]byte("MZ host"),
        "OpenOmsi Coop 0.3.41/programa/bcs-assets/PassengerVoices/pt-BR/ExitThanks_1.wav":[]byte("not the approved WAV"),
    }
    manifest:=""
    for name,b:=range files{
        full:=filepath.Join(dir,filepath.FromSlash(name))
        if err:=os.MkdirAll(filepath.Dir(full),0755);err!=nil {t.Fatal(err)}
        if err:=os.WriteFile(full,b,0644);err!=nil {t.Fatal(err)}
        expected:=fmt.Sprintf("%x",sha256.Sum256(b))
        if strings.HasSuffix(name,"ExitThanks_1.wav"){ expected=strings.Repeat("1",64) }
        manifest+=expected+"  "+name+"\n"
    }
    if err:=os.WriteFile(filepath.Join(dir,"docs","SHA256.txt"),[]byte(manifest),0644);err!=nil {t.Fatal(err)}
    if _,err:=verifyPackage(dir);err==nil {t.Fatal("unapproved edited WAV must fail strict manifest")}
    if _,err:=verifyPackageForActivation(dir);err!=nil {t.Fatal("activation should not depend on optional sound assets",err)}
}
