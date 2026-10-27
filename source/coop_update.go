package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type coopDistributionPart struct {
	SHA256 string
	Size   int64
}

type coopDistribution struct {
	FullPaths bool // Signed cumulative bundles may also carry the managed dynamic host.
	SHA256    string
	Size      int64
	Prefix    string
	Parts     []coopDistributionPart
	Files     map[string]string
}

func coopDigestValid(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size && value == strings.ToLower(value)
}

func coopPayloadFileAllowed(name string) bool {
	_, ok := bundledCoopDistribution.Files[name]
	return ok
}

func verifyCoopPayloadFile(name string, data []byte) bool {
	expected, ok := bundledCoopDistribution.Files[name]
	return ok && fmt.Sprintf("%x", sha256.Sum256(data)) == expected
}

func existingCoopBundle(dir string, spec coopDistribution) bool {
	for name, digest := range spec.Files {
		if (!spec.FullPaths && !strings.HasPrefix(name, bundledCoopFolder+"/")) || name == ".." || name == "." || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") || strings.Contains(name, ":") || filepath.ToSlash(filepath.Clean(filepath.FromSlash(name))) != name {
			return false
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		for parent := filepath.Dir(path); parent != filepath.Clean(dir); parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return false
			}
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		file, err := os.Open(path)
		if err != nil {
			return false
		}
		h := sha256.New()
		_, err = io.Copy(h, file)
		closeErr := file.Close()
		if err != nil || closeErr != nil || fmt.Sprintf("%x", h.Sum(nil)) != digest {
			return false
		}
	}
	return len(spec.Files) > 0
}

func readCoopDistribution(data []byte, spec coopDistribution) (map[string][]byte, error) {
	if int64(len(data)) != spec.Size || fmt.Sprintf("%x", sha256.Sum256(data)) != spec.SHA256 {
		return nil, fmt.Errorf("a transferência do Cooperativo não passou na verificação SHA-256")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(archive.File) != len(spec.Files) {
		return nil, fmt.Errorf("pacote do Cooperativo incompleto")
	}
	files := make(map[string][]byte, len(spec.Files))
	var total uint64
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, spec.Prefix) || entry.FileInfo().IsDir() || entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("caminho inválido no pacote do Cooperativo")
		}
		rel := strings.TrimPrefix(entry.Name, spec.Prefix)
		if rel == "" || rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "\\") || strings.Contains(rel, ":") || strings.HasPrefix(rel, "/") || filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel))) != rel {
			return nil, fmt.Errorf("caminho inválido no pacote do Cooperativo")
		}
		name := bundledCoopFolder + "/" + rel
		if spec.FullPaths {
			name = rel
		}
		expected, ok := spec.Files[name]
		total += entry.UncompressedSize64
		if !ok || files[name] != nil || entry.UncompressedSize64 > 128<<20 || total > 512<<20 {
			return nil, fmt.Errorf("arquivo inesperado no pacote do Cooperativo")
		}
		r, err := entry.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(r, int64(entry.UncompressedSize64)+1))
		closeErr := r.Close()
		if err != nil || closeErr != nil || uint64(len(content)) != entry.UncompressedSize64 || fmt.Sprintf("%x", sha256.Sum256(content)) != expected {
			return nil, fmt.Errorf("arquivo do Cooperativo corrompido: %s", name)
		}
		files[name] = content
	}
	return files, nil
}

// Parts stay under Workers' per-asset limit and the 2.1 Setup still receives a
// small signed EXE. Every part and the complete ZIP are pinned in that EXE.
func downloadCoopDistribution(ctx context.Context, client *http.Client, origin string, spec coopDistribution, cache string, progress func(int64, int64)) ([]byte, error) {
	base, err := updateOrigin(origin)
	if err != nil {
		return nil, err
	}
	if !coopDigestValid(spec.SHA256) || spec.Size < 1 || spec.Size > 256<<20 || len(spec.Parts) == 0 || len(spec.Parts) > 16 {
		return nil, fmt.Errorf("distribuição do Cooperativo inválida")
	}
	var expectedSize int64
	for _, part := range spec.Parts {
		if !coopDigestValid(part.SHA256) || part.Size < 1 || part.Size > 20<<20 {
			return nil, fmt.Errorf("parte do Cooperativo inválida")
		}
		expectedSize += part.Size
	}
	if expectedSize != spec.Size {
		return nil, fmt.Errorf("tamanho do Cooperativo inválido")
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return nil, err
	}
	var archive bytes.Buffer
	archive.Grow(int(spec.Size))
	for _, part := range spec.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(cache, part.SHA256+".bin")
		if old, err := os.ReadFile(path); err == nil && int64(len(old)) == part.Size && fmt.Sprintf("%x", sha256.Sum256(old)) == part.SHA256 {
			archive.Write(old)
			if progress != nil {
				progress(int64(archive.Len()), spec.Size)
			}
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String()+"/packages/"+part.SHA256+".bin", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "OpenOmsi-BBS-Updater/"+bridgeVersion)
		response, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("não foi possível baixar o Cooperativo: %w", err)
		}
		if response.StatusCode != http.StatusOK || response.ContentLength >= 0 && response.ContentLength != part.Size {
			response.Body.Close()
			return nil, fmt.Errorf("download do Cooperativo retornou tamanho ou status inesperado")
		}
		var content bytes.Buffer
		buffer := make([]byte, 64<<10)
		for {
			n, readErr := response.Body.Read(buffer)
			if n > 0 {
				if int64(content.Len()+n) > part.Size {
					response.Body.Close()
					return nil, fmt.Errorf("download do Cooperativo excedeu o tamanho esperado")
				}
				content.Write(buffer[:n])
				if progress != nil {
					progress(int64(archive.Len()+content.Len()), spec.Size)
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				response.Body.Close()
				return nil, readErr
			}
		}
		response.Body.Close()
		if int64(content.Len()) != part.Size || fmt.Sprintf("%x", sha256.Sum256(content.Bytes())) != part.SHA256 {
			return nil, fmt.Errorf("parte do Cooperativo não passou na verificação SHA-256")
		}
		tmp, err := os.CreateTemp(cache, ".coop-*.tmp")
		if err != nil {
			return nil, err
		}
		_, writeErr := tmp.Write(content.Bytes())
		if writeErr == nil {
			writeErr = tmp.Sync()
		}
		closeErr := tmp.Close()
		if writeErr != nil || closeErr != nil {
			os.Remove(tmp.Name())
			return nil, fmt.Errorf("não foi possível salvar o Cooperativo")
		}
		// A stale cache item is replaced only after the new bytes were verified.
		os.Remove(path)
		if err := os.Rename(tmp.Name(), path); err != nil {
			os.Remove(tmp.Name())
			return nil, err
		}
		archive.Write(content.Bytes())
	}
	if fmt.Sprintf("%x", sha256.Sum256(archive.Bytes())) != spec.SHA256 {
		return nil, fmt.Errorf("ZIP do Cooperativo corrompido")
	}
	return archive.Bytes(), nil
}

func loadBundledCoopUpdate(ctx context.Context, dir string, progress func(int64, int64)) (map[string][]byte, error) {
	if existingCoopBundle(dir, bundledCoopDistribution) {
		return nil, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	client := bridgeUpdateClient()
	client.Timeout = 0 // The cancellable operation sets the overall deadline.
	data, err := downloadCoopDistribution(ctx, client, bundledCoopOrigin, bundledCoopDistribution, filepath.Join(cache, "OpenOmsi-BBS", "Coop-0.3.41"), progress)
	if err != nil {
		return nil, err
	}
	return readCoopDistribution(data, bundledCoopDistribution)
}

// Backups contain only the compiled allowlist of managed Coop files. A stock
// updater must never replace the last verified modified build in this snapshot.
func coopBackupSpec(spec coopDistribution) coopDistribution {
	out := coopDistribution{FullPaths: true, Files: map[string]string{}}
	for name, digest := range spec.Files {
		if strings.HasPrefix(name, bundledCoopFolder+"/") {
			out.Files[name] = digest
		}
	}
	return out
}

func createCoopBackup(dir, target string, spec coopDistribution) (err error) {
	spec = coopBackupSpec(spec)
	if !existingCoopBundle(dir, spec) {
		return fmt.Errorf("a versão modificada foi alterada; restaure o Cooperativo incluído antes de criar outro backup")
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".coop-backup-*.zip")
	if err != nil {
		return err
	}
	temporary := f.Name()
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(temporary)
		}
	}()
	z := zip.NewWriter(f)
	for name, digest := range spec.Files {
		var w io.Writer
		w, err = z.Create(name)
		if err != nil {
			return err
		}
		var src *os.File
		src, err = os.Open(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(io.MultiWriter(w, h), src)
		src.Close()
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", h.Sum(nil)) != digest {
			return fmt.Errorf("o arquivo mudou durante o backup: %s", name)
		}
	}
	if err = z.Close(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// A version's verified backup is immutable; no window without a good snapshot.
	if _, e := os.Stat(target); e == nil {
		os.Remove(temporary)
		return nil
	}
	return os.Rename(temporary, target)
}

func restoreCoopBackup(dir, backup string, spec coopDistribution) (err error) {
	spec = coopBackupSpec(spec)
	z, err := zip.OpenReader(backup)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(spec.Files) == 0 || len(z.File) != len(spec.Files) {
		return fmt.Errorf("backup incompleto")
	}
	payload := map[string][]byte{}
	for _, entry := range z.File {
		digest, ok := spec.Files[entry.Name]
		if !ok || payload[entry.Name] != nil || entry.UncompressedSize64 > 512*1024*1024 {
			return fmt.Errorf("arquivo inesperado no backup: %s", entry.Name)
		}
		src, e := entry.Open()
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(src, 512*1024*1024+1))
		src.Close()
		if e != nil {
			return e
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != digest {
			return fmt.Errorf("backup alterado: %s", entry.Name)
		}
		payload[entry.Name] = b
	}
	// Check all parents and save every previous file before changing anything.
	previous := map[string][]byte{}
	for name := range payload {
		p := filepath.Join(dir, filepath.FromSlash(name))
		for parent := filepath.Dir(p); parent != filepath.Clean(dir); parent = filepath.Dir(parent) {
			st, e := os.Lstat(parent)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("pasta de restauração inválida: %s", parent)
			}
		}
		if st, e := os.Lstat(p); e == nil && !st.Mode().IsRegular() {
			return fmt.Errorf("arquivo de restauração inválido: %s", name)
		}
		b, e := os.ReadFile(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		previous[name] = b
	}
	written := []string{}
	defer func() {
		if err != nil {
			for _, name := range written {
				p := filepath.Join(dir, filepath.FromSlash(name))
				var e error
				if previous[name] == nil {
					e = os.Remove(p)
				} else {
					e = writeSetupAtomic(p, previous[name])
				}
				if e != nil {
					err = fmt.Errorf("%w; não foi possível recuperar %s: %v", err, name, e)
				}
			}
		}
	}()
	for name, b := range payload {
		if err = writeSetupAtomic(filepath.Join(dir, filepath.FromSlash(name)), b); err != nil {
			return err
		}
		written = append(written, name)
	}
	if !existingCoopBundle(dir, spec) {
		return fmt.Errorf("a restauração não passou na verificação final")
	}
	return nil
}
