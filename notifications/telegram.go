package notifications

import (
	"strings"
	"regexp"
)

type NotifTelegram struct {
	chatIds []string `json:"chatids"`
	apiKey  string   `json:"apikey"`
	enabled bool     `json:"enabled"`
}

func NewTelegram(chatIds string, botKey string, enabled bool) (*NotifTelegram, error) {

	ids := strings.TrimRight(chatIds, ", ") // Chop off trailing space or comma

	n := &NotifTelegram{}

	// split and assign
	n.chatIds = regexp.MustCompile(`[,\s]+`).Split(ids, -1)
	n.apiKey = botKey
	n.enabled = enabled

	return n, nil
}

func (n NotifTelegram) Send(msg string) error {

	return nil
}
