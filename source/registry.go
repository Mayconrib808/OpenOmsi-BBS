package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const ifeoParent = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\Omsi.exe`
const ifeoBridge = ifeoParent + `\OpenOMSI_BCS_Bridge`
const regString = 1
const regDWORD = 4

type registryValue struct {
	Present bool
	Kind    uint32
	Data    []byte
}
type registryAPI interface {
	Read(view int, key, name string) (registryValue, error)
	Write(view int, key, name string, value registryValue) error
	DeleteValue(view int, key, name string) error
	DeleteKey(view int, key string) error
	Keys(view int, key string) ([]string, error)
}

func activateRegistryWithHost(r registryAPI, c Config, packageDir, bridge string) (pluginHostDeployment, error) {
	d, err := preparePluginHost(c, packageDir)
	if err != nil {
		return d, err
	}
	if err = activateRegistry(r, filepath.Join(c.Root, "Omsi.exe"), bridge); err != nil {
		if cleanupErr := rollbackPluginHost(d); cleanupErr != nil {
			err = fmt.Errorf("%w; helper rollback failed: %v", err, cleanupErr)
		}
		return d, err
	}
	return d, nil
}

func deactivateRegistryWithHost(r registryAPI, bridge string) error {
	roots := map[string]bool{}
	for _, view := range []int{64, 32} {
		owned, err := ownedBridge(r, view)
		if err != nil {
			return err
		}
		if owned {
			command, err := r.Read(view, ifeoBridge, "Debugger")
			if err != nil {
				return err
			}
			if command.text() != `"`+bridge+`"` {
				return fmt.Errorf("bridge belongs to another package folder; use its configurator to deactivate it")
			}
			original, err := r.Read(view, ifeoBridge, "FilterFullPath")
			if err != nil {
				return err
			}
			if filepath.IsAbs(original.text()) && strings.EqualFold(filepath.Base(original.text()), "Omsi.exe") {
				roots[filepath.Dir(original.text())] = true
			}
		}
	}
	if err := deactivateRegistry(r, bridge); err != nil {
		return err
	}
	for root := range roots {
		state, err := removeInstalledPluginHost(root)
		if err != nil {
			return fmt.Errorf("bridge deactivated; helper cleanup failed: %w", err)
		}
		if state == "preserved" {
			fmt.Println("Auxiliar diferente do pacote: preservado na pasta do OMSI. / Helper differs from the package: preserved in the OMSI folder.")
		}
	}
	return nil
}

func stringValue(s string) registryValue {
	u := append(utf16.Encode([]rune(s)), 0)
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	return registryValue{true, regString, b}
}
func dwordValue(n uint32) registryValue {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return registryValue{true, regDWORD, b}
}
func (v registryValue) text() string {
	if !v.Present || v.Kind != regString || len(v.Data)%2 != 0 {
		return ""
	}
	u := make([]uint16, len(v.Data)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(v.Data[2*i:])
	}
	return strings.TrimRight(string(utf16.Decode(u)), "\x00")
}
func (v registryValue) number() (uint32, bool) {
	if !v.Present || v.Kind != regDWORD || len(v.Data) != 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(v.Data), true
}
func legacyBridgeCommand(s string) bool {
	// Legacy installers used one quoted executable path and no extra arguments.
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return false
	}
	s = s[1 : len(s)-1]
	if strings.Contains(s, "\"") {
		return false
	}
	i := strings.LastIndexAny(s, `\/`)
	return i >= 0 && strings.EqualFold(s[i+1:], "OpenOMSI_BCS_Bridge.exe")
}
func ownedBridge(r registryAPI, view int) (bool, error) {
	d, e := r.Read(view, ifeoBridge, "Debugger")
	if e != nil {
		return false, e
	}
	o, e := r.Read(view, ifeoBridge, "BridgeAuthor")
	if e != nil {
		return false, e
	}
	if !d.Present {
		keys, e := r.Keys(view, ifeoParent)
		if e != nil {
			return false, e
		}
		for _, k := range keys {
			if strings.EqualFold(k, "OpenOMSI_BCS_Bridge") {
				return false, fmt.Errorf("existing bridge key has no recognized owner")
			}
		}
		return false, nil
	}
	if !legacyBridgeCommand(d.text()) || (o.Present && o.text() != bridgeAuthor) {
		return false, fmt.Errorf("bridge key belongs to another debugger; refusing to overwrite it")
	}
	return true, nil
}

type registryUndo struct {
	view      int
	key, name string
	value     registryValue
}

func setWithUndo(r registryAPI, undo *[]registryUndo, view int, key, name string, v registryValue) error {
	old, e := r.Read(view, key, name)
	if e != nil {
		return e
	}
	*undo = append(*undo, registryUndo{view, key, name, old})
	return r.Write(view, key, name, v)
}
func rollbackRegistry(r registryAPI, undo []registryUndo) error {
	var failures []string
	for i := len(undo) - 1; i >= 0; i-- {
		u := undo[i]
		var e error
		if u.value.Present {
			e = r.Write(u.view, u.key, u.name, u.value)
		} else {
			e = r.DeleteValue(u.view, u.key, u.name)
		}
		if e != nil {
			failures = append(failures, e.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("rollback incomplete: %s", strings.Join(failures, "; "))
	}
	return nil
}

func activateRegistry(r registryAPI, original, bridge string) (result error) {
	command := `"` + bridge + `"`
	var undo []registryUndo
	var newViews []int
	defer func() {
		if result != nil {
			e := rollbackRegistry(r, undo)
			for _, view := range newViews {
				if cleanupErr := r.DeleteKey(view, ifeoBridge); cleanupErr != nil {
					result = fmt.Errorf("%w; bridge-key cleanup failed: %v", result, cleanupErr)
				}
			}
			if e != nil {
				result = fmt.Errorf("%w; %v", result, e)
			}
		}
	}()
	// Check both views before writing; preserve other tools' filters and global debuggers.
	for _, view := range []int{64, 32} {
		global, e := r.Read(view, ifeoParent, "Debugger")
		if e != nil {
			return e
		}
		if global.Present && !(global.Kind == regString && global.text() == "") {
			return fmt.Errorf("Omsi.exe already has a global debugger; deactivate it first")
		}
		owner, e := ownedBridge(r, view)
		if e != nil {
			return e
		}
		if owner {
			v, e := r.Read(view, ifeoBridge, "FilterFullPath")
			if e != nil {
				return e
			}
			if !samePath(v.text(), original) {
				return fmt.Errorf("bridge is active for another OMSI folder; deactivate that installation first")
			}
		}
		filter, e := r.Read(view, ifeoParent, "UseFilter")
		if e != nil {
			return e
		}
		if filter.Present {
			n, ok := filter.number()
			if !ok || n > 1 {
				return fmt.Errorf("unexpected UseFilter value; refusing to replace it")
			}
		}
		keys, e := r.Keys(view, ifeoParent)
		if e != nil {
			return e
		}
		for _, k := range keys {
			if strings.EqualFold(k, "OpenOMSI_BCS_Bridge") {
				continue
			}
			p, e := r.Read(view, ifeoParent+`\`+k, "FilterFullPath")
			if e != nil {
				return e
			}
			if p.Present && samePath(p.text(), original) {
				return fmt.Errorf("another filter targets this OMSI folder: %s", k)
			}
		}
	}
	for _, view := range []int{64, 32} {
		exists, e := ownedBridge(r, view)
		if e != nil {
			return e
		}
		if !exists {
			newViews = append(newViews, view)
		}
		prev, e := r.Read(view, ifeoBridge, "PreviousUseFilterPresent")
		if e != nil {
			return e
		}
		if !prev.Present {
			f, e := r.Read(view, ifeoParent, "UseFilter")
			if e != nil {
				return e
			}
			present := uint32(0)
			n := uint32(0)
			if f.Present {
				present = 1
				n, _ = f.number()
			}
			for _, entry := range []struct {
				name string
				v    registryValue
			}{{"PreviousUseFilterPresent", dwordValue(present)}, {"PreviousUseFilter", dwordValue(n)}} {
				if e = setWithUndo(r, &undo, view, ifeoBridge, entry.name, entry.v); e != nil {
					return e
				}
			}
		}
		for _, entry := range []struct {
			name string
			v    registryValue
		}{{"FilterFullPath", stringValue(original)}, {"BridgeAuthor", stringValue(bridgeAuthor)}, {"BridgeVersion", stringValue(bridgeVersion)}, {"Debugger", stringValue(command)}} {
			if e = setWithUndo(r, &undo, view, ifeoBridge, entry.name, entry.v); e != nil {
				return e
			}
		}
		if e = setWithUndo(r, &undo, view, ifeoParent, "UseFilter", dwordValue(1)); e != nil {
			return e
		}
	}
	return nil
}

func deactivateRegistry(r registryAPI, bridge string) error {
	for _, view := range []int{64, 32} {
		exists, e := ownedBridge(r, view)
		if e != nil {
			return e
		}
		if !exists {
			continue
		}
		d, e := r.Read(view, ifeoBridge, "Debugger")
		if e != nil {
			return e
		}
		if d.text() != `"`+bridge+`"` {
			return fmt.Errorf("bridge belongs to another package folder; use its configurator to deactivate it")
		}
	}
	// IFEO can be shared between the two views. Read the restoration values before deleting either view.
	type saved struct {
		view       int
		exists     bool
		present, n registryValue
	}
	var states []saved
	for _, view := range []int{64, 32} {
		exists, e := ownedBridge(r, view)
		if e != nil {
			return e
		}
		p, e := r.Read(view, ifeoBridge, "PreviousUseFilterPresent")
		if e != nil {
			return e
		}
		n, e := r.Read(view, ifeoBridge, "PreviousUseFilter")
		if e != nil {
			return e
		}
		states = append(states, saved{view, exists, p, n})
	}
	for _, s := range states {
		if s.exists {
			if e := r.DeleteKey(s.view, ifeoBridge); e != nil {
				return e
			}
		}
	}
	for _, s := range states {
		if !s.exists {
			continue
		}
		keys, e := r.Keys(s.view, ifeoParent)
		if e != nil {
			return e
		}
		if len(keys) > 0 {
			continue
		} // preserve other filters
		current, e := r.Read(s.view, ifeoParent, "UseFilter")
		if e != nil {
			return e
		}
		n, ok := current.number()
		if !ok || n != 1 {
			continue
		}
		had, known := s.present.number()
		if !known {
			continue
		} // legacy activation: leave the harmless filter flag alone
		if had == 0 {
			if e = r.DeleteValue(s.view, ifeoParent, "UseFilter"); e != nil {
				return e
			}
		} else if had == 1 {
			if _, ok := s.n.number(); !ok {
				return fmt.Errorf("invalid restoration value")
			}
			if e = r.Write(s.view, ifeoParent, "UseFilter", s.n); e != nil {
				return e
			}
		}
	}
	return nil
}
