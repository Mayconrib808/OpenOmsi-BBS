package main
import (
 "context"
 "fmt"
 "log"
 "path/filepath"
)
func normalizeClientMode(c localConfig, dir string) (string,error) {
 switch c.ClientMode {
 case "latest","manual":return c.ClientMode,nil
 case "":
  if c.Game!="" && !insideDirectory(filepath.Join(dir,"client"),c.Game) { return "manual",nil }
  return "latest",nil
 default:return "",fmt.Errorf("modo do cliente inválido")
 }
}
func prepareConfiguredClient(ctx context.Context,c localConfig,dir string,resolve func(context.Context,string)(string,error))(localConfig,error) {
 mode,err:=normalizeClientMode(c,dir)
 if err!=nil{return c,err}
 c.ClientMode=mode
 if mode=="manual" {
  if c.Game=="" {return c,fmt.Errorf("informe o caminho do openomsi.exe para o cliente manual")}
  return c,nil
 }
 log.Print(tr("clientdownload"))
 next,err:=resolve(ctx,dir)
 if err!=nil {
  // Network failure may reuse only an unchanged, previously verified managed client.
  if insideDirectory(filepath.Join(dir,"client"),c.Game) {
   version:=filepath.Base(filepath.Dir(c.Game))
   if previous,verifyErr:=cachedClient(dir,version);verifyErr==nil && samePath(previous,c.Game) {
    log.Printf("Client automatic update unavailable; keeping verified %s: %v",version,err)
    return c,nil
   }
  }
  return c,err
 }
 c.Game=next
 return c,nil
}

func validateConfiguredInstall(c localConfig,dir string) error {
 mode,err:=normalizeClientMode(c,dir)
 if err!=nil{return err}
 if mode=="latest" {c.Game=""}
 return validateInstall(c,dir)
}
