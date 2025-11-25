package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/malpou/pantry-expiration-notifier/i18n"
	"github.com/malpou/pantry-expiration-notifier/notification"
	"go.uber.org/zap"
)

type Config struct {
	SpreadsheetID    string
	SheetName        string
	RecipientEmails  []string
	NotificationDays []int
	Language         string
	Translations     i18n.Translations
	SMTP             notification.SMTPConfig
	SACredentials    []byte
}

func Load(logger *zap.Logger) (Config, error) {
	port, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	saCredsRaw := mustEnv("GOOGLE_SA_CREDENTIALS", logger)
	var saCreds []byte
	if decoded, err := base64.StdEncoding.DecodeString(saCredsRaw); err == nil {
		saCreds = decoded
	} else {
		saCreds = []byte(saCredsRaw)
	}

	// Parse comma-separated emails
	emailsRaw := mustEnv("RECIPIENT_EMAILS", logger)
	var emails []string
	for email := range strings.SplitSeq(emailsRaw, ",") {
		email = strings.TrimSpace(email)
		if email != "" {
			emails = append(emails, email)
		}
	}
	if len(emails) == 0 {
		return Config{}, fmt.Errorf("RECIPIENT_EMAILS must contain at least one email")
	}

	// Parse SMTP headers in format "Header-Name: value, Another-Header: value"
	smtpHeaders := make(map[string]string)
	if headersRaw := getEnv("SMTP_HEADERS", ""); headersRaw != "" {
		for pair := range strings.SplitSeq(headersRaw, ",") {
			pair = strings.TrimSpace(pair)
			if parts := strings.SplitN(pair, ":", 2); len(parts) == 2 {
				headerName := strings.TrimSpace(parts[0])
				headerValue := strings.TrimSpace(parts[1])
				if headerName != "" && headerValue != "" {
					smtpHeaders[headerName] = headerValue
				}
			}
		}
	}

	// Parse notification days (comma-separated integers)
	notificationDays := []int{90, 60, 30, 14, 7, 3, 1} // default values
	if daysRaw := getEnv("NOTIFICATION_DAYS", ""); daysRaw != "" {
		var days []int
		for day := range strings.SplitSeq(daysRaw, ",") {
			day = strings.TrimSpace(day)
			if d, err := strconv.Atoi(day); err == nil && d > 0 {
				days = append(days, d)
			}
		}
		if len(days) > 0 {
			notificationDays = days
		}
	}

	// Load translations
	language := getEnv("LANGUAGE", "en")
	translations, err := i18n.Load(language, logger)
	if err != nil {
		return Config{}, fmt.Errorf("failed to load translations: %w", err)
	}

	return Config{
		SpreadsheetID:    mustEnv("SPREADSHEET_ID", logger),
		SheetName:        getEnv("SHEET_NAME", "Sheet1"),
		RecipientEmails:  emails,
		NotificationDays: notificationDays,
		Language:         language,
		Translations:     translations,
		SACredentials:    saCreds,
		SMTP: notification.SMTPConfig{
			Host:     mustEnv("SMTP_HOST", logger),
			Port:     port,
			Username: mustEnv("SMTP_USERNAME", logger),
			Password: mustEnv("SMTP_PASSWORD", logger),
			From:     mustEnv("SMTP_FROM", logger),
			Headers:  smtpHeaders,
		},
	}, nil
}

func mustEnv(key string, logger *zap.Logger) string {
	val := os.Getenv(key)
	if val == "" {
		logger.Fatal("Required environment variable is not set", zap.String("variable", key))
	}
	return val
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
