package main

import (
	"bufio"
	"fmt"
	"strings"
)

type setupUI struct {
	input     *bufio.Scanner
	lang, dir string
}

func (u *setupUI) say(pt, en, de string) { fmt.Println(localText(u.lang, pt, en, de)) }
func (u *setupUI) line(pt, en, de, defaultValue string) (string, error) {
	fmt.Print(localText(u.lang, pt, en, de))
	if defaultValue != "" {
		fmt.Printf(" [%s]", defaultValue)
	}
	fmt.Print(": ")
	if !u.input.Scan() {
		return "", fmt.Errorf("input closed")
	}
	s := strings.TrimSpace(u.input.Text())
	if s == "" {
		s = defaultValue
	}
	return s, nil
}
func (u *setupUI) yes(pt, en, de string) bool {
	s, err := u.line(pt, en, de, "")
	return err == nil && affirmativeAnswer(s)
}

func (u *setupUI) busAutoStart(previous bool) (bool, error) {
	value := localText(u.lang, "não", "no", "nein")
	if previous {
		value = localText(u.lang, "sim", "yes", "ja")
	}
	for {
		answer, err := u.line("Ligar o ônibus automaticamente? (sim/não; não = partida e IBIS manuais)", "Start the bus automatically? (yes/no; no = manual startup and IBIS)", "Bus automatisch starten? (ja/nein; nein = Bus und IBIS selbst vorbereiten)", value)
		if err != nil {
			return previous, err
		}
		if affirmativeAnswer(answer) {
			return true, nil
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "n", "no", "nao", "não", "nein":
			return false, nil
		}
		u.say("Responda sim ou não.", "Answer yes or no.", "Antworte mit ja oder nein.")
	}
}
