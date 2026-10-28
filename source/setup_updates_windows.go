//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

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
	notes := r.Notes[normalizeLanguage(g.config.Language)]
	if notes == "" {
		notes = r.Notes["pt"]
	}
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
