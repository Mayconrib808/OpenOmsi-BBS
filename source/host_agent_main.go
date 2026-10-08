package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func init() { companyRuntimePackageChecksDisabled = true }
func main() {
	executable, _ := os.Executable()
	dir := filepath.Dir(executable)
	var background, smoke bool
	var path string
	flag.BoolVar(&background, "background", false, "Run the host agent without the configuration window")
	flag.BoolVar(&smoke, "gui-smoke", false, "Create and verify native controls without changing user settings")
	flag.StringVar(&path, "config", hostAgentSettingsPath(dir), "Private host settings file")
	flag.Parse()
	if !background {
		if err := runHostAgentGUI(dir, path, smoke); err != nil {
			if smoke {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			hostAgentShowError(err)
		}
		return
	}
	log, err := openHostAgentLog(dir)
	if err != nil {
		return
	}
	defer log.Close()
	config, err := loadHostAgentConfig(path)
	if err != nil {
		fmt.Fprintln(log, err)
		return
	}
	hostUseBundledServer(&config, dir)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runHostAgent(ctx, config, dir, log); err != nil {
		fmt.Fprintln(log, err)
	}
}
