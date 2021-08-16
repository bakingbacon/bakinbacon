package main

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	VERSION_URL = "https://bakingbacon.github.io/bakinbacon/version.json"
)

type Versions []Version

type Version struct {
	Date    time.Time `json:"date"`
	Version string    `json:"version"`
	Notes   string    `json:"notes"`
}

func RunVersionCheck() {

	// Check every 12hrs
	ticker := time.NewTicker(12 * time.Hour)

	for {

		versions := Versions{}

		log.Info("Checking version...")

		// HTTP client 10s timeout
		client := &http.Client{
			Timeout: time.Second * 10,
		}

		resp, err := client.Get(VERSION_URL)
		if err != nil {
			log.WithError(err).Error("Unable to get version update")
		}
		defer resp.Body.Close()
		json.NewDecoder(resp.Body).Decode(&versions)

		// Just log for now
		for _, v := range versions {
			log.WithFields(log.Fields{
				"Date": v.Date, "Version": v.Version, "Notes": v.Notes,
			}).Info("Version Update")
		}

		// wait here for next iteration
		select {
		case <-ticker.C:
			break
		}
	}
}
