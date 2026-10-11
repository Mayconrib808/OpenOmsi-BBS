package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

type bridgeUpdateOptions struct {
	apply, reopen bool
	target        string
	wait          uint
}

func parseBridgeUpdateOptions(args []string) (bridgeUpdateOptions, error) {
	var o bridgeUpdateOptions
	f := flag.NewFlagSet("bridge-update", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.BoolVar(&o.apply, "apply", false, "")
	f.BoolVar(&o.reopen, "reopen", false, "")
	f.StringVar(&o.target, "target-dir", "", "")
	f.UintVar(&o.wait, "wait-setup", 0, "")
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() != 0 || uint64(o.wait) > 0xffffffff || (o.target != "" && !filepath.IsAbs(o.target)) || ((o.wait != 0 || o.reopen) && o.target == "") {
		return o, fmt.Errorf("argumentos inválidos")
	}
	return o, nil
}
