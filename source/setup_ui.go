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
