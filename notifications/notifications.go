package notifications

import (
	"encoding/json"

	"github.com/pkg/errors"

	//log "github.com/sirupsen/logrus"

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

	// Get telegram notifications config from DB, as []byte string
	tConfig, err := storage.DB.GetNotifiersConfig("telegram")
	if err != nil {
		return errors.Wrap(err, "Unable to load telegram config")
	}

	// Configure telegram; Don't save what we just loaded
	if err := N.Configure("telegram", tConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init telegram")
	}

	// Get email notifications config from DB
	eConfig, err := storage.DB.GetNotifiersConfig("email")
	if err != nil {
		return errors.Wrap(err, "Unable to load email config")
	}

	// Configure email; Don't save what we just loaded
	if err := N.Configure("email", eConfig, false); err != nil {
		return errors.Wrap(err, "Unable to init email")
	}

	return nil
}

func (N *Notification) Configure(notifier string, config []byte, saveconfig bool) error {

	switch notifier {
	case "telegram":
		nt, err := NewTelegram(config, saveconfig)
		if err != nil {
			return err
		}
		N.Notifiers["telegram"] = nt

	case "email":
		ne, err := NewEmail(config, saveconfig)
		if err != nil {
			return err
		}
		N.Notifiers["email"] = ne

	default:
		return errors.New("Unknown notification type")
	}

	return nil
}

func (N *Notification) Send(notifier string, message string) error {

	switch notifier {
	case "telegram":
		return N.Notifiers["telegram"].Send(message)
	case "email":
		return N.Notifiers["email"].Send(message)
	}

	return errors.New("Unknown notification type")
}

func (N *Notification) GetConfig() (json.RawMessage, error) {

	// Marshal the current Notifiers as the current config
	// Return RawMessage so as not to double Marshal
	bts, err := json.Marshal(N.Notifiers)
	return json.RawMessage(bts), err
}
