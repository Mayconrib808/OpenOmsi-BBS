package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func cooperativePathFixture(t *testing.T) (string, string, string) {
	t.Helper()
	folder := filepath.Join(t.TempDir(), "OpenOMSI Cooperativo - Cópia")
	game := filepath.Join(folder, "programa", "openomsi.exe")
	if err := os.MkdirAll(filepath.Dir(game), 0755); err != nil {
		t.Fatal(err)
	}
	fakePE(t, game, 0x8664)
	opener := filepath.Join(folder, "OpenOmsi Coop.exe")
	fakePE(t, opener, 0x8664)
	return folder, game, opener
}

func TestCooperativeSetupResolvesFolderRenamedOpenerAndGame(t *testing.T) {
	folder, game, opener := cooperativePathFixture(t)
	root := t.TempDir()
	previous := defaultConfig()
	previous.AutoStart, previous.Traffic, previous.CompanyID = false, 43, "saved-company"
	previous.CompanyProfileSource = "https://company.invalid/profile.json"
	for _, selected := range []string{folder, filepath.Dir(game), opener, game, "\"" + opener + "\""} {
		t.Run(filepath.Base(selected), func(t *testing.T) {
			got, err := setupFormConfig(previous, root, selected, "Maycon", true)
			if err != nil {
				t.Fatal(err)
			}
			if got.OpenOMSI != game || got.Root != root || got.AutoStart || got.Traffic != 43 || got.CompanyID != previous.CompanyID || got.CompanyProfileSource != previous.CompanyProfileSource {
				t.Fatalf("incorrect target or changed BCS settings: %+v", got)
			}
		})
	}
	// The detected target survives a config save and existing configs that
	// selected the renamed opener migrate without writing any user file.
	for _, selected := range []string{folder, opener, game} {
		path := filepath.Join(t.TempDir(), "bridge.ini")
		c := previous
		c.OpenOMSI, c.Root = selected, root
		before := encodeConfig(c)
		if err := os.WriteFile(path, before, 0600); err != nil {
			t.Fatal(err)
		}
		got := readConfig(path)
		after, _ := os.ReadFile(path)
		if got.OpenOMSI != game || got.Root != root || got.AutoStart || !bytes.Equal(before, after) {
			t.Fatalf("runtime did not resolve saved target without modifying settings: %+v", got)
		}
	}
}

func TestCooperativeSelectionStillValidatesTheActualX64Game(t *testing.T) {
	t.Setenv("OMSI_PLUGIN_HOST32", "")
	packageDir := t.TempDir()
	c := defaultConfig()
	c.Root = filepath.Join(t.TempDir(), "OMSI 2 original")
	for _, dir := range []string{"maps", "vehicles", "Drivers"} {
		if err := os.MkdirAll(filepath.Join(c.Root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	fakePE(t, filepath.Join(c.Root, "Omsi.exe"), 0x14c)
	host := filepath.Join(packageDir, "app", "compat", "omsi-plugin-host32.exe")
	os.MkdirAll(filepath.Dir(host), 0755)
	fakePE(t, host, 0x14c)
	_, game, opener := cooperativePathFixture(t)
	c.OpenOMSI = resolveOpenOMSIExecutable(opener)
	if err := validateConfig(c, packageDir); err != nil {
		t.Fatal("valid portable game rejected", err)
	}
	fakePE(t, game, 0x14c)
	if err := validateConfig(c, packageDir); err == nil {
		t.Fatal("x86 portable game bypassed the real x64 check")
	}
	if err := os.Remove(game); err != nil {
		t.Fatal(err)
	}
	if got := resolveOpenOMSIExecutable(opener); got != opener {
		t.Fatal("missing internal game was silently accepted", got)
	}
	if err := validateConfig(c, packageDir); err == nil {
		t.Fatal("incomplete portable extraction was accepted")
	}
}

func TestCooperativeResolverLeavesOfficialAndInvalidSelectionsVisible(t *testing.T) {
	folder, game, _ := cooperativePathFixture(t)
	direct := filepath.Join(folder, "openomsi.exe")
	fakePE(t, direct, 0x8664)
	if got := resolveOpenOMSIExecutable(folder); got != direct {
		t.Fatal("official direct executable lost priority", got)
	}
	for _, selected := range []string{game, direct, filepath.Join(folder, "missing.exe"), filepath.Join(folder, "notes.txt"), "relative.exe", ""} {
		if got := resolveOpenOMSIExecutable(selected); got != selected {
			t.Fatalf("unexpected redirect of %q: %q", selected, got)
		}
	}
	text := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(text, []byte("not a launcher"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := resolveOpenOMSIExecutable(text); got != text {
		t.Fatal("non-executable selection was hidden", got)
	}
}

func TestCooperativeChildKeepsBCSRootArgumentsAndScopedEnvironment(t *testing.T) {
	folder, game, _ := cooperativePathFixture(t)
	f, err := os.OpenFile(game, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("0.3.38-coop.5")
	f.Close()
	root := filepath.Join(t.TempDir(), "OMSI 2 original")
	args := []string{"--root", root, "--bus", "Vehicles/Bus with spaces.bus", "--driver", "Drivers/BCS Driver.odr", "--lan-join", "session-code", "--setvar", "fuel=123.5"}
	env := []string{"KEEP=yes", "OMSI_PLUGIN_HOST32=original-root/helper.exe", "OMSI_CONTENT=bcs-telemetry", "omsi_no_update=0", "OMSI_NO_UPDATE=0", "omsi_no_guard=0", "OPENOMSI_BIN=other.exe", "omsi_launcher=unrelated.exe"}
	before := append([]string(nil), env...)
	cmd := exec.Command(game, args...)
	cmd.Env, cmd.Dir = env, root
	t.Setenv("OMSI_NO_UPDATE", "parent-must-stay")
	prepareOpenOMSICommand(cmd, game)
	want := []string{"KEEP=yes", "OMSI_PLUGIN_HOST32=original-root/helper.exe", "OMSI_CONTENT=bcs-telemetry", "OMSI_NO_UPDATE=1", "OMSI_NO_GUARD=1", "OPENOMSI_BIN=" + game}
	if cmd.Path != game || !reflect.DeepEqual(cmd.Args[1:], args) || cmd.Dir != filepath.Join(folder, "programa") || !reflect.DeepEqual(cmd.Env, want) {
		t.Fatalf("child launch changed BCS data: path=%q args=%q cwd=%q env=%q", cmd.Path, cmd.Args, cmd.Dir, cmd.Env)
	}
	if !reflect.DeepEqual(env, before) || os.Getenv("OMSI_NO_UPDATE") != "parent-must-stay" {
		t.Fatal("cooperative flags leaked into the parent")
	}
	// Existing official launches, including their updater choices, are kept.
	official := filepath.Join(t.TempDir(), "programa", "openomsi.exe")
	os.MkdirAll(filepath.Dir(official), 0755)
	fakePE(t, official, 0x8664)
	cmd = exec.Command(official, args...)
	cmd.Env, cmd.Dir = before, root
	prepareOpenOMSICommand(cmd, official)
	if cmd.Dir != root || !reflect.DeepEqual(cmd.Env, before) {
		t.Fatal("a folder named programa changed official game behavior")
	}
}

func TestCooperativeRealCLIProbeUsesInternalGameAndItsEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process fixture; Windows PE build is validated separately")
	}
	_, game, opener := cooperativePathFixture(t)
	// A real child process asserts the launch environment and working folder.
	// Its opener would fail, so a successful probe proves no opener was run.
	if err := os.WriteFile(opener, []byte("#!/bin/sh\nexit 97\n"), 0755); err != nil {
		t.Fatal(err)
	}
	help := compatibleOpenOMSIHelp(false, true)
	script := "#!/bin/sh\n# 0.3.38-coop.5 OMSI_PLUGIN_HOST32\n" +
		"[ \"$OMSI_NO_UPDATE\" = 1 ] && [ \"$OMSI_NO_GUARD\" = 1 ] && [ \"$OPENOMSI_BIN\" = \"$0\" ] && [ -z \"$OMSI_LAUNCHER\" ] || exit 91\n" +
		"[ \"$PWD\" = \"$(dirname \"$0\")\" ] || exit 92\n" +
		"case \"$1\" in\n--help) cat <<'HELP'\n" + help + "HELP\n;;\n--version) echo 'openomsi 0.3.38-coop.5';;\n*) exit 93;;\nesac\n"
	if err := os.WriteFile(game, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	// fakePE created the path as 0644; explicitly enable the process fixture.
	if err := os.Chmod(game, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMSI_LAUNCHER", "unrelated-launcher")
	c, err := setupFormConfig(defaultConfig(), t.TempDir(), opener, "Maycon", true)
	if err != nil || c.OpenOMSI != game {
		t.Fatal(c, err)
	}
	got, err := checkOpenOMSICompatibility(c.OpenOMSI, false, true)
	if err != nil || got.Version != "0.3.38-coop.5" {
		t.Fatal("real child probe failed", got, err)
	}
	if os.Getenv("OMSI_LAUNCHER") != "unrelated-launcher" {
		t.Fatal("probe changed the parent environment")
	}
	// Real feature checking still rejects an incompatible internal executable.
	script = strings.Replace(script, "      --lan-join <VALUE>", "      --lan-join-old <VALUE>", 1)
	os.WriteFile(game, []byte(script), 0755)
	if _, err := checkOpenOMSICompatibility(game, false, true); err == nil || !strings.Contains(err.Error(), "--lan-join") {
		t.Fatal("portable layout bypassed capability validation", err)
	}
}

func TestCooperativeMarkerAcrossReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openomsi.exe")
	data := append(bytes.Repeat([]byte{'x'}, (64<<10)-5), []byte("0.3.38-coop.5")...)
	os.WriteFile(path, data, 0600)
	if !cooperativeOpenOMSIBuild(path) {
		t.Fatal("cooperative identity missed across the streamed boundary")
	}
}

func TestCooperativeCommandsStayExactThroughCompanyGateway(t *testing.T) {
	for _, transport := range []string{"udp", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			f := newGatewayProtocolFixture(t, 14)
			var send func([]byte)
			var read func() []byte
			if transport == "udp" {
				c := gatewayUDPClient(t, f.g)
				send = func(b []byte) {
					if _, err := c.Write(b); err != nil {
						t.Fatal(err)
					}
				}
				read = func() []byte { return gatewayReadUDP(t, c) }
			} else {
				c, r := gatewayWSClient(t, f.g)
				send = func(b []byte) {
					if _, err := c.Write(gatewayMaskedFrame(2, true, b)); err != nil {
						t.Fatal(err)
					}
				}
				read = func() []byte {
					op, b := gatewayReadWS(t, r)
					if op != 2 {
						t.Fatal(op)
					}
					return b
				}
			}
			send([]byte(strings.Replace(string(gatewayHello(gatewayTestPrivate)), "HELLO|6|", "HELLO|14|", 1)))
			if got := read(); !bytes.HasPrefix(got, []byte("WELCOME|14|1|")) {
				t.Fatal("cooperative client did not join", string(got))
			}
			<-f.packets
			// The server owns authentication/routing. The bridge transports the
			// native CMD envelope without interpreting COOP1 actions or grants.
			for _, packet := range []string{"CMD|1|2|COOP1 q", "CMD|2|1|COOP1 g 9", "CMD|1|2|COOP1 k 9", "CMD|2|1|COOP1 a 9 1 1", "CMD|2|1|COOP1 r 9"} {
				send([]byte(packet))
				if got := read(); string(got) != packet {
					t.Fatal("COOP1 return packet changed", string(got))
				}
				if got := <-f.packets; string(got) != packet {
					t.Fatal("COOP1 outgoing packet changed", string(got))
				}
			}
		})
	}
}

func TestCooperativePatchRetainsProtocol14And15AndSixtyMinuteStartup(t *testing.T) {
	_, profile, _, _ := companyFixture(t)
	for _, item := range []struct {
		version  string
		protocol int
	}{{"0.3.38-coop.5", 14}, {"0.3.39", 14}, {"0.3.40", 15}} {
		profile.OpenOMSIVersion, profile.Protocol = item.version, item.protocol
		if err := validateCompanyProfile(profile); err != nil {
			t.Fatal(item, err)
		}
		session := profile.Sessions[0]
		status := companyServerStatus{Name: "Company", Map: session.MapFile, Version: item.version, Protocol: item.protocol, Time: "13:15", Players: 1, MaxPlayers: 16}
		if err := validateCompanyServer(status, session, MultiplayerTrip{Start: "13:15"}); err != nil {
			t.Fatal(item, err)
		}
	}
	if supportedMultiplayerProtocol(16) || startupTimeout != 60*time.Minute {
		t.Fatal("changed protocol allowlist or 60-minute startup allowance")
	}
}
