package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const defaultDirectory = "https://openomsi-bbs-directory.maycongamesmaximo.workers.dev"

var roomPath = regexp.MustCompile(`^/rooms/[a-f0-9]{32}$`)

func invitation(raw string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !roomPath.MatchString(strings.TrimRight(u.Path, "/")) {
		return "", fmt.Errorf("cole o convite HTTPS completo da empresa")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func randomID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func mapID(m string) string { h := sha256.Sum256([]byte(m)); return hex.EncodeToString(h[:16]) }

type lease struct {
	Role       string       `json:"role"`
	State      string       `json:"state"`
	Address    string       `json:"address"`
	Token      string       `json:"lease_token"`
	Generation string       `json:"generation"`
	Expires    int64        `json:"expires_at"`
	Map        string       `json:"map"`
	Clock      CompanyClock `json:"clock"`
}
type companySettings struct {
	Name       string       `json:"name"`
	Clock      CompanyClock `json:"clock"`
	Version    int          `json:"version"`
	AdminToken string       `json:"admin_token,omitempty"`
}
type httpError struct{ Code int }

func (e httpError) Error() string { return fmt.Sprintf("diretório retornou HTTP %d", e.Code) }
func webClient() *http.Client {
	return &http.Client{Timeout: 8e9, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirecionamento recusado") }}
}
func requestJSON(ctx context.Context, c *http.Client, method, address, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	r, e := http.NewRequestWithContext(ctx, method, address, body)
	if e != nil {
		return e
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := c.Do(r)
	if e != nil {
		return fmt.Errorf("não foi possível acessar o serviço de sessões: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpError{resp.StatusCode}
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(out)
	}
	return nil
}
func claim(ctx context.Context, c *http.Client, room, mapFile, owner string) (lease, error) {
	var l lease
	e := requestJSON(ctx, c, "POST", room+"/v3/maps/"+mapID(mapFile)+"/claim", "", map[string]string{"map": mapFile, "owner": owner}, &l)
	return l, e
}
func renew(ctx context.Context, c *http.Client, room, mapFile, token, state, address string) (lease, error) {
	var l lease
	e := requestJSON(ctx, c, "POST", room+"/v3/maps/"+mapID(mapFile)+"/renew", token, map[string]string{"state": state, "address": address}, &l)
	return l, e
}
func release(c *http.Client, room, mapFile, token string) {
	_ = requestJSON(context.Background(), c, "POST", room+"/v3/maps/"+mapID(mapFile)+"/release", token, map[string]string{}, nil)
}
