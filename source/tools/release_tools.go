// Developer tools: validate the facade layout, or regenerate hashes after a local build.
package main

import (
	"bufio"
	"crypto/sha256"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func layout(finalPath, symbolPath string) error {
	final, e := pe.Open(finalPath)
	if e != nil {
		return e
	}
	defer final.Close()
	symbols, e := pe.Open(symbolPath)
	if e != nil {
		return e
	}
	defer symbols.Close()
	if final.Machine != pe.IMAGE_FILE_MACHINE_I386 {
		return fmt.Errorf("facade must be PE32 x86")
	}
	h, ok := final.OptionalHeader.(*pe.OptionalHeader32)
	if !ok || h.Subsystem != 2 {
		return fmt.Errorf("facade must be Windows GUI")
	}
	for _, name := range []string{".text", ".rdata", ".data"} {
		a, b := final.Section(name), symbols.Section(name)
		if a == nil || b == nil || a.VirtualAddress != b.VirtualAddress || a.VirtualSize != b.VirtualSize {
			return fmt.Errorf("final and symbol layouts differ at %s", name)
		}
	}
	for _, s := range symbols.Symbols {
		if s.Name != "main.legacyRvaBacking" {
			continue
		}
		if s.SectionNumber <= 0 || int(s.SectionNumber) > len(symbols.Sections) {
			return fmt.Errorf("invalid backing symbol")
		}
		section := symbols.Sections[s.SectionNumber-1]
		begin := uint64(section.VirtualAddress) + uint64(s.Value)
		end := begin + 4<<20
		if section.Characteristics&0x80000000 == 0 {
			return fmt.Errorf("backing is not writable")
		}
		for _, rva := range []uint64{0x461350, 0x4614f8, 0x461500} {
			if rva < begin || rva+4 > end {
				return fmt.Errorf("BCS RVA 0x%X outside backing [0x%X,0x%X)", rva, begin, end)
			}
		}
		fmt.Printf("PE32 GUI: all three BCS RVAs are inside writable backing [0x%X,0x%X).\n", begin, end)
		return nil
	}
	return fmt.Errorf("backing symbol missing; provide an unstripped build")
}
func hashes(dir string) error {
	list, e := os.Open(filepath.Join(dir, "docs", "PACKAGE_FILES.txt"))
	if e != nil {
		return e
	}
	defer list.Close()
	var output strings.Builder
	scan := bufio.NewScanner(list)
	for scan.Scan() {
		rel := scan.Text()
		if rel == "" || strings.HasPrefix(rel, "#") {
			continue
		}
		if strings.EqualFold(rel, "docs/SHA256.txt") || strings.ContainsAny(rel, `\:`) || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("unsafe package entry: %s", rel)
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		inside, e := filepath.Rel(dir, path)
		if e != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("entry outside package")
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		f.Close()
		if e != nil {
			return e
		}
		fmt.Fprintf(&output, "%x  %s\r\n", h.Sum(nil), rel)
	}
	if e = scan.Err(); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "docs", "SHA256.txt"), []byte(output.String()), 0644)
}
func main() {
	var e error
	if len(os.Args) == 4 && os.Args[1] == "layout" {
		e = layout(os.Args[2], os.Args[3])
	} else if len(os.Args) == 3 && os.Args[1] == "hashes" {
		e = hashes(os.Args[2])
	} else {
		e = fmt.Errorf("usage: release_tools layout FINAL SYMBOL_BUILD | hashes PACKAGE_DIRECTORY")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
