package notifications

type NotifEmail struct {
	username  string `json:"username"`
	password  string `json:"password"`
	smtp_host string `json:"smtphost"`
	smtp_port int    `json:"smtpport"`
}

func (n NotifEmail) Send(msg string) error {

	return nil
}
