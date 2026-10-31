//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The installed Setup knows its own signed publication sequence, even when
// the previous release never displayed its changelog. Select ONLY the release
// milestones newer than that sequence; do not show the same Hotfix 1 notes to
// someone who is already running Hotfix 1.
func notesSinceInstalledSequence(lang string, installed uint64, target bridgeRelease) string {
    if target.Sequence < 2026103003 || target.Version != "2.2.2-hotfix.2" {
        return localReleaseNotes(lang,target)
    }
    var items []string
    if installed < 2026102515 {
        items = append(items,localText(lang,
            "2.2.0.8 e anteriores: evolução dos servidores multiplayer, conexão da empresa, sincronização de viagens e correções do Setup e do atualizador.",
            "2.2.0.8 and earlier: multiplayer server evolution, company connections, trip synchronization and Setup/updater repairs.",
            "2.2.0.8 und früher: Weiterentwicklung der Mehrspieler-Server, Firmenverbindungen, Fahrtsynchronisierung und Setup-/Update-Reparaturen."))
    }
    if installed < 2026102602 {
        items = append(items,localText(lang,
            "2.2.1–2.2.1.5: painel cooperativo, cobrador, troca de motorista, gestos e guarda-chuvas dos passageiros; controles do host e correções de portas e interação.",
            "2.2.1–2.2.1.5: coop panel, conductor, driver handover, passenger gestures and umbrellas; host controls and door/interaction fixes.",
            "2.2.1–2.2.1.5: Koop-Anzeige, Schaffner, Fahrerwechsel, Fahrgastgesten und Regenschirme, Hoststeuerung sowie Tür-/Interaktionskorrekturen."))
    }
    if installed < 2026102702 {
        items = append(items,localText(lang,
            "2.2.1.6–2.2.1.8: melhorias na transferência de direção, posição/rota dos passageiros, cobranças de bilhetes, sincronização de veículos e servidor P15.",
            "2.2.1.6–2.2.1.8: driver transfer, passenger position/route fixes, ticket collection, vehicle synchronization and P15 server improvements.",
            "2.2.1.6–2.2.1.8: Fahrerübernahme, Fahrgastpositionen und Routen, Ticketzahlung, Fahrzeugsynchronisierung und P15-Server."))
    }
    if installed < 2026102901 {
        items = append(items,localText(lang,
            "2.2.1.9 e suas hotfixes: OpenOMSI modificado 0.3.45, atualização manual/automática, suporte a versões e idiomas diferentes do BCS, correções do PeDePe e ajustes em HostAgent/CompanyHost.",
            "2.2.1.9 and hotfixes: modified openOMSI 0.3.45, manual/automatic updates, support for different BCS versions and languages, PeDePe fixes and HostAgent/CompanyHost refinements.",
            "2.2.1.9 und Hotfixes: angepasstes openOMSI 0.3.45, manuelle/automatische Updates, verschiedene BBS-Versionen und Sprachen, PeDePe-Korrekturen sowie HostAgent-/CompanyHost-Anpassungen."))
    }
    if installed < 2026103001 {
        items = append(items,localText(lang,
            "2.2.2: eventos visuais do BCS, fiscais no ônibus, veículos de emergência, tráfego multiplayer e embarque mais flexível; OpenOMSI 0.3.46/P15.",
            "2.2.2: visual BCS incidents, onboard inspectors, emergency vehicles, multiplayer traffic and more flexible boarding; openOMSI 0.3.46/P15.",
            "2.2.2: visuelle BBS-Ereignisse, Kontrolleur im Bus, Einsatzfahrzeuge, Mehrspieler-Verkehr und flexiblerer Einstieg; openOMSI 0.3.46/P15."))
    }
    if installed < 2026103002 {
        items = append(items,localText(lang,
            "Hotfix 1: patrulha do fiscal, saída traseira em ônibus personalizados, escala dos carros de emergência, resposta de 180 s e distribuição de carros para convidados.",
            "Hotfix 1: inspector aisle patrol, custom-bus rear exit, emergency-vehicle scale, 180 s response window and improved guest traffic.",
            "Hotfix 1: Kontrollgang, Hintertür von Zusatzbussen, Größe der Einsatzfahrzeuge, 180-s-Einsätze und Verkehr bei Gästen."))
    }
    if installed < 2026103003 {
        items = append(items,localText(lang,
            "Hotfix 2: desembarque por portas realmente abertas, parada correta, retomada do caminho sem teleporte, fiscal sem repetição infinita, tolerância na parada e diagnóstico de rotas e tráfego.",
            "Hotfix 2: alighting through physically open exits, correct stop matching, path recovery without teleporting, inspector loop fix, boarding allowance and better routing/traffic diagnostics.",
            "Hotfix 2: Ausstieg durch tatsächlich offene Türen, richtige Haltestelle, Wegkorrektur ohne Teleport, Kontrolleur-Schleife behoben, toleranter Einstieg und bessere Diagnose."))
    }
    if len(items)==0 {return localReleaseNotes(lang,target)}
    return strings.Join(items,"\r\n\r\n")
}

func localReleaseNotes(lang string, release bridgeRelease) string {
    s:=release.Notes[normalizeLanguage(lang)]
    if s=="" {s=release.Notes["pt"]}
    return s
}

type setupUpdateResult struct {
	release *bridgeRelease
	err     error
}

func (g *setupGUI) checkUpdates() {
	if g.updateChecking {
		return
	}
	g.updateChecking = true
	g.updateState = "updates_checking"
	g.renderUpdateState()
	guiUser.NewProc("EnableWindow").Call(g.updateButton, 0)
	ctx, cancel := context.WithCancel(context.Background())
	g.updateCancel = cancel
	go func() {
		release, err := checkBridgeRelease(ctx, bridgeUpdateClient(), bridgeUpdateOrigin)
		g.updateResults <- setupUpdateResult{release, err}
		guiUser.NewProc("PostMessageW").Call(g.window, guiWMUpdates, 0, 0)
	}()
}

func (g *setupGUI) finishUpdateCheck() {
	r := <-g.updateResults
	g.updateChecking = false
	if g.updateCancel != nil {
		g.updateCancel()
		g.updateCancel = nil
	}
	g.availableUpdate = r.release
	g.updateState = "updates_current"
	if errors.Is(r.err, errUpdateNotConfigured) {
		g.updateState = "updates_unconfigured"
	} else if r.err != nil {
		g.updateState = "updates_unavailable"
		logSetup(g.dir, "check-updates", r.err)
	} else if r.release != nil {
		g.updateState = "updates_available"
	}
	g.renderUpdateState()
	if !g.busy {
		guiUser.NewProc("EnableWindow").Call(g.updateButton, 1)
	}
}

func (g *setupGUI) updateCommand() {
	if g.availableUpdate == nil {
		g.checkUpdates()
		return
	}
	// Updating must work before activation has saved the form.
	root, err := cleanInputPath(guiText(g.root))
	if err != nil {
		guiMessageBox(g.window, err.Error(), 0x10)
		return
	}
	r := *g.availableUpdate
	notes := notesSinceInstalledSequence(g.config.Language,bridgeReleaseSequence(),r)
	if guiMessageBox(g.window, fmt.Sprintf(g.text("updates_confirm"), r.Version)+"\r\n\r\n"+notes, 0x24) != 6 {
		return
	}
	g.work(g.text("updates_downloading"), func(ctx context.Context) guiResult {
		if err := setupChangesAllowed(g.config.Language); err != nil {
			return guiResult{err: err}
		}
		jar, err := installedBCSJar(root)
		if err != nil {
			return guiResult{err: err}
		}
		if jar != "" {
			if err = checkBCSClosedForUpdate(jar); err != nil {
				return guiResult{err: err}
			}
		}
		if err = checkUpdateHostClosed(g.dir); err != nil {
			return guiResult{err: err}
		}
		// Integrity drift in the old installation is a reason to repair it, not
		// a reason to deny the signed updater. Keep it in the log for diagnosis
		// but let the SHA-256/signature-verified update continue.
		if _, integrityErr := verifyPackage(g.dir); integrityErr != nil {
			logSetup(g.dir, "update-integrity-warning", integrityErr)
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return guiResult{err: err}
		}
		path, err := downloadBridgeRelease(ctx, bridgeUpdateClient(), r.Installer, filepath.Join(cache, "OpenOmsi-BBS", "Updates"))
		return guiResult{err: err, updateInstaller: path, updateHash: r.Installer.SHA256}
	})
}

func launchSetupUpdate(dir, path, hash string) error {
	if err := verifyDownloadedBridgeRelease(path, hash); err != nil {
		return err
	}
	return startBridgeUpdateProcess(path, filepath.Dir(path), "--target-dir", dir, "--wait-setup", strconv.Itoa(os.Getpid()), "--reopen")
}

func (g *setupGUI) renderUpdateState() {
	if g.updateStatus == 0 || g.updateButton == 0 {
		return
	}
	state := g.updateState
	if state == "" {
		state = "updates_checking"
	}
	text, button := g.text(state), g.text("updates_check")
	if g.availableUpdate != nil && !g.updateChecking {
		text = fmt.Sprintf(g.text("updates_available"), g.availableUpdate.Version)
		button = g.text("updates_install")
	}
	guiSetText(g.updateStatus, text)
	guiSetText(g.updateButton, button)
}

func (g *setupGUI) coopBackupCommand(restore bool) {
	if restore && guiMessageBox(g.window, g.text("coop_restore_confirm"), 0x24) != 6 {
		return
	}
	work, done := "coop_backup_work", "coop_backup_done"
	if restore {
		work, done = "coop_restore_work", "coop_restore_done"
	}
	g.work(g.text(work), func(ctx context.Context) guiResult {
		if err := setupChangesAllowed(g.config.Language); err != nil {
			return guiResult{err: err}
		}
		if err := checkUpdateHostClosed(g.dir); err != nil {
			return guiResult{err: err}
		}
		jar, err := installedBCSJar(g.config.Root)
		if err != nil {
			return guiResult{err: err}
		}
		if jar != "" {
			if err = checkBCSClosedForUpdate(jar); err != nil {
				return guiResult{err: err}
			}
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return guiResult{err: err}
		}
		path := filepath.Join(cache, "OpenOmsi-BBS", "Backups", "Coop-"+bridgeVersion+"-"+bundledCoopDistribution.SHA256+".zip")
		if restore {
			err = restoreCoopBackup(g.dir, path, bundledCoopDistribution)
		} else {
			err = createCoopBackup(g.dir, path, bundledCoopDistribution)
		}
		return guiResult{err: err, status: g.text(done)}
	})
}
