package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/evilsocket/islazy/tui"
	log "github.com/sirupsen/logrus"
)

func LineCounter(f string) (int, error) {
	file, err := os.Open(f)
	if err != nil {
		return -1, err
	}
	r := bufio.NewReader(file)
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, nil
		}
	}
}

func CheckErr(err error, level, text string) {
	if err != nil {
		Logg(fmt.Sprint(tui.Red(text), err), level)
	}
}

func Logg(text, level string) {
	switch level {
	case "Fatal":
		msg = tui.Red(text)
		log.Fatal(msg)
	case "Error":
		msg = tui.Red(text)
		log.Error(msg)
	case "Warn":
		msg = tui.Yellow(text)
		log.Warn(msg)
	case "Info":
		msg = tui.Green(text)
		log.Info(msg)
	case "Debug":
		msg = tui.Blue(text)
		log.Debug(msg)
	}
}
