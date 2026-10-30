package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
 "sort"
 "strings"
)

const bcsLoggerEntry = "bbs/f.class"

func bcsJarSHA(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Keep the original legacy backup name when possible. If a BCS update replaced
// the vendor JAR while an older Bridge backup is still beside it, preserve that
// backup and create a build-specific backup for the new vendor JAR instead.
func bcsPatchBackupPath(path, expectedSHA string) (string, error) {
	legacy := path + ".antes-proxima-viagem.bak"
	old, err := bcsJarSHA(legacy)
	if err == nil {
		if old == expectedSHA {
			return legacy, nil
		}
		prefix := expectedSHA
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		return path + ".antes-proxima-viagem." + prefix + ".bak", nil
	}
	if os.IsNotExist(err) {
		return legacy, nil
	}
	return "", err
}

// expectedSHA is a snapshot of this installed archive, not a BCS version gate.
// Detect concurrent changes, preserve other entries and keep a recoverable copy.
func patchBCSJar(path, expectedSHA string, logger []byte) (bool, error) {
 return patchBCSJarEntries(path,expectedSHA,map[string][]byte{bcsLoggerEntry:logger})
}

func patchBCSJarEntries(path, expectedSHA string, replacements map[string][]byte) (bool,error) {
 logger:=replacements[bcsLoggerEntry]
	if len(logger) == 0 {
		return false, fmt.Errorf("payload de comunicação ausente")
	}
	reader, e := zip.OpenReader(path)
	if e != nil {
		return false, fmt.Errorf("não foi possível ler o JAR: %w", e)
	}
	found, changed := false, false
 seen:=map[string]bool{}
 for _,entry:=range reader.File {
  if seen[entry.Name] {reader.Close();return false,fmt.Errorf("entrada duplicada no JAR: %s",entry.Name)}
  seen[entry.Name]=true
  upper:=strings.ToUpper(entry.Name)
  if strings.HasPrefix(upper,"META-INF/")&&(strings.HasSuffix(upper,".SF")||strings.HasSuffix(upper,".RSA")||strings.HasSuffix(upper,".DSA")||strings.HasSuffix(upper,".EC")) {
   reader.Close();return false,fmt.Errorf("o JAR assinado foi preservado")
  }
  if entry.Name==bcsLoggerEntry {found=true}
  if want,ok:=replacements[entry.Name];ok {
   r,err:=entry.Open();if err!=nil {reader.Close();return false,err}
   data,err:=io.ReadAll(io.LimitReader(r,1<<20));r.Close()
   if err!=nil {reader.Close();return false,err}
   if !bytes.Equal(data,want){changed=true}
  }
 }
 if !found {reader.Close();return false,fmt.Errorf("este arquivo não contém o logger do BCS verificado")}
 for name:=range replacements {if !seen[name]{changed=true}}
 if !changed {reader.Close();return false,nil}
	actual, e := bcsJarSHA(path)
	if e != nil || actual != expectedSHA {
		reader.Close()
		return false, fmt.Errorf("o JAR do BCS mudou durante a preparação; tente novamente com o BCS fechado")
	}
	tmp, e := os.CreateTemp(filepath.Dir(path), ".bcs-next-trip-*.tmp")
	if e != nil {
		reader.Close()
		return false, e
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	writer := zip.NewWriter(tmp)
	if e = writer.SetComment(reader.Comment); e != nil {
		reader.Close()
		tmp.Close()
		return false, e
	}
	for _, entry := range reader.File {
		header := entry.FileHeader
		if payload,replace := replacements[entry.Name]; replace {
			header.CRC32, header.CompressedSize, header.CompressedSize64 = 0, 0, 0
			header.UncompressedSize, header.UncompressedSize64 = 0, 0
			w, err := writer.CreateHeader(&header)
			if err == nil {
				_, err = w.Write(payload)
			}
			if err != nil {
				reader.Close()
				tmp.Close()
				return false, err
			}
		} else {
			r, err := entry.OpenRaw()
			if err != nil {
				reader.Close()
				tmp.Close()
				return false, err
			}
			w, err := writer.CreateRaw(&header)
			if err == nil {
				_, err = io.Copy(w, r)
			}
			if err != nil {
				reader.Close()
				tmp.Close()
				return false, err
			}
		}
	}
	var additions []string
 for name:=range replacements {if !seen[name]{additions=append(additions,name)}}
 sort.Strings(additions)
 for _,name:=range additions {
  w,err:=writer.Create(name);if err==nil {_,err=w.Write(replacements[name])}
  if err!=nil {reader.Close();tmp.Close();return false,err}
 }
	if e = writer.Close(); e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		reader.Close()
		return false, e
	}
	// Verify every entry and CRC before allowing replacement.
	if e = verifyPatchedBCSEntries(tmpName, reader, replacements); e != nil {
		reader.Close()
		return false, e
	}
	reader.Close()
	backup, e := bcsPatchBackupPath(path, expectedSHA)
	if e != nil {
		return false, e
	}
	if old, err := bcsJarSHA(backup); err == nil {
		if old != expectedSHA {
			return false, fmt.Errorf("há um backup diferente neste caminho; ele foi preservado")
		}
	} else if os.IsNotExist(err) {
		if err = copyBCSBackup(path, backup); err != nil {
			return false, err
		}
		if old, err = bcsJarSHA(backup); err != nil || old != expectedSHA {
			return false, fmt.Errorf("o backup não corresponde ao JAR verificado; o original foi preservado")
		}
	} else {
		return false, err
	}
	if actual, e = bcsJarSHA(path); e != nil || actual != expectedSHA {
		return false, fmt.Errorf("o JAR mudou durante a preparação; atualização cancelada")
	}
	if e = os.Rename(tmpName, path); e != nil {
		return false, fmt.Errorf("não foi possível substituir o JAR. Feche completamente o BCS e tente novamente: %w", e)
	}
	return true, nil
}

func copyBCSBackup(source, destination string) error {
	r, e := os.Open(source)
	if e != nil {
		return e
	}
	defer r.Close()
	w, e := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(w, r)
	if e == nil {
		e = w.Sync()
	}
	ce := w.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(destination)
	}
	return e
}

func verifyPatchedBCSJar(path string, before *zip.ReadCloser, logger []byte) error {
 return verifyPatchedBCSEntries(path,before,map[string][]byte{bcsLoggerEntry:logger})
}
func verifyPatchedBCSEntries(path string,before *zip.ReadCloser,replacements map[string][]byte)error {
	after, e := zip.OpenReader(path)
	if e != nil {
		return e
	}
	defer after.Close()
	oldNames:=map[string]bool{};for _,entry:=range before.File {oldNames[entry.Name]=true}
 added:=0;for name:=range replacements {if !oldNames[name]{added++}}
 if len(after.File) != len(before.File)+added {
		return fmt.Errorf("a estrutura do JAR não foi preservada")
	}
	for i, entry := range after.File {
        var previous *zip.File
        if i<len(before.File){previous=before.File[i]}
        if previous!=nil && entry.Name != previous.Name {
			return fmt.Errorf("a ordem dos arquivos do JAR mudou")
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, r)
		r.Close()
		if err != nil {
			return fmt.Errorf("verificação CRC: %w", err)
		}
		if payload,replace:=replacements[entry.Name]; replace {
			want := sha256.Sum256(payload)
			if !bytes.Equal(h.Sum(nil), want[:]) {
				return fmt.Errorf("logger atualizado inválido")
			}
		} else {
			if previous==nil{return fmt.Errorf("arquivo adicional inesperado: %s",entry.Name)}
			r, err = previous.Open()
			if err != nil {
				return err
			}
			old := sha256.New()
			_, err = io.Copy(old, r)
			r.Close()
			if err != nil || !bytes.Equal(h.Sum(nil), old.Sum(nil)) {
				return fmt.Errorf("um arquivo alheio à comunicação foi alterado: %s", entry.Name)
			}
		}
	}
	return nil
}
