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
	email    = "email"
)

type Notifier interface {
	Send(string)
	IsEnabled() bool
}

type NotificationHandler struct {
	Notifiers        map[string]Notifier
	lastSentCategory map[Category]time.Time
	Storage          *storage.Storage
}

func NewHandler(db *storage.Storage) (*NotificationHandler, error) {
	n := &NotificationHandler{
		Notifiers:         make(map[string]Notifier),
		lastSentCategory: make(map[Category]time.Time),
		Storage:          db,
	}

	if err := n.LoadNotifiers(); err != nil {
		return nil, errors.Wrap(err, "Failed to instantiate notification handler")
	}

	return n, nil
}

func (n *NotificationHandler) LoadNotifiers() error {
	// Get telegram notifications config from DB, as []byte string
	tConfig, err := n.Storage.GetNotifiersConfig("telegram")
	if err != nil {
		return errors.Wrap(err, "Unable to load telegram config")
	}

	// Configure telegram; Don't save what we just loaded
	if err := n.Configure("telegram", tConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init telegram")
	}

	// Get email notifications config from DB
	eConfig, err := n.Storage.GetNotifiersConfig("email")
	if err != nil {
		return errors.Wrap(err, "Unable to load email config")
	}

	// Configure email; Don't save what we just loaded
	if err := n.Configure("email", eConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init email")
	}

	return nil
}

func (n *NotificationHandler) Configure(notifier string, config []byte, saveConfig bool) error {
	switch notifier {
	case telegram:
		nt, err := n.NewTelegram(config, saveConfig)
		if err != nil {
			return err
		}
		n.Notifiers[telegram] = nt

	case email:
		ne, err := n.NewEmail(config, saveConfig)
		if err != nil {
			return err
		}
		n.Notifiers[email] = ne

	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (n *NotificationHandler) Send(message string, category Category) {
	// Check that we haven't sent a message from this category
	// within the past 10 minutes
	if lastSentTime, ok := n.lastSentCategory[category]; ok {
		if lastSentTime.After(time.Now().UTC().Add(time.Minute * -10)) {
			log.Info("NotificationHandler last sent within 10 minutes")
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

func (n *NotificationHandler) TestSend(notifier string, message string) error {
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

func (n *NotificationHandler) GetConfig() (json.RawMessage, error) {
	// Marshal the current Notifiers as the current config
	// Return RawMessage so as not to double Marshal
	bts, err := json.Marshal(n.Notifiers)
	return json.RawMessage(bts), err
}
