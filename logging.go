package main

import (
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
)

var logFile *os.File

func setupLogging(logDebug bool, logTrace bool) {

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to determine working directory: %s", err)
	}

	runID := time.Now().Format("log-bakinbacon-2006-01-02")
	logLocation := filepath.Join(cwd, runID+".log")

	logFile, err = os.OpenFile(logLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s for output: %s", logLocation, err)
	}

	if logDebug {
		log.SetLevel(log.DebugLevel)
	}

	if logTrace {
		log.SetLevel(log.TraceLevel)
	}

	// Write everything to log file too
	log.AddHook(&writer.Hook{
		Writer: logFile,
		LogLevels: []log.Level{
			log.TraceLevel,
			log.DebugLevel,
			log.InfoLevel,
			log.WarnLevel,
			log.ErrorLevel,
			log.FatalLevel,
			log.PanicLevel,
		},
		Formatter: &log.TextFormatter{
			FullTimestamp: true,
			DisableColors: true,
		},
	})
}

func closeLogging() {
	logFile.Close()
}
