package notifications

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type Notifier interface {
	Send(string) error
}

type Notification struct {
	Notifiers map[string]Notifier
}

var N *Notification

func New() error {

	N = &Notification{}
	N.Notifiers = make(map[string]Notifier, 2)

	if err := N.LoadNotifiers(); err != nil {
		return errors.Wrap(err, "Failed New Notification")
	}

	return nil
}

func (N *Notification) LoadNotifiers() error {

	nt := NotifTelegram{}
	ne := NotifEmail{}

	// Get telegram notifications config from DB, as []byte string
	tConfig, err := storage.DB.GetNotifiersConfig("telegram")
	if err != nil {
		return errors.Wrap(err, "Unable to load telegram config")
	}

	// Unmarshal the byte-string to object
	if tConfig != nil {
		if err := json.Unmarshal(tConfig, &nt); err != nil {
			return errors.Wrap(err, "Unable to unmarshal telegram config")
		}
		log.WithField("Config", nt).Info("Loaded telegram notifier")
	}

	// Get email notifications config from DB
	eConfig, err := storage.DB.GetNotifiersConfig("email")
	if err != nil {
		return errors.Wrap(err, "Unable to load email config")
	}

	// Unmarshal the byte-string to object
	if eConfig != nil {
		if err := json.Unmarshal(eConfig, &ne); err != nil {
			return errors.Wrap(err, "Unable to unmarshal email config")
		}
		log.WithField("Config", ne).Info("Loaded email notifier")
	}

	// Assign notifiers
	N.Notifiers["telegram"] = nt
	N.Notifiers["email"] = ne

	return nil
}

func (N *Notification) GetConfig() (json.RawMessage, error) {

	// Marshal the current Notifiers as the current config
	// Return RawMessage so as not to double Marshal
	bts, err := json.Marshal(N.Notifiers)
	return json.RawMessage(bts), err
}

func (N *Notification) Configure(config map[string]string) error {

	switch(config["type"]) {
	case "telegram":

		chatIds, ok := config["chatids"]
		if !ok {
			return errors.New("Missing chatids")
		}

		botKey, ok := config["botkey"]
		if !ok {
			return errors.New("Missing bot API key")
		}

		enabled, err := strconv.ParseBool(config["enabled"])
		if err != nil {
			return errors.New("Unable to parse enabled")
		}

		_, err = NewTelegram(chatIds, botKey, enabled)
		if err != nil {
			return err
		}

	case "email":
	default:
		return errors.New("Unknown notification type")
	}

	return nil
}
