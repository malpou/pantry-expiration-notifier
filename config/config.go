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
	emails := splitTrimmed(emailsRaw, ",")
	if len(emails) == 0 {
		return Config{}, fmt.Errorf("RECIPIENT_EMAILS must contain at least one valid email address, got: %q", emailsRaw)
	}

	// Parse SMTP headers in format "Header-Name: value, Another-Header: value"
	smtpHeaders := make(map[string]string)
	if headersRaw := getEnv("SMTP_HEADERS", ""); headersRaw != "" {
		for _, pair := range splitTrimmed(headersRaw, ",") {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				return Config{}, fmt.Errorf("SMTP_HEADERS contains invalid header %q: expected format 'Name: value'", pair)
			}
			headerName := strings.TrimSpace(parts[0])
			headerValue := strings.TrimSpace(parts[1])
			if headerName == "" || headerValue == "" {
				return Config{}, fmt.Errorf("SMTP_HEADERS contains invalid header %q: name and value must be non-empty", pair)
			}
			smtpHeaders[headerName] = headerValue
		}
	}

	// Parse notification days (comma-separated positive integers)
	notificationDays := []int{90, 60, 30, 14, 7, 3, 1} // default values
	if daysRaw := getEnv("NOTIFICATION_DAYS", ""); daysRaw != "" {
		var days []int
		for _, dayStr := range splitTrimmed(daysRaw, ",") {
			d, err := strconv.Atoi(dayStr)
			if err != nil {
				return Config{}, fmt.Errorf("NOTIFICATION_DAYS contains invalid integer %q: %w", dayStr, err)
			}
			if d <= 0 {
				return Config{}, fmt.Errorf("NOTIFICATION_DAYS contains non-positive value %d: must be greater than 0", d)
			}
			days = append(days, d)
		}
		if len(days) == 0 {
			return Config{}, fmt.Errorf("NOTIFICATION_DAYS must contain at least one value, got: %q", daysRaw)
		}
		notificationDays = days
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

func splitTrimmed(s, sep string) []string {
	var result []string
	for part := range strings.SplitSeq(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
