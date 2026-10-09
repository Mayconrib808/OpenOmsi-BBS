package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const setupPage = `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>OpenOmsi + BBS 3.0</title><style>body{font:17px system-ui;background:#111b27;color:#eef4fa;max-width:680px;margin:40px auto;padding:20px}h1{font-size:30px}label{display:block;margin:18px 0 6px}input,button{font:inherit;padding:12px;box-sizing:border-box;width:100%;border-radius:8px;border:1px solid #64778b}button{background:#75d8b7;color:#102820;margin-top:24px;cursor:pointer}small{color:#afc1d4}details{margin-top:25px}code{overflow-wrap:anywhere}.error{color:#ffada5}.done{border:1px solid #75d8b7;padding:20px;border-radius:12px}</style><h1>OpenOmsi + BBS 3.0</h1><p>Uma configuração. Depois, comece a viagem normalmente no BCS.</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}{{if .Done}}<div class="done"><p>Configuração salva.</p><p>No BCS, escolha esta pasta em <b>OpenOMSI Path</b>:</p><code>{{.Dir}}</code><p>Convite fixo da empresa:</p><code>{{.Invite}}</code><p>Agora inicie uma viagem. O primeiro jogador hospedará o mapa; os próximos entrarão na mesma sessão.</p></div>{{else}}<form method="post"><input type="hidden" name="token" value="{{.Token}}"><label>openomsi.exe original instalado</label><input name="game" value="{{.Game}}" placeholder="C:\Jogos\openOMSI\openomsi.exe" required><label>Pasta do OMSI 2</label><input name="root" value="{{.Root}}" placeholder="C:\Steam\steamapps\common\OMSI 2" required><label>Seu nome no multiplayer</label><input name="player" value="{{.Player}}" maxlength="80" required><label>Convite da empresa</label><input name="invite" value="{{.Invite}}" placeholder="Cole o link recebido; ou crie a empresa abaixo"><details><summary>Criar uma empresa (uma vez)</summary><label>Nome da empresa</label><input name="name" maxlength="120"><label>Diferença do horário da empresa, em horas</label><input name="offset" type="number" min="-24" max="24" step="0.5" value="-3"><small>Exemplos: −3, −8. Aplicada ao relógio de referência abaixo, incluindo a mudança de dia.</small><label>Relógio de referência</label><input name="zone" value="Europe/Berlin"><small>Europe/Berlin é a referência padrão do BCS. Use UTC para um deslocamento fixo de UTC.</small></details><label><input type="checkbox" name="edit" style="width:auto"> Atualizar nome e horário da minha empresa (criador, com mapas vazios)</label><button>Salvar e usar no BCS</button></form><p><small>O PC do primeiro jogador precisa continuar ligado enquanto hospeda. Prévia: o serviço central precisa receber a atualização 3.0.</small></p>{{end}}</html>`

type setupView struct {
	Token, Dir, Game, Root, Player, Invite, Error string
	Done                                          bool
}

func setup(dir string) error {
	c, _ := readConfig(dir)
	token := randomID()
	page := template.Must(template.New("setup").Parse(setupPage))
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	address := "http://" + listener.Addr().String()
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	var mu sync.Mutex
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Host != listener.Addr().String() {
			http.Error(w, "invalid host", 403)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		v := setupView{Token: token, Dir: dir, Game: c.Game, Root: c.Root, Player: c.Player, Invite: c.Invitation}
		if r.Method == "GET" {
			if r.URL.Query().Get("token") != token {
				http.Error(w, "Abra Configurar.exe", 403)
				return
			}
			_ = page.Execute(w, v)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if r.ParseForm() != nil || r.Form.Get("token") != token || r.Header.Get("Origin") != address {
			http.Error(w, "invalid form", 403)
			return
		}
		candidate := localConfig{Version: 3, Game: strings.Trim(strings.TrimSpace(r.Form.Get("game")), "\""), Root: strings.Trim(strings.TrimSpace(r.Form.Get("root")), "\""), Player: strings.TrimSpace(r.Form.Get("player")), Invitation: strings.TrimSpace(r.Form.Get("invite"))}
		v.Game, v.Root, v.Player, v.Invite = candidate.Game, candidate.Root, candidate.Player, candidate.Invitation
		err := validateInstall(candidate, dir)
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		client := webClient()
		defer client.CloseIdleConnections()
		if err == nil && candidate.Invitation == "" {
			offset, parseErr := strconv.ParseFloat(r.Form.Get("offset"), 64)
			settings := companySettings{Name: strings.TrimSpace(r.Form.Get("name")), Clock: CompanyClock{TimeZone: r.Form.Get("zone"), ShiftMinutes: int(offset * 60)}}
			if parseErr != nil || offset < -24 || offset > 24 || offset*60 != float64(settings.Clock.ShiftMinutes) {
				err = fmt.Errorf("horário inválido")
			} else if err = validateCompanyClock(settings.Clock); err == nil {
				candidate.Invitation = defaultDirectory + "/rooms/" + randomID()
				var created companySettings
				err = requestJSON(ctx, client, "POST", candidate.Invitation+"/v3/settings", "", settings, &created)
				if err == nil {
					b, _ := json.MarshalIndent(created, "", "  ")
					err = os.WriteFile(filepath.Join(dir, "company-admin.json"), b, 0600)
					v.Invite = candidate.Invitation
				}
			}
		} else if err == nil {
			candidate.Invitation, err = invitation(candidate.Invitation)
			if err == nil {
				var s companySettings
				err = requestJSON(ctx, client, "GET", candidate.Invitation+"/v3/settings", "", nil, &s)
				if err == nil {
					err = validateCompanyClock(s.Clock)
				}
			}
		}
		if err == nil && r.Form.Get("edit") == "on" {
			var admin companySettings
			b, readErr := os.ReadFile(filepath.Join(dir, "company-admin.json"))
			if readErr != nil || json.Unmarshal(b, &admin) != nil || admin.AdminToken == "" || candidate.Invitation != c.Invitation {
				err = fmt.Errorf("a chave do criador desta empresa não foi encontrada neste PC")
			} else {
				offset, parseErr := strconv.ParseFloat(r.Form.Get("offset"), 64)
				update := companySettings{Name: strings.TrimSpace(r.Form.Get("name")), Clock: CompanyClock{TimeZone: r.Form.Get("zone"), ShiftMinutes: int(offset * 60)}}
				if parseErr != nil || offset < -24 || offset > 24 || offset*60 != float64(update.Clock.ShiftMinutes) {
					err = fmt.Errorf("horário inválido")
				} else if err = validateCompanyClock(update.Clock); err == nil {
					err = requestJSON(ctx, client, "POST", candidate.Invitation+"/v3/settings", admin.AdminToken, update, nil)
				}
			}
		}

		if err == nil {
			b, _ := json.MarshalIndent(candidate, "", "  ")
			tmp := configPath(dir) + ".tmp"
			err = os.WriteFile(tmp, b, 0600)
			if err == nil {
				err = os.Rename(tmp, configPath(dir))
			}
			if err == nil {
				c = candidate
				v.Done = true
				v.Invite = c.Invitation
				time.AfterFunc(2*time.Second, func() { _ = srv.Close() })
			}
		}
		if err != nil {
			v.Error = fmt.Sprintf("%v. Se o serviço retornar 404, falta atualizar o diretório para 3.0.", err)
		}
		_ = page.Execute(w, v)
	})
	if e = openSetup(address + "/?token=" + token); e != nil {
		return e
	}
	timer := time.AfterFunc(30*time.Minute, func() { _ = srv.Close() })
	defer timer.Stop()
	e = srv.Serve(listener)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
