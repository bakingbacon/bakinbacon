package notifications

import (
	"encoding/json"

	"github.com/pkg/errors"

	"bakinbacon/storage"
)

type NotifyEmail struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Smtp_host string `json:"smtphost"`
	Smtp_port int    `json:"smtpport"`
	Enabled   bool   `json:"enabled"`
}

func NewEmail(config []byte, saveConfig bool) (*NotifyEmail, error) {

	ne := &NotifyEmail{}

	return ne, nil
}

func (n NotifyEmail) Send(msg string) error {

	return nil
}

func (n NotifyEmail) SaveConfig() error {

	// Marshal ourselves to []byte and send to storage manager
	config, err := json.Marshal(n)
	if err != nil {
		return errors.Wrap(err, "Unable to marshal email config")
	}

	if err := storage.DB.SaveNotifiersConfig("email", config); err != nil {
		return errors.Wrap(err, "Unable to save email config")
	}

	return nil
}
