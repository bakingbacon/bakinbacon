package notifications

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type NotifyTelegram struct {
	ChatIds []int  `json:"chatids"`
	ApiKey  string `json:"apikey"`
	Enabled bool   `json:"enabled"`

	storage *storage.Storage
}

// NewTelegram creates a new NotifyTelegram object using a JSON byte-stream
// provided from either DB lookup or web UI. The stream is unmarshaled into
// a new object which is returned.
//
// If saveConfig is true, save the new object's config to DB. Normally would not
// do this if we just loaded from DB on app startup, but would want to do this
// after getting new config from web UI.
func (n *NotificationHandler) NewTelegram(config []byte, saveConfig bool) (*NotifyTelegram, error) {

	nt := &NotifyTelegram{
		Enabled: true,
		storage: n.storage,
	}

	// empty config from db?
	if config != nil {
		if err := json.Unmarshal(config, nt); err != nil {
			return nt, errors.Wrap(err, "Unable to unmarshal telegram config")
		}
	}

	if saveConfig {
		if err := nt.SaveConfig(); err != nil {
			return nt, err
		}
	}

	return nt, nil
}

func (n *NotifyTelegram) IsEnabled() bool {
	return n.Enabled
}

func (n *NotifyTelegram) Send(msg string) {

	// curl -G \
	//  --data-urlencode "chat_id=111112233" \
	//  --data-urlencode "text=$message" \
	//  https://api.telegram.org/bot${TOKEN}/sendMessage

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.ApiKey), nil)
	if err != nil {
		log.WithError(err).Error("Unable to make Telegram request")
		return
	}

	req.Header.Add("Content-type", "application/x-www-form-urlencoded")

	// Query parameters
	q := req.URL.Query()
	q.Add("text", msg)

	// HTTP client 10s timeout
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	// Loop over chatIds, sending message
	for _, chatId := range n.ChatIds {

		q.Set("chat_id", strconv.Itoa(chatId))

		// Encode URL parameters
		req.URL.RawQuery = q.Encode()

		// Execute
		resp, err := client.Do(req)
		if err != nil {
			log.WithFields(log.Fields{
				"ChatId": chatId,
			}).WithError(err).Error("Unable to send Telegram message")
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"ChatId": chatId,
			}).WithError(err).Error("Unable to read Telegram API response")
		}

		log.WithField("Resp", string(body)).Debug("Telegram Reply")
	}

	log.WithField("MSG", msg).Info("Sent Telegram Message")
}

func (n *NotifyTelegram) SaveConfig() error {

	// Marshal ourselves to []byte and send to storage manager
	config, err := json.Marshal(n)
	if err != nil {
		return errors.Wrap(err, "Unable to marshal telegram config")
	}

	if err := n.storage.SaveNotifiersConfig(TELEGRAM, config); err != nil {
		return errors.Wrap(err, "Unable to save telegram config")
	}

	return nil
}
