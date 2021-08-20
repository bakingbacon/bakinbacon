package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"

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

		// Anon func to get defer ability
		err := func() error {
			resp, err := client.Get(VERSION_URL)
			if err != nil {
				return errors.Wrap(err, "Unable to get version update")
			}
			defer resp.Body.Close()

			if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
				return errors.Wrap(err, "Unable to decode version check")
			}
			return nil
		}()
		if err != nil {
			log.WithError(err).Error("Error checking version")
		} else {

			// Just log for now
			for _, v := range versions {
				log.WithFields(log.Fields{
					"Date": v.Date, "Version": v.Version, "Notes": v.Notes,
				}).Info("Version Update")
			}
		}

		// wait here for next iteration
		<-ticker.C
	}
}
