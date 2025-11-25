package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/malpou/pantry-expiration-notifier/templates"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Config struct {
	SpreadsheetID    string
	SheetName        string
	RecipientEmails  []string
	NotificationDays []int
	Language         string
	Translations     map[string]string
	SMTP             SMTPConfig
	SACredentials    []byte
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Headers  map[string]string
}

type Product struct {
	Row        int
	Name       string
	Packaging  string
	Quantity   int
	Expiration time.Time
	SentDays   []int
}

type ExpiringProduct struct {
	Product
	DaysUntil int
	Threshold int
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	// Load .env file as fallback if environment variables aren't set
	_ = godotenv.Load()

	cfg, err := loadConfig(logger)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(cfg.SACredentials))
	if err != nil {
		logger.Fatal("Unable to create Sheets client", zap.Error(err))
	}

	resp, err := srv.Spreadsheets.Values.Get(cfg.SpreadsheetID, cfg.SheetName+"!A:E").Do()
	if err != nil {
		logger.Fatal("Unable to read sheet", zap.Error(err))
	}

	products := parseProducts(resp.Values)

	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		logger.Fatal("Failed to load timezone", zap.Error(err))
	}
	today := time.Now().In(loc).Truncate(24 * time.Hour)

	var expiring []ExpiringProduct
	var updates []sheets.ValueRange

	for _, p := range products {
		daysUntil := int(p.Expiration.Sub(today).Hours() / 24)

		for _, threshold := range cfg.NotificationDays {
			if daysUntil <= threshold && !slices.Contains(p.SentDays, threshold) {
				expiring = append(expiring, ExpiringProduct{
					Product:   p,
					DaysUntil: daysUntil,
					Threshold: threshold,
				})

				p.SentDays = append(p.SentDays, threshold)
				updates = append(updates, sheets.ValueRange{
					Range:  fmt.Sprintf("%s!E%d", cfg.SheetName, p.Row),
					Values: [][]any{{formatSentDays(p.SentDays)}},
				})
				break
			}
		}
	}

	if len(expiring) == 0 {
		logger.Info("No notifications to send")
		return
	}

	if err := sendEmail(cfg, expiring); err != nil {
		logger.Fatal("Failed to send email", zap.Error(err))
	}
	logger.Info("Sent notification",
		zap.Int("products", len(expiring)),
		zap.Int("recipients", len(cfg.RecipientEmails)))

	batchReq := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "RAW",
		Data:             make([]*sheets.ValueRange, len(updates)),
	}
	for i := range updates {
		batchReq.Data[i] = &updates[i]
	}
	if _, err = srv.Spreadsheets.Values.BatchUpdate(cfg.SpreadsheetID, batchReq).Do(); err != nil {
		logger.Fatal("Unable to update sheet", zap.Error(err))
	}
	logger.Info("Sheet updated successfully")
}

func loadConfig(logger *zap.Logger) (Config, error) {
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
	translations, err := loadTranslations(language, logger)
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
		SMTP: SMTPConfig{
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

func loadTranslations(lang string, logger *zap.Logger) (map[string]string, error) {
	// Define required translation keys
	requiredKeys := []string{
		"email_subject",
		"email_title",
		"email_intro",
		"email_footer",
		"days_expired",
		"days_expiring_today",
		"days_one_remaining",
		"days_remaining",
	}

	// Construct path to translation file
	translationFile := filepath.Join("i18n", lang+".json")

	// Check if file exists
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("unsupported language '%s': translation file not found at %s", lang, translationFile)
	}

	// Read translation file
	data, err := os.ReadFile(translationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read translation file %s: %w", translationFile, err)
	}

	// Parse JSON
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return nil, fmt.Errorf("failed to parse translation file %s: %w", translationFile, err)
	}

	// Validate all required keys are present
	var missingKeys []string
	for _, key := range requiredKeys {
		if _, ok := translations[key]; !ok {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("translation file %s is missing required keys: %s", translationFile, strings.Join(missingKeys, ", "))
	}

	logger.Info("Loaded translations", zap.String("language", lang), zap.Int("keys", len(translations)))
	return translations, nil
}

func sendEmail(cfg Config, products []ExpiringProduct) error {
	var body bytes.Buffer

	tplProducts := make([]templates.ExpiringProduct, len(products))
	for i, p := range products {
		tplProducts[i] = templates.ExpiringProduct{
			Name:      p.Name,
			Packaging: p.Packaging,
			Quantity:  p.Quantity,
			DaysUntil: p.DaysUntil,
			Threshold: p.Threshold,
		}
	}

	err := templates.ExpirationEmail(tplProducts, templates.Translations(cfg.Translations)).Render(context.Background(), &body)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subjectText := fmt.Sprintf(cfg.Translations["email_subject"], len(products))
	subject := fmt.Sprintf("=?UTF-8?B?%s?=",
		base64.StdEncoding.EncodeToString([]byte(subjectText)))

	msg := buildMIMEMessage(cfg.SMTP.From, cfg.RecipientEmails, subject, body.String(), cfg.SMTP.Headers)

	auth := smtp.PlainAuth("", cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Host)
	addr := fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)

	return smtp.SendMail(addr, auth, cfg.SMTP.From, cfg.RecipientEmails, msg)
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

func parseProducts(rows [][]any) []Product {
	var products []Product
	for i, row := range rows {
		if i == 0 || len(row) < 4 {
			continue
		}
		qty, _ := strconv.Atoi(fmt.Sprint(row[2]))
		exp, err := time.Parse("2006-01-02", fmt.Sprint(row[3]))
		if err != nil {
			continue
		}
		var sentDays []int
		if len(row) >= 5 && row[4] != nil {
			sentDays = parseSentDays(fmt.Sprint(row[4]))
		}
		products = append(products, Product{
			Row:        i + 1,
			Name:       fmt.Sprint(row[0]),
			Packaging:  fmt.Sprint(row[1]),
			Quantity:   qty,
			Expiration: exp,
			SentDays:   sentDays,
		})
	}
	return products
}

func parseSentDays(s string) []int {
	if s == "" {
		return nil
	}
	var days []int
	for part := range strings.SplitSeq(s, ",") {
		if d, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			days = append(days, d)
		}
	}
	return days
}

func formatSentDays(days []int) string {
	var parts []string
	for _, d := range days {
		parts = append(parts, strconv.Itoa(d))
	}
	return strings.Join(parts, ",")
}
