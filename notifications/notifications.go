package notifications

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type Category int

const (
	Balance Category = iota + 1
	Signer
	BakingOk
	BakingFail
	EndorseFail
	Version
	Nonce

	telegram = "telegram"
	email = "email"
)

type Notifier interface {
	Send(string)
	IsEnabled() bool
}

type Notification struct {
	Notifiers        map[string]Notifier
	lastSentCategory map[Category]time.Time
}

var N *Notification

func New() error {

	N = new(Notification)
	N.Notifiers = make(map[string]Notifier)
	N.lastSentCategory = make(map[Category]time.Time)

	if err := N.LoadNotifiers(); err != nil {
		return errors.Wrap(err, "Failed New Notification")
	}

	return nil
}

func (n *Notification) LoadNotifiers() error {

	// Get telegram notifications config from DB, as []byte string
	tConfig, err := storage.DB.GetNotifiersConfig("telegram")
	if err != nil {
		return errors.Wrap(err, "Unable to load telegram config")
	}

	// Configure telegram; Don't save what we just loaded
	if err := n.Configure("telegram", tConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init telegram")
	}

	// Get email notifications config from DB
	eConfig, err := storage.DB.GetNotifiersConfig("email")
	if err != nil {
		return errors.Wrap(err, "Unable to load email config")
	}

	// Configure email; Don't save what we just loaded
	if err := n.Configure("email", eConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init email")
	}

	return nil
}

func (n *Notification) Configure(notifier string, config []byte, saveConfig bool) error {

	switch notifier {
	case telegram:
		nt, err := NewTelegram(config, saveConfig)
		if err != nil {
			return err
		}
		n.Notifiers[telegram] = nt

	case email:
		ne, err := NewEmail(config, saveConfig)
		if err != nil {
			return err
		}
		n.Notifiers[email] = ne

	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (n *Notification) Send(message string, category Category) {
	// Check that we haven't sent a message from this category
	// within the past 10 minutes
	if lastSentTime, ok := n.lastSentCategory[category]; ok {
		if lastSentTime.After(time.Now().UTC().Add(time.Minute * -10)) {
			log.Info("Notification last sent within 10 minutes")
			return
		}
	}

	// Add/update notification timestamp for category
	n.lastSentCategory[category] = time.Now().UTC()

	for k, n := range n.Notifiers {
		if n.IsEnabled() {
			n.Send(message)
		} else {
			log.Infof("Notifications for '%s' are disabled", k)
		}
	}
}

func (n *Notification) TestSend(notifier string, message string) error {
	switch notifier {
	case telegram:
		n.Notifiers[telegram].Send(message)
	case email:
		n.Notifiers[email].Send(message)
	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (n *Notification) GetConfig() (json.RawMessage, error) {
	// Marshal the current Notifiers as the current config
	// Return RawMessage so as not to double Marshal
	bts, err := json.Marshal(n.Notifiers)
	return json.RawMessage(bts), err
}
