package notifications

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"bakinbacon/storage"
)

type NotifyEmail struct {
	Username string `json:"username"`
	Password string `json:"password"`
	SMTPHost string `json:"smtphost"`
	SMTPport int    `json:"smtpport"`
	Enabled  bool   `json:"enabled"`
	Storage  *storage.Storage
}

func (n *NotificationHandler) NewEmail(config []byte, saveConfig bool) (*NotifyEmail, error) {
	return &NotifyEmail{
		Enabled: true,
		Storage: n.Storage,
	}, nil
}

func (n *NotifyEmail) IsEnabled() bool {
	return n.Enabled
}

func (n *NotifyEmail) Send(msg string) {
	// TODO Not implemented yet
	log.Warn("email notifications not yet implemented")
}

func (n *NotifyEmail) SaveConfig() error {
	// Marshal ourselves to []byte and send to storage manager
	config, err := json.Marshal(n)
	if err != nil {
		return errors.Wrap(err, "Unable to marshal email config")
	}

	if err := n.Storage.SaveNotifiersConfig("email", config); err != nil {
		return errors.Wrap(err, "Unable to save email config")
	}

	return nil
}
