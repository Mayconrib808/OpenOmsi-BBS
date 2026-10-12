package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var previousV3Adapters = map[string]bool{
	"41e69fa360ac2e8c03626047efef4e4fc9b173ba92ddcdfadd24683659395d40": true,
	"4d43dd25421d9435737c23d81706412c9b2e24437eb22b7eb1fed32acdeacc2c": true,
	"50a497c5539981176a4f5d9dcef5b8c81877d01834adfc9db525fac56c42cc1c": true,
	"58ddb14fcdf9348ec11eb2fec6b0aabb7183c8048591b2018e8b6fff0a469eaf": true,
	"ebaa78c18740068b1138f41d2341804ae31f1340e2e473f8b5dd7c8ff8fb17ea": true,
}

func knownPreviousV3Adapter(dir string) bool {
	if !filepath.IsAbs(dir) || !strings.EqualFold(filepath.Base(filepath.Clean(dir)), "PeDePeAdapter") || !strings.EqualFold(filepath.Base(filepath.Dir(filepath.Clean(dir))), "OpenOMSI-BBS") {
		return false
	}
	b, e := readSmallFile(filepath.Join(dir, "openomsi.exe"), 16<<20)
	if e != nil {
		return false
	}
	h := fmt.Sprintf("%x", sha256.Sum256(b))
	if previousV3Adapters[h] {
		return true
	}
	exe, e := os.Executable()
	if e != nil {
		return false
	}
	own, e := readSmallFile(exe, 16<<20)
	return e == nil && fmt.Sprintf("%x", sha256.Sum256(own)) == h
}
