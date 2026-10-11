package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// The SHA is the complete inspected vendor build, not a filename/version guess.
// Preserve all other entries byte for byte and keep a recoverable original.
func patchBCSJar(path, expectedSHA string, logger []byte) (bool, error) {
	if len(logger) == 0 {
		return false, fmt.Errorf("payload de comunicação ausente")
	}
	reader, e := zip.OpenReader(path)
	if e != nil {
		return false, fmt.Errorf("não foi possível ler o JAR: %w", e)
	}
	found := false
	for _, entry := range reader.File {
		if entry.Name == bcsLoggerEntry {
			found = true
			r, err := entry.Open()
			if err != nil {
				reader.Close()
				return false, err
			}
			data, err := io.ReadAll(io.LimitReader(r, 256<<10))
			r.Close()
			if err != nil {
				reader.Close()
				return false, err
			}
			if bytes.Equal(data, logger) {
				reader.Close()
				return false, nil
			}
		}
	}
	if !found {
		reader.Close()
		return false, fmt.Errorf("este arquivo não contém o logger do BCS verificado")
	}
	actual, e := bcsJarSHA(path)
	if e != nil || actual != expectedSHA {
		reader.Close()
		return false, fmt.Errorf("este JAR é de outra versão ou foi modificado; nenhum arquivo foi alterado")
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
		if entry.Name == bcsLoggerEntry {
			header.CRC32, header.CompressedSize, header.CompressedSize64 = 0, 0, 0
			header.UncompressedSize, header.UncompressedSize64 = 0, 0
			w, err := writer.CreateHeader(&header)
			if err == nil {
				_, err = w.Write(logger)
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
	if e = verifyPatchedBCSJar(tmpName, reader, logger); e != nil {
		reader.Close()
		return false, e
	}
	reader.Close()
	backup := path + ".antes-proxima-viagem.bak"
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
	after, e := zip.OpenReader(path)
	if e != nil {
		return e
	}
	defer after.Close()
	if len(after.File) != len(before.File) {
		return fmt.Errorf("a estrutura do JAR não foi preservada")
	}
	for i, entry := range after.File {
		previous := before.File[i]
		if entry.Name != previous.Name {
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
		if entry.Name == bcsLoggerEntry {
			want := sha256.Sum256(logger)
			if !bytes.Equal(h.Sum(nil), want[:]) {
				return fmt.Errorf("logger atualizado inválido")
			}
		} else {
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
