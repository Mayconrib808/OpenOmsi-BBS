package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func peDePeAdapterLog(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\r\n", time.Now().Format(time.RFC3339), text)
}

func peDePeAdapterFail(logPath, language, message string) {
	peDePeAdapterLog(logPath, "ERROR: "+message)
	peDePeAdapterShowError(message)
}

func peDePeRealExecutable(c Config, packageDir string) (string, error) {
	real := filepath.Clean(strings.TrimSpace(c.OpenOMSI))
	if !filepath.IsAbs(real) || !strings.EqualFold(filepath.Base(real), "openomsi.exe") || !fileExists(real) {
		return "", fmt.Errorf("%s", localText(c.Language,
			"Configure o caminho do openOMSI real no Setup.exe da 2.1.0 antes de selecionar o adaptador no BBS.",
			"Configure the real openOMSI path in the 2.1.0 Setup.exe before selecting the adapter in BBS.",
			"Konfiguriere zuerst den echten openOMSI-Pfad in Setup.exe 2.1.0, bevor du den Adapter in BBS auswählst."))
	}
	self, err := os.Executable()
	if err == nil && samePath(self, real) {
		return "", fmt.Errorf("the PeDePe adapter points to itself; choose the real openOMSI executable in Setup")
	}
	adapter := filepath.Join(packageDir, "PeDePeAdapter", "openomsi.exe")
	if samePath(adapter, real) {
		return "", fmt.Errorf("the PeDePe adapter cannot be configured as the real openOMSI executable")
	}
	return real, nil
}

func main() {
	packageDir := packageRoot()
	logPath := filepath.Join(appDir(packageDir), "pedepe-native-v2.1.log")
	cfg := readInstalledConfig(packageDir)
	args := append([]string(nil), os.Args[1:]...)
	peDePeAdapterLog(logPath, "PeDePe native adapter v"+bridgeVersion+" argv="+fmt.Sprintf("%q", args))

	real, err := peDePeRealExecutable(cfg, packageDir)
	if err != nil {
		peDePeAdapterFail(logPath, cfg.Language, err.Error())
		os.Exit(2)
	}

	plan, err := peDePeCompanyPlan(context.Background(), cfg, packageDir, args)
	if err != nil {
		peDePeAdapterFail(logPath, cfg.Language, localText(cfg.Language,
			"O multiplayer da empresa não ficou pronto. A viagem não será aberta como single-player para evitar um registro incorreto no BBS.\n",
			"Company multiplayer is not ready. The trip will not be opened as single-player to avoid an incorrect BBS record.\n",
			"Der Firmen-Multiplayer ist nicht bereit. Die Fahrt wird nicht als Einzelspieler gestartet, damit BBS keinen falschen Datensatz speichert.\n")+err.Error())
		os.Exit(3)
	}
	freshArgs, err := peDePeFreshTripArgs(args, cfg.Root)
	if err != nil {
		peDePeAdapterFail(logPath, cfg.Language, "BBS fresh-trip conversion: "+err.Error())
		os.Exit(6)
	}
	launchArgs := peDePeForwardArgs(freshArgs, plan)
	launchArgs, err = peDePeDriverArgs(launchArgs, cfg.Root)
	if err != nil {
		peDePeAdapterFail(logPath, cfg.Language, "BBS driver path: "+err.Error())
		os.Exit(7)
	}
	peDePeAdapterLog(logPath, "adapter revision=situation-fresh-trip-driver-2 outgoing argv="+fmt.Sprintf("%q", launchArgs))
	if plan != nil {
		peDePeAdapterLog(logPath, fmt.Sprintf("company multiplayer: %s / %s / %s", plan.CompanyID, plan.Session.ID, plan.Session.ServerURL))
	} else if cfg.Multiplayer {
		in := parsePeDePeNativeInvocation(args)
		if in.HasLANJoin {
			peDePeAdapterLog(logPath, "incoming launch already contains --lan-join; native/future BBS multiplayer kept unchanged")
		} else {
			peDePeAdapterLog(logPath, "launch is not a complete PeDePe trip; forwarding without company multiplayer")
		}
	}

	cmd := exec.Command(real, launchArgs...)
	cmd.Dir = cfg.Root
	if root, ok := pedepeArgValue(args, "--root"); ok && filepath.IsAbs(root) {
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			cmd.Dir = root
		}
	}
	if cmd.Dir == "" {
		cmd.Dir = filepath.Dir(real)
	}
	cmd.Env = os.Environ()
	var contentDir string
	if plan != nil {
		contentDir, err = os.MkdirTemp("", "openomsi-bbs-pedepe-")
		if err != nil {
			peDePeAdapterFail(logPath, cfg.Language, err.Error())
			os.Exit(4)
		}
		defer os.RemoveAll(contentDir)
		cmd.Env = multiplayerEnvironment(cmd.Env, contentDir)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	peDePeAdapterLog(logPath, "forwarding to real openOMSI: "+real)
	if err = cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			peDePeAdapterLog(logPath, fmt.Sprintf("real openOMSI exit code=%d", exit.ExitCode()))
			os.Exit(exit.ExitCode())
		}
		peDePeAdapterFail(logPath, cfg.Language, "openOMSI: "+err.Error())
		os.Exit(5)
	}
	peDePeAdapterLog(logPath, "real openOMSI exited normally")
}
