package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"

	log "github.com/sirupsen/logrus"

	"bakinbacon/notifications"
)

const (
	VERSION_URL = "https://bakingbacon.github.io/bakinbacon/version.json"
	STATS_URL   = "https://bakinbacon.io/stats.php"
)

var (
	commitHash string
	version    = "v1.0.0"
)

type Versions []Version

type Version struct {
	Date    time.Time `json:"date"`
	Version string    `json:"version"`
	Notes   string    `json:"notes"`
}

func (bb *BakinBacon) RunVersionCheck() {

	// Check every 8hrs
	ticker := time.NewTicker(8 * time.Hour)

	for {

		bb.submitAnonymousStats()

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

			// Assume JSON is in version order, get the latest entry
			latestVersion := versions[0]

			// If newer version available, send notification
			if semver.Compare(version, latestVersion.Version) == -1 {
				bb.NotificationHandler.SendNotification(fmt.Sprintf("A new version, %s, of Bakin'Bacon is available! You are currently running %s.",
					latestVersion, version), notifications.VERSION)
			}
		}

		// wait here for next iteration
		<-ticker.C
	}
}

func (bb *BakinBacon) submitAnonymousStats() {

	// Get PKH of baker
	_, pkh, err := bb.Signer.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Unable to get PKH for stats")
		return
	}

	// create hash of PKH to anonymize the info
	uuid := fmt.Sprintf("%x", md5.Sum([]byte(pkh)))

	// json of stats
	stats, _ := json.Marshal(map[string]string{
		"uuid":  uuid,
		"os":    fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
		"chash": commitHash,
		"bbver": version,
	})

	statsPostBody := bytes.NewBuffer(stats)
	resp, err := http.Post(STATS_URL, "application/json", statsPostBody)
	if err != nil {
		log.WithError(err).Error("Unable to post stats")
		return
	}
	defer resp.Body.Close()

	log.Debug("Posted anonymous stats")
}
