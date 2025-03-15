package main

import (
	"fmt"
	"github.com/barny-dev/ceslav/internal/commands"
	"io"
	"log"
	"os"
)

func main() {
	logFilePath := os.Getenv("CESLAV_LOG")
	if logFilePath != "" {
		//logFile :=
		setupLogFile(logFilePath)
		log.Printf("ceslav log file initialized\n")
		//defer panicIfErr(logFile.Close())
	} else {
		log.SetOutput(io.Discard)
	}
	if err := commands.Cmd().Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		panicIfErr(err)
		os.Exit(1)
	}
}

func setupLogFile(logFilePath string) *os.File {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "could not open log file: %s\n", logFilePath)
		panicIfErr(err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	return logFile
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
