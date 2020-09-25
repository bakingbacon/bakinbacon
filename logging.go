package main

import (
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
)

func setupLogging(logDebug bool) {

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		DisableSorting: true,
	})

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to determine working directory: %s", err)
	}
	runID := time.Now().Format("goendorse-2006-01-02-15-04-05")
	logLocation := filepath.Join(cwd, runID + ".log")

	logFile, err := os.OpenFile(logLocation, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s for output: %s", logLocation, err)
	}

	if logDebug {
		log.SetLevel(log.DebugLevel)
	}

	// Write everything to log file too
	log.AddHook(&writer.Hook{
		Writer: logFile,
		LogLevels: []log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
			log.WarnLevel,
			log.InfoLevel,
			log.DebugLevel,
		},
	})
}