package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func companyHostInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt + ": ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(line), `"`), nil
}

func companyHostLocalSettingsPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(executable), "CompanyHost.local.json")
}

func loadCompanyHostLocalOptions(path string) (companyHostOptions, bool) {
	var options companyHostOptions
	text, err := os.ReadFile(path)
	if err != nil || len(text) > 16<<10 {
		return options, false
	}
	if err := json.Unmarshal(text, &options); err != nil || !filepath.IsAbs(options.Server) || !filepath.IsAbs(options.Root) || !filepath.IsAbs(options.Profile) || options.Session == "" || strings.Contains(options.Profile, "://") {
		return companyHostOptions{}, false
	}
	return options, true
}

func saveCompanyHostLocalOptions(path string, options companyHostOptions) error {
	if path == "" {
		return fmt.Errorf("cannot locate CompanyHost.local.json")
	}
	text, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "company-host-options-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(append(text, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func interactiveCompanyHost(reader *bufio.Reader) (companyHostOptions, error) {
	var options companyHostOptions
	fmt.Println("OpenOmsi + BBS — servidor com relógio da empresa")
	fmt.Println("Cole os caminhos completos. Não precisa digitar comandos do CMD nem editar o relógio do Windows.")
	if previous, found := loadCompanyHostLocalOptions(companyHostLocalSettingsPath()); found {
		fmt.Printf("Configuração anterior:\nServidor: %s\nOMSI 2: %s\nPerfil: %s\nSessão: %s\n", previous.Server, previous.Root, previous.Profile, previous.Session)
		choice, err := companyHostInput(reader, "Enter para iniciar com estes caminhos; digite n para configurar novamente")
		if err != nil {
			return options, err
		}
		if choice == "" || affirmativeAnswer(choice) {
			return previous, nil
		}
	}
	var err error
	if options.Server, err = companyHostInput(reader, "Pasta do servidor dedicado ou caminho do openomsi.exe"); err != nil {
		return options, err
	}
	if options.Server == "" {
		return options, fmt.Errorf("informe a pasta do servidor dedicado")
	}
	options.Server, err = filepath.Abs(options.Server)
	if err != nil {
		return options, err
	}
	if st, err := os.Stat(options.Server); err == nil && st.IsDir() {
		options.Server = filepath.Join(options.Server, "openomsi.exe")
	}
	if options.Root, err = companyHostInput(reader, "Pasta original do OMSI 2 (contém Omsi.exe)"); err != nil {
		return options, err
	}
	if options.Root == "" {
		return options, fmt.Errorf("informe a pasta original do OMSI 2")
	}
	options.Root, err = filepath.Abs(options.Root)
	if err != nil {
		return options, err
	}
	if options.Profile, err = companyHostInput(reader, "Caminho do perfil .json criado pelo Setup"); err != nil {
		return options, err
	}
	if options.Profile == "" {
		return options, fmt.Errorf("informe o perfil .json da empresa")
	}
	if strings.Contains(options.Profile, "://") {
		return options, fmt.Errorf("o host precisa de um perfil .json local")
	}
	options.Profile, err = filepath.Abs(options.Profile)
	if err != nil {
		return options, err
	}
	profile, err := loadCompanyProfile(context.Background(), options.Profile, ".", companyHostClient())
	if err != nil {
		return options, err
	}
	if profile.Clock == nil {
		return options, fmt.Errorf("o perfil precisa do relógio da empresa; configure-o no Setup atualizado")
	}
	civil, err := companyNow(*profile.Clock, time.Now())
	if err != nil {
		return options, err
	}
	fmt.Printf("Empresa: %s\nRelógio: %s %+d minutos\nData e hora agora: %s\n", profile.CompanyName, profile.Clock.TimeZone, profile.Clock.ShiftMinutes, civil.Format("2006-01-02 15:04:05"))
	for i, session := range profile.Sessions {
		fmt.Printf("%d - %s [%s] (%s)\n", i+1, session.Name, session.ID, session.Date)
	}
	if len(profile.Sessions) == 1 {
		options.Session = profile.Sessions[0].ID
	} else {
		choice, err := companyHostInput(reader, "Número da sessão")
		if err != nil {
			return options, err
		}
		n, err := strconv.Atoi(choice)
		if err != nil || n < 1 || n > len(profile.Sessions) {
			return options, fmt.Errorf("número de sessão inválido")
		}
		options.Session = profile.Sessions[n-1].ID
	}
	options.Config = filepath.Join(filepath.Dir(options.Server), "server.cfg")
	fmt.Printf("Configuração original: %s\n", options.Config)
	fmt.Println("O servidor anterior precisa estar fechado. A configuração original será preservada.")
	_, err = companyHostInput(reader, "Pressione Enter para iniciar e manter o relógio sincronizado")
	return options, err
}

func main() {
	var options companyHostOptions
	interactive := len(os.Args) == 1
	reader := bufio.NewReader(os.Stdin)
	flag.StringVar(&options.Profile, "profile", "", "Local JSON profile of the company")
	flag.StringVar(&options.Session, "session", "", "Session ID from the company profile")
	flag.StringVar(&options.Server, "server", "", "Absolute path to the dedicated openomsi.exe")
	flag.StringVar(&options.Root, "root", "", "Absolute path to the original OMSI 2 folder")
	flag.StringVar(&options.Config, "config", "", "Existing server.cfg (default: beside the server executable)")
	flag.Parse()
	if interactive {
		var err error
		options, err = interactiveCompanyHost(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "CompanyHost:", err)
			_, _ = companyHostInput(reader, "Pressione Enter para fechar")
			os.Exit(1)
		}
		savedOptions := options
		options.onReady = func() {
			if err := saveCompanyHostLocalOptions(companyHostLocalSettingsPath(), savedOptions); err != nil {
				fmt.Fprintln(os.Stderr, "Não foi possível lembrar os caminhos para a próxima execução:", err)
			}
		}
	}
	if flag.NArg() != 0 || options.Profile == "" || options.Session == "" || options.Server == "" || options.Root == "" {
		flag.Usage()
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runCompanyHost(ctx, options, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "CompanyHost:", err)
		if interactive {
			_, _ = companyHostInput(reader, "Pressione Enter para fechar")
		}
		os.Exit(1)
	}
}
