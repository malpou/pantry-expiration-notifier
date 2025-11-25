package notification

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/malpou/pantry-expiration-notifier/data"
	"github.com/malpou/pantry-expiration-notifier/i18n"
	"github.com/malpou/pantry-expiration-notifier/templates"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Headers  map[string]string
}

func SendEmail(cfg SMTPConfig, recipients []string, products []data.ExpiringProduct, translations i18n.Translations) error {
	var body bytes.Buffer

	err := templates.ExpirationEmail(products, translations).Render(context.Background(), &body)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subjectText := fmt.Sprintf(translations.Get("email_subject"), len(products))
	subject := fmt.Sprintf("=?UTF-8?B?%s?=",
		base64.StdEncoding.EncodeToString([]byte(subjectText)))

	msg := buildMIMEMessage(cfg.From, recipients, subject, body.String(), cfg.Headers)

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	return smtp.SendMail(addr, auth, cfg.From, recipients, msg)
}

func buildMIMEMessage(from string, to []string, subject, htmlBody string, headers map[string]string) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	for headerName, headerValue := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", headerName, headerValue))
	}
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	return buf.Bytes()
}
