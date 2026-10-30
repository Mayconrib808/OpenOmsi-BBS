//go:build windows

package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The seven Bridge executables are embedded. The approved Coop archive is
// downloaded and verified against the immutable hashes compiled into this EXE.
//
//go:embed bridge_update_payload.zip
var automaticBridgePayload []byte

func automaticUpdateMessage(text string, failed bool) {
	flags := uintptr(0x40)
	if failed {
		flags = 0x10
	}
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(winPtr(text))), uintptr(unsafe.Pointer(winPtr("Atualizar OpenOMSI BCS Bridge"))), flags)
}

func elevateAutomaticUpdate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	info := shellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: winPtr("runas"), File: winPtr(exe), Parameters: winPtr(automaticUpdateElevationArguments()), Directory: winPtr(filepath.Dir(exe)), Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	ole := syscall.NewLazyDLL("ole32.dll")
	initialized, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(initialized) >= 0 {
		defer ole.NewProc("CoUninitialize").Call()
	}
	ok, _, err := setupShell.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return err
	}
	if info.Process == 0 {
		return fmt.Errorf("não foi possível iniciar o atualizador")
	}
	defer syscall.CloseHandle(syscall.Handle(info.Process))
	_, err = syscall.WaitForSingleObject(syscall.Handle(info.Process), syscall.INFINITE)
	return err
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if len(os.Args) == 2 && os.Args[1] == "--coop-progress-smoke" {
		_, err := withCoopUpdateProgress("pt", func(ctx context.Context, progress func(int64, int64)) (map[string][]byte, error) {
			for i := int64(0); i <= 4; i++ {
				progress(i, 4)
				time.Sleep(100 * time.Millisecond)
			}
			return nil, nil
		})
		if err != nil {
			os.Exit(1)
		}
		return
	}
	opts, err := parseBridgeUpdateOptions(os.Args[1:])
	if err != nil {
		automaticUpdateMessage(err.Error(), true)
		return
	}
	saved := readConfig(filepath.Join(managedDataDir(exeDir()), "bridge.ini"))
	var dir string
	var c Config
	if opts.target != "" {
		// When Setup hands the updater its own directory, that directory is the
		// recovery target. Do not demand that the old installation is already
		// complete: missing adapter/config/binaries are exactly what an updater
		// must be able to repair.
		dir = filepath.Clean(opts.target)
		if !filepath.IsAbs(dir) || filepath.Dir(dir) == dir {
			err = fmt.Errorf("pasta de atualização inválida")
		} else {
			c = readInstalledConfig(dir)
			if !filepath.IsAbs(c.Root) && filepath.IsAbs(saved.Root) {
				c.Root = filepath.Clean(saved.Root)
			}
		}
	} else {
		dir, c, err = automaticBridgeUpdateTarget(newWindowsRegistry(), saved.Root, steamRootsForBridgeUpdate())
	}
	if err == nil {
		err = waitForSetupUpdate(uint32(opts.wait), dir)
	}
	if err == nil {
		err = checkUpdateHostClosed(dir)
	}
	if err == nil {
		var running bool
		running, err = gamesRunning()
		if err == nil && running {
			err = fmt.Errorf("feche BCS, OMSI e openOMSI antes de atualizar. Depois, execute este mesmo atualizador novamente")
		}
	}
	if err == nil {
		err = checkBridgeSetupClosedForUpdate(dir)
	}
	var payload map[string][]byte
	if err == nil {
		payload, err = readAutomaticBridgePayload(automaticBridgePayload)
	}
	if err == nil {
		var coop map[string][]byte
		coop, err = withCoopUpdateProgress(c.Language, func(ctx context.Context, progress func(int64, int64)) (map[string][]byte, error) {
			return loadBundledCoopUpdate(ctx, dir, progress)
		})
		if err == nil {
			for name, data := range coop {
				payload[name] = data
			}
		}
	}
	var state bcsNextTripPreparation
	if err == nil {
		state, err = installAutomaticBridgeUpdate(dir, c, payload, checkBCSClosedForUpdate)
	}
	if dir != "" {
		if f, e := os.OpenFile(filepath.Join(appDir(dir), "setup-v1.1.3.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); e == nil {
			fmt.Fprintf(f, "%s automatic-next-trip-update BCS=%s changed=%t result=%v\r\n", time.Now().Format(time.RFC3339), state.Path, state.Changed, err)
			f.Close()
		}
	}
	// Most Steam libraries are writable by their owner. Ask Windows for UAC
	// only when filesystem permissions actually require it; no registry writes
	// or company setup are part of this update.
	if err != nil && (errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.ERROR_ACCESS_DENIED)) && !isElevated() && !opts.apply {
		if elevationErr := elevateAutomaticUpdate(); elevationErr != nil && elevationErr != syscall.Errno(1223) {
			automaticUpdateMessage(elevationErr.Error(), true)
		}
		return
	}
	if err != nil {
		if errors.Is(err, errBridgeSetupStillOpen) {
			err = fmt.Errorf("%s", localText(c.Language,
				"O Setup desta instalação ainda está concluindo uma operação. Aguarde ou cancele essa operação e execute a atualização novamente.",
				"This installation's Setup is still completing an operation. Wait or cancel that operation, then run the update again.",
				"Das Setup dieser Installation führt noch einen Vorgang aus. Den Vorgang abwarten oder abbrechen und das Update erneut starten."))
		}
		if errors.Is(err, errBridgeSetupCloseFailed) {
			err = fmt.Errorf("%s", localText(c.Language,
				"Não foi possível solicitar o fechamento do Setup. Feche o Setup desta instalação e execute a atualização novamente.",
				"Could not request Setup to close. Close this installation's Setup and run the update again.",
				"Das Setup konnte nicht zum Schließen aufgefordert werden. Das Setup dieser Installation schließen und das Update erneut starten."))
		}
		automaticUpdateMessage(localText(c.Language, "Não foi possível concluir a atualização:\r\n", "Could not complete the update:\r\n", "Update fehlgeschlagen:\r\n")+strings.TrimSpace(err.Error()), true)
		os.Exit(1)
	}
	if opts.reopen || bridgeVersion == "2.2.2" {
		if e := startBridgeUpdateProcess(filepath.Join(dir, "Setup.exe"), dir); e != nil {
			automaticUpdateMessage(e.Error(), true)
		}
		return
	}
	automaticUpdateMessage(localText(c.Language,
		"Bridge atualizada!\r\n\r\nAbra o Setup/BCS e jogue normalmente. Se alguma configuração antiga estava faltando, o Setup pode recriá-la sem desfazer a atualização.\r\n\r\nSuas pastas, perfil da empresa e servidor foram preservados quando disponíveis.",
		"Bridge updated!\r\n\r\nOpen Setup/BCS and play normally. If an old setting was missing, Setup can recreate it without undoing the update.\r\n\r\nYour paths, company profile and server were preserved when available.",
		"Bridge aktualisiert!\r\n\r\nSetup/BBS öffnen und normal spielen. Fehlende alte Einstellungen können im Setup neu erstellt werden, ohne das Update rückgängig zu machen.\r\n\r\nPfade, Firmenprofil und Server bleiben erhalten, soweit vorhanden."), false)
}

func automaticUpdateElevationArguments() string {
	args := []string{"--apply"}
	for _, arg := range os.Args[1:] {
		if arg != "--apply" {
			args = append(args, arg)
		}
	}
	for i, arg := range args {
		args[i] = syscall.EscapeArg(arg)
	}
	return strings.Join(args, " ")
}
