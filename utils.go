package main

import (
	"bufio"
	"bytes"
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
		switch level {
		case "Error":
			msg = tui.Red(text)
			log.Error(msg, err)
		case "Warn":
			msg = tui.Yellow(text)
			log.Warn(msg, err)
		case "Info":
			msg = tui.Green(text)
			log.Info(msg, err)
		case "Debug":
			msg = tui.Blue(text)
			log.Debug(msg, err)
		}
	}
}
