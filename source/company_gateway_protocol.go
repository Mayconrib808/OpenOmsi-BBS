package main

// The official simulator keeps ownership of the world. This adapter changes
// only the vehicle filename announced in HELLO/INFO, never physics, people,
// clock, weather or binary state. Peers still receive the original filename.
import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const companyGatewayPacketLimit = 65507

func companyGatewayBusKey(path string) string {
	if len(path) > 1024 || !utf8.ValidString(path) || strings.ContainsAny(path, "|;\r\n\x00") {
		return ""
	}
	s := strings.ReplaceAll(strings.TrimSpace(path), `\`, "/")
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." {
			return ""
		}
	}
	lower := strings.ToLower(s)
	if i := strings.LastIndex(lower, "/vehicles/"); i >= 0 {
		s, lower = s[i+1:], lower[i+1:]
	}
	if !strings.HasPrefix(lower, "vehicles/") || !strings.HasSuffix(lower, ".bus") {
		return ""
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, ":") {
			return ""
		}
	}
	return lower
}

// Installed buses are simulator models, not permissions for players. Unknown
// filenames go through a missing sentinel, activating upstream's stand-in path.
// The first installed bus is that stand-in. It is never offered as a player list.
func companyGatewayFleet(root string, preferred []string, token string) ([]string, map[string]string, string, error) {
	catalog := map[string]string{}
	vehicles := filepath.Join(root, "Vehicles")
	err := filepath.WalkDir(vehicles, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".bus") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if key := companyGatewayBusKey(rel); key != "" {
			catalog[key] = rel
			if len(catalog) > 10000 {
				return fmt.Errorf("mais de 10000 modelos locais de ônibus; revise a pasta Vehicles")
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, "", fmt.Errorf("não foi possível ler os modelos locais de ônibus: %w", err)
	}
	if len(catalog) == 0 {
		return nil, nil, "", fmt.Errorf("instale pelo menos um ônibus comum no OMSI 2 para servir de modelo substituto; não é preciso instalar os ônibus privados dos jogadores")
	}
	paths := make([]string, 0, len(catalog))
	for _, path := range catalog {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return companyGatewayBusKey(paths[i]) < companyGatewayBusKey(paths[j]) })
	first := paths[0]
	choices := append(append([]string(nil), preferred...), "Vehicles/MAN_SD202/MAN_D92.bus", "Vehicles/MAN_SD200/MAN_SD77.bus")
	for _, choice := range choices {
		if installed, ok := catalog[companyGatewayBusKey(choice)]; ok {
			first = installed
			break
		}
	}
	for i, path := range paths {
		if path == first {
			paths[0], paths[i] = paths[i], paths[0]
			break
		}
	}
	mask := "Vehicles/__OpenOmsi_BBS_" + token + "/missing.bus"
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(mask))); !os.IsNotExist(err) {
		return nil, nil, "", fmt.Errorf("o caminho do modelo substituto reservado já existe")
	}
	paths = append(paths, mask)
	if len(strings.Join(paths, ";")) > 900<<10 {
		return nil, nil, "", fmt.Errorf("a lista de modelos locais excede o tamanho da configuração do servidor")
	}
	return paths, catalog, mask, nil
}

func companyGatewayConfig(text []byte, udpPort, webPort int, fleet []string) []byte {
	values := map[string]string{
		"port": fmt.Sprint(udpPort), "web_port": fmt.Sprint(webPort), "tunnel": "0",
		"vehicles": strings.Join(fleet, ";"), "free_player_vehicles": "0",
	}
	lines := strings.Split(strings.ReplaceAll(string(bytes.TrimPrefix(text, []byte{0xef, 0xbb, 0xbf})), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if value, replace := values[key]; ok && replace {
			lines[i] = key + " = " + value
			seen[key] = true
		}
	}
	for _, key := range []string{"port", "web_port", "tunnel", "vehicles", "free_player_vehicles"} {
		if !seen[key] {
			lines = append(lines, key+" = "+values[key])
		}
	}
	return []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n")
}

func companyGatewayID(text string) uint32 {
	var id uint64
	if text == "" || len(text) > 10 {
		return 0
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + uint64(c-'0')
		if id > 0xffffffff {
			return 0
		}
	}
	return uint32(id)
}

// Called with gateway.mu held. A transport may update its own assigned ID only.
func (g *companyGateway) incoming(p *companyGatewayPeer, packet []byte) ([]byte, error) {
	if len(packet) > companyGatewayPacketLimit {
		return nil, fmt.Errorf("datagrama muito grande")
	}
	index, isHello := -1, bytes.HasPrefix(packet, []byte("HELLO|"))
	if isHello {
		index = 4
	} else if bytes.HasPrefix(packet, []byte("INFO|")) {
		index = 3
	}
	if index < 0 {
		return packet, nil
	}
	parts := strings.Split(string(packet), "|")
	if len(parts) <= index {
		return nil, fmt.Errorf("anúncio de ônibus inválido")
	}
	if isHello {
		if parts[1] != fmt.Sprint(multiplayerProtocol) {
			return packet, nil
		}
	} else if id := companyGatewayID(parts[1]); id == 0 || id != p.id {
		return nil, fmt.Errorf("anúncio com ID de outro jogador")
	}
	original := parts[index]
	if original != "" {
		key := companyGatewayBusKey(original)
		if key == "" {
			return nil, fmt.Errorf("caminho de ônibus inválido")
		}
		if installed, ok := g.catalog[key]; ok {
			parts[index] = installed
		} else {
			parts[index] = g.mask
		}
	}
	p.bus = original
	if !isHello && p.id != 0 {
		if record := g.buses[p.id]; record != nil && record.owner == p {
			record.bus = original
		}
	}
	result := []byte(strings.Join(parts, "|"))
	if len(result) > companyGatewayPacketLimit {
		return nil, fmt.Errorf("anúncio de ônibus muito grande")
	}
	return result, nil
}

// Called with gateway.mu held, before forwarding even the first WELCOME.
func (g *companyGateway) outgoing(p *companyGatewayPeer, packet []byte) []byte {
	if bytes.HasPrefix(packet, []byte("WELCOME|")) {
		parts := strings.Split(string(packet), "|")
		if len(parts) > 3 && parts[1] == fmt.Sprint(multiplayerProtocol) {
			if id := companyGatewayID(parts[2]); id != 0 {
				if old := g.buses[p.id]; old != nil && old.owner == p && p.id != id {
					delete(g.buses, p.id)
				}
				p.id = id
				g.buses[id] = &companyGatewayBus{owner: p, bus: p.bus}
			}
		}
	} else if bytes.HasPrefix(packet, []byte("INFO|")) {
		parts := strings.Split(string(packet), "|")
		if len(parts) > 3 {
			if record := g.buses[companyGatewayID(parts[1])]; record != nil {
				parts[3] = record.bus
				return []byte(strings.Join(parts, "|"))
			}
		}
	} else if bytes.HasPrefix(packet, []byte("BYE|")) {
		delete(g.buses, companyGatewayID(strings.TrimPrefix(string(packet), "BYE|")))
	}
	return packet
}
