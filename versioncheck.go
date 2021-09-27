package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"

	log "github.com/sirupsen/logrus"

	"bakinbacon/notifications"
)

const (
	VERSION_URL = "https://bakingbacon.github.io/bakinbacon/version.json"
)

var (
	commitHash string
	version    = "v0.7.0"
)

type Versions []Version

type Version struct {
	Date    time.Time `json:"date"`
	Version string    `json:"version"`
	Notes   string    `json:"notes"`
}

func RunVersionCheck() {

	// Check every 12hrs
	ticker := time.NewTicker(24 * time.Hour)

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

			// Assume JSON is in version order, get latest entry
			latestVersion := versions[0]

			// If newer version available, send notification
			if semver.Compare(version, latestVersion.Version) == -1 {
				notifications.N.Send(fmt.Sprintf("A new version, %s, of Bakin'Bacon is available! You are currently running %s.",
					latestVersion, version), notifications.VERSION)
			}

		}

		// wait here for next iteration
		<-ticker.C
	}
}
