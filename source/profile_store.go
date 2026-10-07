package main

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "os"
 "path/filepath"
 "strings"
)

func managedDataDir(packageDir string) string {
 if root := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(root) { return filepath.Join(root, "OpenOmsi-BBS") }
 if root, err := os.UserConfigDir(); err == nil && filepath.IsAbs(root) { return filepath.Join(root, "OpenOmsi-BBS") }
 return filepath.Join(packageDir, "user-settings")
}

func managedCompanyCacheDir(packageDir string) string {
 return filepath.Join(managedDataDir(packageDir), "Companies")
}

func readInstalledConfig(packageDir string) Config {
 local := readConfig(configPath(packageDir))
 if local.Root != "" && local.OpenOMSI != "" { return local }
 saved := readConfig(filepath.Join(managedDataDir(packageDir), "bridge.ini"))
 if saved.Root != "" && saved.OpenOMSI != "" { return saved }
 return local
}

func writeManagedFileAtomic(path string, data []byte) error {
 if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return err }
 f, err := os.CreateTemp(filepath.Dir(path), ".openomsi-bbs-*.tmp")
 if err != nil { return err }
 name := f.Name()
 defer os.Remove(name)
 if _, err = f.Write(data); err != nil { f.Close(); return err }
 if err = f.Sync(); err != nil { f.Close(); return err }
 if err = f.Close(); err != nil { return err }
 return os.Rename(name, path)
}

func saveInstalledConfig(packageDir string, c Config) error {
 path := configPath(packageDir)
 if old, err := os.ReadFile(path); err == nil {
  if err = writeManagedFileAtomic(path+".backup", old); err != nil { return err }
 }
 if err := writeManagedFileAtomic(path, encodeConfig(c)); err != nil { return err }
 return writeManagedFileAtomic(filepath.Join(managedDataDir(packageDir), "bridge.ini"), encodeConfig(c))
}

func saveManagedProfile(packageDir string, p CompanyProfile) (string, error) {
 if err := validateCompanyProfile(p); err != nil { return "", err }
 data, err := json.MarshalIndent(p, "", "  ")
 if err != nil { return "", err }
 path := filepath.Join(managedCompanyCacheDir(packageDir), p.CompanyID+".json")
 if err = writeManagedFileAtomic(path, append(data, '\n')); err != nil { return "", err }
 return path, nil
}

// Import once into user settings. An administrator's local source remains a
// refresh source while available; the imported copy survives moving the ZIP.
func enrollCompany(ctx context.Context, packageDir, source, player string, c Config) (Config, *CompanyProfile, error) {
 source = strings.Trim(strings.TrimSpace(source), "\"")
 player = strings.TrimSpace(player)
 if !companyText(player, 32) || strings.Contains(player, "|") { return c, nil, fmt.Errorf("use um nome de 1 a 32 caracteres, sem | ou caracteres de controle") }
 if !strings.Contains(source, "://") && !filepath.IsAbs(source) { source = filepath.Join(packageDir, source) }
 p, err := loadCompanyProfile(ctx, source, packageDir, companyHTTPClient())
 if err != nil { return c, nil, err }
 internal, err := saveManagedProfile(packageDir, p)
 if err != nil { return c, nil, err }
 c.Multiplayer, c.CompanyID, c.PlayerName = true, p.CompanyID, player
 c.CompanyProfile, c.CompanyProfileSource = internal, source
 return c, &p, nil
}

// Keep company identity pinned. The cache never invents hashes or adopts the
// player's installed files; every loaded administrator profile is validated.
func loadInstalledCompanyProfile(ctx context.Context, c Config, packageDir string, client *http.Client) (CompanyProfile, error) {
 source := c.CompanyProfileSource
 if source == "" { source = c.CompanyProfile }
 p, err := loadCompanyProfile(ctx, source, packageDir, client)
 if err != nil && !strings.Contains(source, "://") && os.IsNotExist(err) && companyIDPattern.MatchString(c.CompanyID) {
  p, err = loadCompanyProfile(ctx, filepath.Join(managedCompanyCacheDir(packageDir), c.CompanyID+".json"), packageDir, client)
 }
 if err != nil { return p, err }
 if p.CompanyID != c.CompanyID { return p, fmt.Errorf("o perfil mudou de empresa; selecione o novo perfil no menu") }
 _, err = saveManagedProfile(packageDir, p)
 return p, err
}
