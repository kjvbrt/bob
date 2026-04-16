package email

import (
	"net/smtp"
	"os"
	"strings"
)

type Config struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func ConfigFromEnv() Config {
	return Config{
		Host: os.Getenv("SMTP_HOST"),
		Port: os.Getenv("SMTP_PORT"),
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: os.Getenv("SMTP_FROM"),
	}
}

func (c Config) Enabled() bool {
	return c.Host != "" && c.From != ""
}

func (c Config) Send(to, subject, body string) error {
	if !c.Enabled() {
		return nil
	}
	port := c.Port
	if port == "" {
		port = "587"
	}
	addr := c.Host + ":" + port

	msg := strings.Join([]string{
		"From: Bob the Tracker <" + c.From + ">",
		"To: " + to,
		"Subject: " + subject,
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if c.User != "" {
		auth = smtp.PlainAuth("", c.User, c.Pass, c.Host)
	}

	return smtp.SendMail(addr, auth, c.From, []string{to}, []byte(msg))
}
