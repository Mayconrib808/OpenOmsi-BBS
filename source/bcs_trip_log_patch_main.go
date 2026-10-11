//go:build windows

package main

import (
	_ "embed"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Generated locally from the inspected user-supplied JAR, never committed.
//
//go:embed bcs_trip_log_patch.bin
var bcsTripLogger []byte

//go:embed bcs_trip_log_patch.sha256
var bcsTripOriginalSHA string

var bcsUser32 = syscall.NewLazyDLL("user32.dll")
var bcsComdlg32 = syscall.NewLazyDLL("comdlg32.dll")

func bcsPatchMessage(message string, flags uintptr) uintptr {
	title, _ := syscall.UTF16PtrFromString("OpenOmsi-BBS — comunicação da próxima viagem")
	value, _ := syscall.UTF16PtrFromString(message)
	r, _, _ := bcsUser32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(value)), uintptr(unsafe.Pointer(title)), flags)
	return r
}

type bcsOpenFileName struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefaultExtension             *uint16
	CustomData, Hook             uintptr
	TemplateName                 *uint16
	Reserved                     uintptr
	ReservedDWORD, FlagsEx       uint32
}

func bcsPatchChooseJar() string {
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("JAR do Bus Company Simulator\x00*.jar\x00\x00"))
	title, _ := syscall.UTF16PtrFromString("Selecione o busbetrieb-simulator.jar instalado no BCS")
	info := bcsOpenFileName{File: &buf[0], MaxFile: uint32(len(buf)), Filter: &filter[0], FilterIndex: 1, Title: title, Flags: 0x80000 | 0x1000 | 0x800 | 0x8}
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, _ := bcsComdlg32.NewProc("GetOpenFileNameW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func main() {
	if bcsPatchMessage("Feche completamente o BCS e o openOMSI antes de continuar.\n\nEste ajuste faz o BCS registrar no log a próxima viagem que você selecionou. O adaptador instalado poderá então atualizar a tabela.\n\nVocê selecionará o busbetrieb-simulator.jar da instalação do BCS. Será criado um backup do original.\n\nContinuar?", 0x24) != 6 {
		return
	}
	path := bcsPatchChooseJar()
	if path == "" {
		return
	}
	if bcsPatchMessage("Aplicar o ajuste neste arquivo?\n\n"+path+"\n\nSomente o logger será atualizado. O JAR precisa ser exatamente a versão analisada; outras versões serão preservadas.\n\nBackup: "+path+".antes-proxima-viagem.bak", 0x24) != 6 {
		return
	}
	changed, e := patchBCSJar(path, strings.TrimSpace(bcsTripOriginalSHA), bcsTripLogger)
	if e != nil {
		bcsPatchMessage(e.Error(), 0x10)
		return
	}
	if !changed {
		bcsPatchMessage("Esse ajuste já está instalado neste JAR.", 0x40)
		return
	}
	bcsPatchMessage("Ajuste instalado e integridade verificada. O original foi guardado no backup.\n\nAbra o BCS normalmente, inicie uma viagem e teste Próxima viagem após concluí-la.\n\nMantenha a atualização anterior de jogadores e host. O atualizador oficial do BCS pode substituir este ajuste; se isso ocorrer, não aplique em uma versão desconhecida.", 0x40)
}
