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
	BALANCE Category = iota + 1
	SIGNER
	BAKING_OK
	BAKING_FAIL
	ENDORSE_FAIL
	VERSION
	NONCE
	PAYOUTS

	TELEGRAM = "telegram"
	EMAIL    = "email"
)

type Notifier interface {
	Send(string)
	IsEnabled() bool
}

type NotificationHandler struct {
	notifiers        map[string]Notifier
	lastSentCategory map[Category]time.Time
	storage          *storage.Storage
}

func NewHandler(db *storage.Storage) (*NotificationHandler, error) {

	n := &NotificationHandler{
		notifiers:        make(map[string]Notifier),
		lastSentCategory: make(map[Category]time.Time),
		storage:          db,
	}

	if err := n.LoadNotifiers(); err != nil {
		return nil, errors.Wrap(err, "Failed to instantiate notification handler")
	}

	return n, nil
}

func (n *NotificationHandler) LoadNotifiers() error {

	// Get telegram notifications config from DB, as []byte string
	telegramConfig, err := n.storage.GetNotifiersConfig(TELEGRAM)
	if err != nil {
		return errors.Wrap(err, "Unable to load telegram config")
	}

	// Configure telegram; Don't save what we just loaded
	if err := n.Configure(TELEGRAM, telegramConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init telegram")
	}

	// Get email notifications config from DB
	emailConfig, err := n.storage.GetNotifiersConfig(EMAIL)
	if err != nil {
		return errors.Wrap(err, "Unable to load email config")
	}

	// Configure email; Don't save what we just loaded
	if err := n.Configure(EMAIL, emailConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init email")
	}

	return nil
}

func (n *NotificationHandler) Configure(notifier string, config []byte, saveConfig bool) error {

	switch notifier {
	case TELEGRAM:
		nt, err := n.NewTelegram(config, saveConfig)
		if err != nil {
			return err
		}
		n.notifiers[TELEGRAM] = nt

	case EMAIL:
		ne, err := n.NewEmail(config, saveConfig)
		if err != nil {
			return err
		}
		n.notifiers[EMAIL] = ne

	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (n *NotificationHandler) SendNotification(message string, category Category) {

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

	for k, n := range n.notifiers {
		if n.IsEnabled() {
			n.Send(message)
		} else {
			log.Infof("Notifications for '%s' are disabled", k)
		}
	}
}

func (n *NotificationHandler) TestSend(notifier string, message string) error {

	switch notifier {
	case TELEGRAM:
		n.notifiers[TELEGRAM].Send(message)
	case EMAIL:
		n.notifiers[EMAIL].Send(message)
	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (n *NotificationHandler) GetConfig() (json.RawMessage, error) {

	// Marshal the current Notifiers as the current config
	// Return RawMessage so as not to double Marshal
	bts, err := json.Marshal(n.notifiers)
	return json.RawMessage(bts), err
}
